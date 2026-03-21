package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sort"
	"strings"

	"github.com/badskater/distributed-encoder/internal/controller/engine"
	"github.com/badskater/distributed-encoder/internal/controller/webhooks"
	"github.com/badskater/distributed-encoder/internal/db"
	pb "github.com/badskater/distributed-encoder/internal/proto/encoderv1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ReportResult is called by an agent when a task finishes (success or failure).
// It updates the task and job state in the database accordingly.
func (s *Server) ReportResult(ctx context.Context, req *pb.TaskResult) (*pb.Ack, error) {
	if req.GetTaskId() == "" || req.GetJobId() == "" {
		return nil, status.Error(codes.InvalidArgument, "task_id and job_id are required")
	}

	s.logger.LogAttrs(ctx, slog.LevelInfo, "task result received",
		slog.String("task_id", req.GetTaskId()),
		slog.String("job_id", req.GetJobId()),
		slog.Bool("success", req.GetSuccess()),
		slog.Int64("exit_code", int64(req.GetExitCode())),
	)

	if err := s.processResult(ctx, req); err != nil {
		return nil, err
	}

	// Recalculate denormalised job counters.
	if err := s.store.UpdateJobTaskCounts(ctx, req.GetJobId()); err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "failed to update job task counts",
			slog.String("job_id", req.GetJobId()),
			slog.String("error", err.Error()),
		)
		return nil, status.Errorf(codes.Internal, "grpc reportresult: update job counts: %v", err)
	}

	// Check whether the job is now terminal.
	if err := s.checkJobCompletion(ctx, req.GetJobId()); err != nil {
		return nil, err
	}

	return &pb.Ack{Ok: true}, nil
}

// processResult marks a single task as completed or failed.
func (s *Server) processResult(ctx context.Context, req *pb.TaskResult) error {
	if req.GetSuccess() {
		p := db.CompleteTaskParams{
			ID:       req.GetTaskId(),
			ExitCode: int(req.GetExitCode()),
		}
		if m := req.GetMetrics(); m != nil {
			p.FramesEncoded = m.GetFramesEncoded()
			p.AvgFPS = float64(m.GetAvgFps())
			p.OutputSize = m.GetOutputSize()
			p.DurationSec = m.GetDurationSec()
			if v := float64(m.GetVmafScore()); v != 0 {
				p.VMafScore = &v
			}
			if v := float64(m.GetPsnr()); v != 0 {
				p.PSNR = &v
			}
			if v := float64(m.GetSsim()); v != 0 {
				p.SSIM = &v
			}
		}
		if err := s.store.CompleteTask(ctx, p); err != nil {
			return status.Errorf(codes.Internal, "grpc reportresult: complete task: %v", err)
		}
		return nil
	}

	// Failure path.
	if err := s.store.FailTask(ctx, req.GetTaskId(), int(req.GetExitCode()), req.GetErrorMsg()); err != nil {
		return status.Errorf(codes.Internal, "grpc reportresult: fail task: %v", err)
	}
	return nil
}

// checkJobCompletion inspects the refreshed job counters and transitions the
// job to "completed" or "failed" when no tasks remain pending or running.
// When a controller-side ConcatRunner is configured and all encode tasks have
// finished, it claims the pending concat task and runs it in a goroutine.
func (s *Server) checkJobCompletion(ctx context.Context, jobID string) error {
	job, err := s.store.GetJobByID(ctx, jobID)
	if err != nil {
		return status.Errorf(codes.Internal, "grpc reportresult: get job: %v", err)
	}

	// Controller-side concat: exactly one pending task (the concat) remains and
	// no tasks are currently running.
	if s.concatRunner != nil && job.TasksPending == 1 && job.TasksRunning == 0 {
		tasks, err := s.store.ListTasksByJob(ctx, jobID)
		if err != nil {
			return status.Errorf(codes.Internal, "grpc reportresult: list tasks for concat: %v", err)
		}

		var concatTask *db.Task
		for _, t := range tasks {
			if t.TaskType == db.TaskTypeConcat && t.Status == "pending" {
				concatTask = t
				break
			}
		}

		if concatTask != nil {
			if job.TasksFailed > 0 {
				// Encode chunks failed — skip concat and mark it failed.
				_ = s.store.FailTask(ctx, concatTask.ID, -1, "skipped: chunk tasks failed")
				_ = s.store.UpdateJobTaskCounts(ctx, jobID)
				// Fall through to normal terminal logic below.
			} else {
				// CAS: transition concat task pending→running; only one goroutine wins.
				if claimErr := s.store.ClaimConcatTask(ctx, concatTask.ID); claimErr != nil {
					if errors.Is(claimErr, db.ErrNotFound) {
						return nil // another goroutine already claimed it
					}
					return status.Errorf(codes.Internal, "grpc reportresult: claim concat task: %v", claimErr)
				}

				// Collect completed chunk paths sorted by chunk index.
				sort.Slice(tasks, func(i, j int) bool {
					return tasks[i].ChunkIndex < tasks[j].ChunkIndex
				})
				var chunkPaths []string
				for _, t := range tasks {
					if t.TaskType != db.TaskTypeConcat && t.Status == "completed" {
						chunkPaths = append(chunkPaths, t.OutputPath)
					}
				}

				go func() {
					runErr := s.concatRunner.RunConcat(context.Background(), job, chunkPaths, concatTask.OutputPath)
					if runErr != nil {
						s.logger.Error("controller concat failed",
							slog.String("job_id", jobID),
							slog.String("error", runErr.Error()),
						)
						_ = s.store.FailTask(context.Background(), concatTask.ID, 1, runErr.Error())
					} else {
						_ = s.store.CompleteTask(context.Background(), db.CompleteTaskParams{
							ID:       concatTask.ID,
							ExitCode: 0,
						})
					}
					_ = s.store.UpdateJobTaskCounts(context.Background(), jobID)
					_ = s.checkJobCompletion(context.Background(), jobID)
				}()
				return nil // don't fall through to job completion yet
			}
		}
	}

	if job.TasksPending > 0 || job.TasksRunning > 0 {
		return nil
	}

	var newStatus string
	if job.TasksFailed > 0 {
		newStatus = "failed"
	} else {
		newStatus = "completed"
	}

	if err := s.store.UpdateJobStatus(ctx, jobID, newStatus); err != nil {
		return status.Errorf(codes.Internal, "grpc reportresult: update job status: %v", err)
	}

	s.logger.LogAttrs(ctx, slog.LevelInfo, "job status updated",
		slog.String("job_id", jobID),
		slog.String("status", newStatus),
	)

	// For completed hdr_detect jobs, scan task stdout for the sentinel line
	// and write the result back to the source.
	if job.JobType == "hdr_detect" && newStatus == "completed" {
		if err := s.extractHDRResult(ctx, job); err != nil {
			s.logger.LogAttrs(ctx, slog.LevelWarn, "hdr_detect result extraction failed",
				slog.String("job_id", jobID),
				slog.String("error", err.Error()),
			)
		}
	}

	eventType := "job." + newStatus
	s.webhooks.Emit(ctx, webhooks.Event{
		Type: eventType,
		Payload: map[string]any{
			"job_id":          jobID,
			"tasks_completed": job.TasksCompleted,
			"tasks_failed":    job.TasksFailed,
		},
	})

	return nil
}

// extractHDRResult scans the task stdout logs of a completed hdr_detect job
// for the DE_HDR_RESULT sentinel line, parses it, and updates the source.
func (s *Server) extractHDRResult(ctx context.Context, job *db.Job) error {
	tasks, err := s.store.ListTasksByJob(ctx, job.ID)
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		return nil
	}
	task := tasks[0]

	// Fetch stdout logs in pages until the sentinel is found or logs are exhausted.
	const pageSize = 500
	var cursor int64
	for {
		logs, err := s.store.ListTaskLogs(ctx, db.ListTaskLogsParams{
			TaskID:   task.ID,
			Stream:   "stdout",
			Cursor:   cursor,
			PageSize: pageSize,
		})
		if err != nil {
			return err
		}

		for _, l := range logs {
			if !strings.HasPrefix(l.Message, engine.HDRResultSentinel) {
				continue
			}
			raw := strings.TrimPrefix(l.Message, engine.HDRResultSentinel)
			var result struct {
				HDRType   string `json:"hdr_type"`
				DVProfile int    `json:"dv_profile"`
			}
			if err := json.Unmarshal([]byte(raw), &result); err != nil {
				s.logger.Warn("hdr_detect: failed to parse sentinel JSON",
					"job_id", job.ID,
					"raw", raw,
					"error", err,
				)
				return nil
			}
			if err := s.store.UpdateSourceHDR(ctx, db.UpdateSourceHDRParams{
				ID:        job.SourceID,
				HDRType:   result.HDRType,
				DVProfile: result.DVProfile,
			}); err != nil {
				return err
			}
			s.logger.LogAttrs(ctx, slog.LevelInfo, "source HDR metadata updated",
				slog.String("source_id", job.SourceID),
				slog.String("hdr_type", result.HDRType),
				slog.Int("dv_profile", result.DVProfile),
			)
			return nil
		}

		if len(logs) < pageSize {
			break
		}
		cursor = logs[len(logs)-1].ID
	}

	s.logger.LogAttrs(ctx, slog.LevelWarn, "hdr_detect: sentinel line not found in stdout logs",
		slog.String("job_id", job.ID),
	)
	return nil
}

package grpc

import (
	"context"
	"log/slog"

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
func (s *Server) checkJobCompletion(ctx context.Context, jobID string) error {
	job, err := s.store.GetJobByID(ctx, jobID)
	if err != nil {
		return status.Errorf(codes.Internal, "grpc reportresult: get job: %v", err)
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
	return nil
}

package grpc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	pb "github.com/badskater/distributed-encoder/internal/proto/encoderv1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PollTask handles the task polling RPC. An agent calls this to request its
// next task assignment.
func (s *Server) PollTask(ctx context.Context, req *pb.PollTaskReq) (*pb.TaskAssignment, error) {
	if req.GetAgentId() == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}

	task, err := s.store.ClaimNextTask(ctx, req.GetAgentId(), req.GetTags())
	if err != nil {
		return nil, fmt.Errorf("grpc poll task: %w", err)
	}

	if task == nil {
		return &pb.TaskAssignment{HasTask: false}, nil
	}

	// Read script files from ScriptDir.
	scripts := make(map[string]string)
	if task.ScriptDir != "" {
		scripts = readScriptDir(task.ScriptDir, s)
	}

	// Update job task counts.
	if err := s.store.UpdateJobTaskCounts(ctx, task.JobID); err != nil {
		s.logger.Warn("grpc poll task: failed to update job task counts",
			"job_id", task.JobID,
			"error", err,
		)
	}

	// Transition job status to "running" if it is still "assigned".
	job, err := s.store.GetJobByID(ctx, task.JobID)
	if err != nil {
		s.logger.Warn("grpc poll task: failed to get job",
			"job_id", task.JobID,
			"error", err,
		)
	} else if job.Status == "assigned" {
		if err := s.store.UpdateJobStatus(ctx, job.ID, "running"); err != nil {
			s.logger.Warn("grpc poll task: failed to update job status",
				"job_id", job.ID,
				"error", err,
			)
		}
	}

	// Determine job priority.
	var priority int32
	if job != nil {
		priority = int32(job.Priority)
	}

	return &pb.TaskAssignment{
		HasTask:    true,
		TaskId:     task.ID,
		JobId:      task.JobID,
		ChunkIndex: int32(task.ChunkIndex),
		Scripts:    scripts,
		Variables:  task.Variables,
		SourcePath: task.SourcePath,
		OutputPath: task.OutputPath,
		TimeoutSec: 3600,
		Priority:   priority,
	}, nil
}

// readScriptDir reads all files in a directory and returns them as a map of
// filename to content. Errors are logged as warnings and an empty map is
// returned on failure.
func readScriptDir(dir string, s *Server) map[string]string {
	scripts := make(map[string]string)

	entries, err := os.ReadDir(dir)
	if err != nil {
		s.logger.Warn("grpc poll task: failed to read script dir",
			"dir", dir,
			"error", err,
		)
		return scripts
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			s.logger.Warn("grpc poll task: failed to read script file",
				"path", path,
				"error", err,
			)
			continue
		}
		scripts[entry.Name()] = string(data)
	}

	return scripts
}

package grpc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/badskater/encodeswarmr/internal/controller/config"
	"github.com/badskater/encodeswarmr/internal/controller/webhooks"
	"github.com/badskater/encodeswarmr/internal/db"
	"github.com/badskater/encodeswarmr/internal/db/teststore"
	pb "github.com/badskater/encodeswarmr/internal/proto/encoderv1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ---------------------------------------------------------------------------
// Stub
// ---------------------------------------------------------------------------

type pollStub struct {
	teststore.Stub
	// ClaimNextTask control
	task        *db.Task
	claimErr    error
	claimedID   string
	claimedTags []string
	// UpdateJobTaskCounts control
	updateCountsErr error
	updatedCountsJobID string
	// GetJobByID control
	job    *db.Job
	jobErr error
	// UpdateJobStatus control
	updatedJobStatus string
}

func (s *pollStub) ClaimNextTask(_ context.Context, agentID string, tags []string) (*db.Task, error) {
	s.claimedID = agentID
	s.claimedTags = tags
	return s.task, s.claimErr
}

func (s *pollStub) UpdateJobTaskCounts(_ context.Context, jobID string) error {
	s.updatedCountsJobID = jobID
	return s.updateCountsErr
}

func (s *pollStub) GetJobByID(_ context.Context, _ string) (*db.Job, error) {
	return s.job, s.jobErr
}

func (s *pollStub) UpdateJobStatus(_ context.Context, _ string, newStatus string) error {
	s.updatedJobStatus = newStatus
	return nil
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newPollServer(stub *pollStub) *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wh := webhooks.New(stub, webhooks.Config{}, logger)
	return &Server{
		store:    stub,
		cfg:      &config.GRPCConfig{},
		agentCfg: &config.AgentConfig{TaskTimeoutSec: 7200},
		logger:   logger,
		webhooks: wh,
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestPollTask_MissingAgentID(t *testing.T) {
	srv := newPollServer(&pollStub{})
	_, err := srv.PollTask(context.Background(), &pb.PollTaskReq{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestPollTask_NoTaskAvailable(t *testing.T) {
	stub := &pollStub{task: nil}
	srv := newPollServer(stub)

	resp, err := srv.PollTask(context.Background(), &pb.PollTaskReq{AgentId: "agent-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.HasTask {
		t.Error("expected HasTask=false when no task available")
	}
	if stub.claimedID != "agent-1" {
		t.Errorf("ClaimNextTask called with agentID=%q, want %q", stub.claimedID, "agent-1")
	}
}

func TestPollTask_ClaimError(t *testing.T) {
	stub := &pollStub{claimErr: errors.New("db error")}
	srv := newPollServer(stub)

	_, err := srv.PollTask(context.Background(), &pb.PollTaskReq{AgentId: "agent-1"})
	if err == nil {
		t.Fatal("expected error when ClaimNextTask fails")
	}
}

func TestPollTask_TaskAssigned_NoScriptDir(t *testing.T) {
	stub := &pollStub{
		task: &db.Task{
			ID:         "task-1",
			JobID:      "job-1",
			ChunkIndex: 2,
			ScriptDir:  "", // empty — no files to read
			SourcePath: "\\\\nas\\source.mkv",
			OutputPath: "\\\\nas\\out\\chunk2.mkv",
		},
		job: &db.Job{ID: "job-1", Status: "assigned", Priority: 5},
	}
	srv := newPollServer(stub)

	resp, err := srv.PollTask(context.Background(), &pb.PollTaskReq{AgentId: "agent-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.HasTask {
		t.Error("expected HasTask=true")
	}
	if resp.TaskId != "task-1" {
		t.Errorf("TaskId = %q, want %q", resp.TaskId, "task-1")
	}
	if resp.JobId != "job-1" {
		t.Errorf("JobId = %q, want %q", resp.JobId, "job-1")
	}
	if resp.ChunkIndex != 2 {
		t.Errorf("ChunkIndex = %d, want 2", resp.ChunkIndex)
	}
	if resp.TimeoutSec != 7200 {
		t.Errorf("TimeoutSec = %d, want 7200", resp.TimeoutSec)
	}
	if resp.Priority != 5 {
		t.Errorf("Priority = %d, want 5", resp.Priority)
	}
	// Job should transition from "assigned" to "running".
	if stub.updatedJobStatus != "running" {
		t.Errorf("job status = %q, want %q", stub.updatedJobStatus, "running")
	}
}

func TestPollTask_JobAlreadyRunning_NoStatusUpdate(t *testing.T) {
	stub := &pollStub{
		task: &db.Task{ID: "task-2", JobID: "job-2"},
		job:  &db.Job{ID: "job-2", Status: "running"}, // already running
	}
	srv := newPollServer(stub)

	_, err := srv.PollTask(context.Background(), &pb.PollTaskReq{AgentId: "agent-3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.updatedJobStatus != "" {
		t.Errorf("job status should not change for running job, got %q", stub.updatedJobStatus)
	}
}

func TestPollTask_GetJobError_Continues(t *testing.T) {
	// GetJobByID failing should only log a warning, not return an error.
	stub := &pollStub{
		task:   &db.Task{ID: "task-3", JobID: "job-3"},
		jobErr: errors.New("db unavailable"),
	}
	srv := newPollServer(stub)

	resp, err := srv.PollTask(context.Background(), &pb.PollTaskReq{AgentId: "agent-4"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.HasTask {
		t.Error("expected HasTask=true even when GetJobByID fails")
	}
	// Priority should default to 0 when job is nil.
	if resp.Priority != 0 {
		t.Errorf("Priority = %d, want 0 when job lookup fails", resp.Priority)
	}
}

func TestPollTask_WithScriptDir(t *testing.T) {
	// Create a temp dir with some script files.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "encode.bat"), []byte("@echo off\nffmpeg ..."), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "source.avs"), []byte("FFVideoSource(...)"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Create a subdirectory that should be skipped.
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o700); err != nil {
		t.Fatal(err)
	}

	stub := &pollStub{
		task: &db.Task{
			ID:        "task-scripts",
			JobID:     "job-scripts",
			ScriptDir: dir,
		},
		job: &db.Job{ID: "job-scripts", Status: "assigned"},
	}
	srv := newPollServer(stub)

	resp, err := srv.PollTask(context.Background(), &pb.PollTaskReq{AgentId: "agent-scripts"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Scripts) != 2 {
		t.Errorf("expected 2 scripts, got %d: %v", len(resp.Scripts), resp.Scripts)
	}
	if resp.Scripts["encode.bat"] == "" {
		t.Error("expected encode.bat in scripts")
	}
}

func TestPollTask_ScriptDirNotExist(t *testing.T) {
	// A non-existent script dir should log a warning and return an empty map, not fail.
	stub := &pollStub{
		task: &db.Task{
			ID:        "task-nodir",
			JobID:     "job-nodir",
			ScriptDir: "/does/not/exist/at/all",
		},
		job: &db.Job{ID: "job-nodir", Status: "assigned"},
	}
	srv := newPollServer(stub)

	resp, err := srv.PollTask(context.Background(), &pb.PollTaskReq{AgentId: "agent-nodir"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Scripts) != 0 {
		t.Errorf("expected empty scripts map for missing dir, got %d entries", len(resp.Scripts))
	}
}

func TestPollTask_TagsPassedThrough(t *testing.T) {
	stub := &pollStub{task: nil} // no task available
	srv := newPollServer(stub)

	_, err := srv.PollTask(context.Background(), &pb.PollTaskReq{
		AgentId: "agent-tags",
		Tags:    []string{"gpu", "4k"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stub.claimedTags) != 2 || stub.claimedTags[0] != "gpu" {
		t.Errorf("tags not forwarded to ClaimNextTask: %v", stub.claimedTags)
	}
}

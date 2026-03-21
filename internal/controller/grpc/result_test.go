package grpc

import (
	"context"
	"errors"
	"io"
	"log/slog"
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
// Stub store
// ---------------------------------------------------------------------------

// resultStub is a minimal db.Store that records which task operations were
// called and controls the job returned by GetJobByID.
type resultStub struct {
	teststore.Stub
	// Injected responses
	job    *db.Job
	jobErr error
	tasks  []*db.Task
	logs   []*db.TaskLog
	// Recorded calls
	completedID      string
	failedID         string
	failCode         int
	failMsg          string
	updatedJobStatus string
	updatedHDRSourceID string
	updatedHDRType     string
	updatedHDRProfile  int
}

func (s *resultStub) CompleteTask(_ context.Context, p db.CompleteTaskParams) error {
	s.completedID = p.ID
	return nil
}
func (s *resultStub) FailTask(_ context.Context, id string, code int, msg string) error {
	s.failedID = id
	s.failCode = code
	s.failMsg = msg
	return nil
}
func (s *resultStub) GetJobByID(_ context.Context, _ string) (*db.Job, error) {
	return s.job, s.jobErr
}
func (s *resultStub) UpdateJobStatus(_ context.Context, _ string, newStatus string) error {
	s.updatedJobStatus = newStatus
	return nil
}
func (s *resultStub) UpdateSourceHDR(_ context.Context, p db.UpdateSourceHDRParams) error {
	s.updatedHDRSourceID = p.ID
	s.updatedHDRType = p.HDRType
	s.updatedHDRProfile = p.DVProfile
	return nil
}
func (s *resultStub) ListTasksByJob(_ context.Context, _ string) ([]*db.Task, error) {
	return s.tasks, nil
}
func (s *resultStub) ListTaskLogs(_ context.Context, _ db.ListTaskLogsParams) ([]*db.TaskLog, error) {
	return s.logs, nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newResultServer(store *resultStub) *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wh := webhooks.New(store, webhooks.Config{}, logger)
	return &Server{
		store:    store,
		cfg:      &config.GRPCConfig{},
		agentCfg: &config.AgentConfig{TaskTimeoutSec: 3600},
		logger:   logger,
		webhooks: wh,
	}
}

// allDoneJob returns a job where all tasks are complete (no pending/running).
func allDoneJob(id string, failed int) *db.Job {
	return &db.Job{
		ID:             id,
		Status:         "running",
		TasksTotal:     3,
		TasksCompleted: 3 - failed,
		TasksFailed:    failed,
		TasksPending:   0,
		TasksRunning:   0,
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestReportResult_MissingFields(t *testing.T) {
	srv := newResultServer(&resultStub{})
	ctx := context.Background()

	cases := []struct {
		name string
		req  *pb.TaskResult
	}{
		{"empty", &pb.TaskResult{}},
		{"missing job_id", &pb.TaskResult{TaskId: "t1"}},
		{"missing task_id", &pb.TaskResult{JobId: "j1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := srv.ReportResult(ctx, tc.req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if st, ok := status.FromError(err); !ok || st.Code() != codes.InvalidArgument {
				t.Errorf("expected InvalidArgument, got %v", err)
			}
		})
	}
}

func TestReportResult_SuccessPath(t *testing.T) {
	stub := &resultStub{
		job: allDoneJob("job-1", 0), // all completed, no failures
	}
	srv := newResultServer(stub)
	ctx := context.Background()

	req := &pb.TaskResult{
		TaskId:  "task-1",
		JobId:   "job-1",
		Success: true,
		Metrics: &pb.EncodeMetrics{
			FramesEncoded: 1000,
			AvgFps:        24.5,
			OutputSize:    1024 * 1024,
		},
	}

	ack, err := srv.ReportResult(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ack.Ok {
		t.Error("expected ack.ok = true")
	}
	if stub.completedID != "task-1" {
		t.Errorf("CompleteTask not called with task-1, got %q", stub.completedID)
	}
	if stub.updatedJobStatus != "completed" {
		t.Errorf("expected job status 'completed', got %q", stub.updatedJobStatus)
	}
}

func TestReportResult_FailurePath(t *testing.T) {
	stub := &resultStub{
		job: allDoneJob("job-2", 1), // one failure
	}
	srv := newResultServer(stub)
	ctx := context.Background()

	req := &pb.TaskResult{
		TaskId:   "task-2",
		JobId:    "job-2",
		Success:  false,
		ExitCode: 1,
		ErrorMsg: "encoder crashed",
	}

	ack, err := srv.ReportResult(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ack.Ok {
		t.Error("expected ack.ok = true")
	}
	if stub.failedID != "task-2" {
		t.Errorf("FailTask not called with task-2, got %q", stub.failedID)
	}
	if stub.failCode != 1 {
		t.Errorf("expected exit code 1, got %d", stub.failCode)
	}
	if stub.failMsg != "encoder crashed" {
		t.Errorf("expected error msg 'encoder crashed', got %q", stub.failMsg)
	}
	if stub.updatedJobStatus != "failed" {
		t.Errorf("expected job status 'failed', got %q", stub.updatedJobStatus)
	}
}

func TestReportResult_PendingTasksRemain(t *testing.T) {
	// Job still has pending tasks — should NOT transition to terminal state.
	stub := &resultStub{
		job: &db.Job{
			ID:           "job-3",
			Status:       "running",
			TasksTotal:   5,
			TasksPending: 2, // more work to do
			TasksRunning: 0,
		},
	}
	srv := newResultServer(stub)
	ctx := context.Background()

	req := &pb.TaskResult{
		TaskId:  "task-3",
		JobId:   "job-3",
		Success: true,
	}

	if _, err := srv.ReportResult(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.updatedJobStatus != "" {
		t.Errorf("job status should not change while tasks remain, got %q", stub.updatedJobStatus)
	}
}

func TestReportResult_GetJobError(t *testing.T) {
	stub := &resultStub{
		jobErr: errors.New("db unavailable"),
	}
	srv := newResultServer(stub)
	ctx := context.Background()

	req := &pb.TaskResult{TaskId: "t", JobId: "j", Success: true}

	_, err := srv.ReportResult(ctx, req)
	if err == nil {
		t.Fatal("expected error when GetJobByID fails")
	}
	if st, ok := status.FromError(err); !ok || st.Code() != codes.Internal {
		t.Errorf("expected Internal error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// extractHDRResult via checkJobCompletion (triggered by hdr_detect job
// completing successfully inside ReportResult)
// ---------------------------------------------------------------------------

func TestReportResult_HDRDetect_SentinelParsed(t *testing.T) {
	// When an hdr_detect job completes, the controller should find the
	// DE_HDR_RESULT sentinel in task stdout and call UpdateSourceHDR.
	stub := &resultStub{
		job: &db.Job{
			ID:             "job-hdr",
			SourceID:       "src-hdr",
			JobType:        "hdr_detect",
			Status:         "running",
			TasksTotal:     1,
			TasksCompleted: 1,
			TasksFailed:    0,
			TasksPending:   0,
			TasksRunning:   0,
		},
		tasks: []*db.Task{{ID: "task-hdr", JobID: "job-hdr"}},
		logs: []*db.TaskLog{
			{ID: 1, TaskID: "task-hdr", Stream: "stdout",
				Message: `DE_HDR_RESULT={"hdr_type":"dolby_vision","dv_profile":8}`},
		},
	}
	srv := newResultServer(stub)
	ctx := context.Background()

	req := &pb.TaskResult{
		TaskId:  "task-hdr",
		JobId:   "job-hdr",
		Success: true,
	}

	ack, err := srv.ReportResult(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ack.Ok {
		t.Error("expected ack.ok = true")
	}
	if stub.updatedHDRSourceID != "src-hdr" {
		t.Errorf("UpdateSourceHDR source_id = %q, want %q", stub.updatedHDRSourceID, "src-hdr")
	}
	if stub.updatedHDRType != "dolby_vision" {
		t.Errorf("UpdateSourceHDR hdr_type = %q, want %q", stub.updatedHDRType, "dolby_vision")
	}
	if stub.updatedHDRProfile != 8 {
		t.Errorf("UpdateSourceHDR dv_profile = %d, want 8", stub.updatedHDRProfile)
	}
	if stub.updatedJobStatus != "completed" {
		t.Errorf("job status = %q, want %q", stub.updatedJobStatus, "completed")
	}
}

func TestReportResult_HDRDetect_NoSentinel(t *testing.T) {
	// When no sentinel line is found, UpdateSourceHDR must NOT be called but
	// the job should still transition to "completed".
	stub := &resultStub{
		job: &db.Job{
			ID:             "job-hdr-nosent",
			SourceID:       "src-nosent",
			JobType:        "hdr_detect",
			Status:         "running",
			TasksTotal:     1,
			TasksCompleted: 1,
			TasksFailed:    0,
			TasksPending:   0,
			TasksRunning:   0,
		},
		tasks: []*db.Task{{ID: "task-nosent", JobID: "job-hdr-nosent"}},
		logs: []*db.TaskLog{
			{ID: 1, TaskID: "task-nosent", Stream: "stdout", Message: "some random stdout line"},
		},
	}
	srv := newResultServer(stub)

	req := &pb.TaskResult{TaskId: "task-nosent", JobId: "job-hdr-nosent", Success: true}

	if _, err := srv.ReportResult(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.updatedHDRSourceID != "" {
		t.Errorf("UpdateSourceHDR should not be called, but got source_id = %q", stub.updatedHDRSourceID)
	}
	if stub.updatedJobStatus != "completed" {
		t.Errorf("job status = %q, want %q", stub.updatedJobStatus, "completed")
	}
}

func TestReportResult_HDRDetect_NonHDRJob_NoExtraction(t *testing.T) {
	// A regular encode job completing should never trigger HDR extraction.
	stub := &resultStub{
		job: allDoneJob("job-enc", 0),
	}
	stub.job.JobType = "encode"
	srv := newResultServer(stub)

	req := &pb.TaskResult{TaskId: "task-enc", JobId: "job-enc", Success: true}

	if _, err := srv.ReportResult(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.updatedHDRSourceID != "" {
		t.Errorf("UpdateSourceHDR should not be called for encode jobs")
	}
}

func TestReportResult_HDRDetect_HDR10Plus(t *testing.T) {
	// Verify HDR10+ sentinel value is parsed correctly.
	stub := &resultStub{
		job: &db.Job{
			ID: "job-hdr10p", SourceID: "src-hdr10p", JobType: "hdr_detect",
			Status: "running", TasksTotal: 1, TasksCompleted: 1,
		},
		tasks: []*db.Task{{ID: "task-hdr10p", JobID: "job-hdr10p"}},
		logs: []*db.TaskLog{
			{ID: 1, TaskID: "task-hdr10p", Stream: "stdout",
				Message: `DE_HDR_RESULT={"hdr_type":"hdr10+","dv_profile":0}`},
		},
	}
	srv := newResultServer(stub)

	if _, err := srv.ReportResult(context.Background(), &pb.TaskResult{
		TaskId: "task-hdr10p", JobId: "job-hdr10p", Success: true,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.updatedHDRType != "hdr10+" {
		t.Errorf("hdr_type = %q, want %q", stub.updatedHDRType, "hdr10+")
	}
	if stub.updatedHDRProfile != 0 {
		t.Errorf("dv_profile = %d, want 0", stub.updatedHDRProfile)
	}
}

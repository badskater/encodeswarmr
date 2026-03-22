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
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ---------------------------------------------------------------------------
// Fake client-streaming server for SyncOfflineResults
// ---------------------------------------------------------------------------

type fakeSyncStream struct {
	grpc.ServerStream
	results []*pb.TaskResult
	recvIdx int
	recvErr error
	closed  *pb.SyncResponse
	ctx     context.Context
}

func newFakeSyncStream(results []*pb.TaskResult) *fakeSyncStream {
	return &fakeSyncStream{results: results, ctx: context.Background()}
}

func (s *fakeSyncStream) Context() context.Context { return s.ctx }
func (s *fakeSyncStream) SetHeader(_ metadata.MD) error  { return nil }
func (s *fakeSyncStream) SendHeader(_ metadata.MD) error { return nil }
func (s *fakeSyncStream) SetTrailer(_ metadata.MD)       {}

func (s *fakeSyncStream) Recv() (*pb.TaskResult, error) {
	if s.recvErr != nil {
		return nil, s.recvErr
	}
	if s.recvIdx >= len(s.results) {
		return nil, io.EOF
	}
	r := s.results[s.recvIdx]
	s.recvIdx++
	return r, nil
}

func (s *fakeSyncStream) SendAndClose(resp *pb.SyncResponse) error {
	s.closed = resp
	return nil
}

// ---------------------------------------------------------------------------
// Stub
// ---------------------------------------------------------------------------

// syncStub covers all store methods used by sync.go, result.go helpers.
type syncStub struct {
	teststore.Stub
	// GetTaskByID
	task    *db.Task
	taskErr error
	// CompleteTask / FailTask
	completedID string
	failedID    string
	// UpdateJobTaskCounts
	updateCountsErr    error
	updateCountsJobID  string
	// GetJobByID
	job    *db.Job
	jobErr error
	// UpdateJobStatus
	updatedJobStatus string
	// ListTasksByJob / ListTaskLogs (for HDR extraction — not triggered in sync tests)
	logs []*db.TaskLog
}

func (s *syncStub) GetTaskByID(_ context.Context, _ string) (*db.Task, error) {
	return s.task, s.taskErr
}
func (s *syncStub) CompleteTask(_ context.Context, p db.CompleteTaskParams) error {
	s.completedID = p.ID
	return nil
}
func (s *syncStub) FailTask(_ context.Context, id string, _ int, _ string) error {
	s.failedID = id
	return nil
}
func (s *syncStub) UpdateJobTaskCounts(_ context.Context, jobID string) error {
	s.updateCountsJobID = jobID
	return s.updateCountsErr
}
func (s *syncStub) GetJobByID(_ context.Context, _ string) (*db.Job, error) {
	return s.job, s.jobErr
}
func (s *syncStub) UpdateJobStatus(_ context.Context, _ string, newStatus string) error {
	s.updatedJobStatus = newStatus
	return nil
}
func (s *syncStub) ListTasksByJob(_ context.Context, _ string) ([]*db.Task, error) {
	return nil, nil
}
func (s *syncStub) ListTaskLogs(_ context.Context, _ db.ListTaskLogsParams) ([]*db.TaskLog, error) {
	return s.logs, nil
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newSyncServer(stub *syncStub) *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wh := webhooks.New(stub, webhooks.Config{}, logger)
	return &Server{
		store:    stub,
		cfg:      &config.GRPCConfig{},
		agentCfg: &config.AgentConfig{TaskTimeoutSec: 3600},
		logger:   logger,
		webhooks: wh,
	}
}

// allDoneSyncJob builds a minimal job where no tasks remain pending/running.
func allDoneSyncJob(id string) *db.Job {
	return &db.Job{
		ID:             id,
		Status:         "running",
		JobType:        "encode",
		TasksTotal:     1,
		TasksCompleted: 1,
		TasksFailed:    0,
		TasksPending:   0,
		TasksRunning:   0,
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestSyncOfflineResults_EmptyStream(t *testing.T) {
	srv := newSyncServer(&syncStub{})

	stream := newFakeSyncStream(nil)
	err := srv.SyncOfflineResults(stream)
	if err != nil {
		t.Fatalf("unexpected error on empty stream: %v", err)
	}
	if stream.closed == nil {
		t.Fatal("expected SendAndClose to be called")
	}
	if stream.closed.Accepted != 0 {
		t.Errorf("expected 0 accepted, got %d", stream.closed.Accepted)
	}
	if len(stream.closed.RejectedTaskIds) != 0 {
		t.Errorf("expected 0 rejected, got %v", stream.closed.RejectedTaskIds)
	}
}

func TestSyncOfflineResults_AcceptSuccess(t *testing.T) {
	stub := &syncStub{
		task: &db.Task{ID: "task-sync-1", Status: "running"},
		job:  allDoneSyncJob("job-sync-1"),
	}
	srv := newSyncServer(stub)

	stream := newFakeSyncStream([]*pb.TaskResult{
		{
			TaskId:        "task-sync-1",
			JobId:         "job-sync-1",
			Success:       true,
			OfflineResult: true,
		},
	})

	if err := srv.SyncOfflineResults(stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stream.closed.Accepted != 1 {
		t.Errorf("expected 1 accepted, got %d", stream.closed.Accepted)
	}
	if stub.completedID != "task-sync-1" {
		t.Errorf("CompleteTask not called with task-sync-1, got %q", stub.completedID)
	}
}

func TestSyncOfflineResults_RejectAlreadyFinished(t *testing.T) {
	// A task that is already "completed" and the result is NOT marked offline
	// should be rejected.
	stub := &syncStub{
		task: &db.Task{ID: "task-done", Status: "completed"},
		job:  allDoneSyncJob("job-done"),
	}
	srv := newSyncServer(stub)

	stream := newFakeSyncStream([]*pb.TaskResult{
		{
			TaskId:        "task-done",
			JobId:         "job-done",
			Success:       true,
			OfflineResult: false, // not flagged as offline replay
		},
	})

	if err := srv.SyncOfflineResults(stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stream.closed.Accepted != 0 {
		t.Errorf("expected 0 accepted (rejected), got %d", stream.closed.Accepted)
	}
	if len(stream.closed.RejectedTaskIds) != 1 || stream.closed.RejectedTaskIds[0] != "task-done" {
		t.Errorf("expected task-done in rejected IDs, got %v", stream.closed.RejectedTaskIds)
	}
}

func TestSyncOfflineResults_AlreadyFailed_Rejected(t *testing.T) {
	// A task that is already "failed" and not marked offline → rejected.
	stub := &syncStub{
		task: &db.Task{ID: "task-failed", Status: "failed"},
	}
	srv := newSyncServer(stub)

	stream := newFakeSyncStream([]*pb.TaskResult{
		{TaskId: "task-failed", JobId: "job-x", Success: false, OfflineResult: false},
	})

	if err := srv.SyncOfflineResults(stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stream.closed.RejectedTaskIds) != 1 {
		t.Errorf("expected task-failed in rejected IDs, got %v", stream.closed.RejectedTaskIds)
	}
}

func TestSyncOfflineResults_OfflineFlagOverridesRejection(t *testing.T) {
	// When OfflineResult=true, even a completed task should be re-processed.
	stub := &syncStub{
		task: &db.Task{ID: "task-replay", Status: "completed"},
		job:  allDoneSyncJob("job-replay"),
	}
	srv := newSyncServer(stub)

	stream := newFakeSyncStream([]*pb.TaskResult{
		{
			TaskId:        "task-replay",
			JobId:         "job-replay",
			Success:       true,
			OfflineResult: true, // force re-process
		},
	})

	if err := srv.SyncOfflineResults(stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stream.closed.Accepted != 1 {
		t.Errorf("expected 1 accepted, got %d", stream.closed.Accepted)
	}
}

func TestSyncOfflineResults_RecvError(t *testing.T) {
	srv := newSyncServer(&syncStub{})

	stream := newFakeSyncStream(nil)
	stream.recvErr = errors.New("network error")

	err := srv.SyncOfflineResults(stream)
	if err == nil {
		t.Fatal("expected error on Recv failure")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Errorf("expected Internal error, got %v", err)
	}
}

func TestSyncOfflineResults_GetTaskByIDError(t *testing.T) {
	stub := &syncStub{
		taskErr: errors.New("db unavailable"),
	}
	srv := newSyncServer(stub)

	stream := newFakeSyncStream([]*pb.TaskResult{
		{TaskId: "task-notfound", JobId: "job-x", Success: true, OfflineResult: true},
	})

	err := srv.SyncOfflineResults(stream)
	if err == nil {
		t.Fatal("expected error when GetTaskByID fails")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Errorf("expected Internal error, got %v", err)
	}
}

func TestSyncOfflineResults_UpdateJobTaskCountsError(t *testing.T) {
	stub := &syncStub{
		task:            &db.Task{ID: "task-cnt", Status: "running"},
		job:             allDoneSyncJob("job-cnt"),
		updateCountsErr: errors.New("db write error"),
	}
	srv := newSyncServer(stub)

	stream := newFakeSyncStream([]*pb.TaskResult{
		{TaskId: "task-cnt", JobId: "job-cnt", Success: true, OfflineResult: true},
	})

	err := srv.SyncOfflineResults(stream)
	if err == nil {
		t.Fatal("expected error when UpdateJobTaskCounts fails")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Errorf("expected Internal error, got %v", err)
	}
}

func TestSyncOfflineResults_GetJobError_AfterCounts(t *testing.T) {
	stub := &syncStub{
		task:   &db.Task{ID: "task-jberr", Status: "running"},
		jobErr: errors.New("db unavailable"),
	}
	srv := newSyncServer(stub)

	stream := newFakeSyncStream([]*pb.TaskResult{
		{TaskId: "task-jberr", JobId: "job-jberr", Success: true, OfflineResult: true},
	})

	err := srv.SyncOfflineResults(stream)
	if err == nil {
		t.Fatal("expected error when GetJobByID fails during checkJobCompletion")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Errorf("expected Internal error, got %v", err)
	}
}

func TestSyncOfflineResults_MultipleResults_MixedOutcomes(t *testing.T) {
	// Run two sequential syncs to verify accept/reject accounting is independent.
	t.Run("accepted", func(t *testing.T) {
		stub := &syncStub{
			task: &db.Task{ID: "t1", Status: "running"},
			job:  allDoneSyncJob("job-mixed"),
		}
		srv := newSyncServer(stub)

		stream := newFakeSyncStream([]*pb.TaskResult{
			{TaskId: "t1", JobId: "job-mixed", Success: true, OfflineResult: true},
		})
		if err := srv.SyncOfflineResults(stream); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stream.closed.Accepted != 1 {
			t.Errorf("expected 1 accepted, got %d", stream.closed.Accepted)
		}
		if len(stream.closed.RejectedTaskIds) != 0 {
			t.Errorf("expected 0 rejected, got %v", stream.closed.RejectedTaskIds)
		}
	})

	t.Run("rejected_already_completed", func(t *testing.T) {
		stub := &syncStub{
			task: &db.Task{ID: "t2", Status: "completed"},
		}
		srv := newSyncServer(stub)

		stream := newFakeSyncStream([]*pb.TaskResult{
			{TaskId: "t2", JobId: "job-mixed", Success: true, OfflineResult: false},
		})
		if err := srv.SyncOfflineResults(stream); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stream.closed.Accepted != 0 {
			t.Errorf("expected 0 accepted, got %d", stream.closed.Accepted)
		}
		if len(stream.closed.RejectedTaskIds) != 1 || stream.closed.RejectedTaskIds[0] != "t2" {
			t.Errorf("expected t2 in rejected IDs, got %v", stream.closed.RejectedTaskIds)
		}
	})
}

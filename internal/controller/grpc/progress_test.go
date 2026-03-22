package grpc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/badskater/encodeswarmr/internal/controller/config"
	"github.com/badskater/encodeswarmr/internal/controller/webhooks"
	"github.com/badskater/encodeswarmr/internal/db/teststore"
	pb "github.com/badskater/encodeswarmr/internal/proto/encoderv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ---------------------------------------------------------------------------
// Fake client-streaming server for ReportProgress
// ---------------------------------------------------------------------------

type fakeProgressStream struct {
	grpc.ServerStream
	updates []*pb.ProgressUpdate
	recvIdx int
	recvErr error
	closed  *pb.Ack
	ctx     context.Context
}

func newFakeProgressStream(updates []*pb.ProgressUpdate) *fakeProgressStream {
	return &fakeProgressStream{updates: updates, ctx: context.Background()}
}

func (s *fakeProgressStream) Context() context.Context { return s.ctx }
func (s *fakeProgressStream) SetHeader(_ metadata.MD) error  { return nil }
func (s *fakeProgressStream) SendHeader(_ metadata.MD) error { return nil }
func (s *fakeProgressStream) SetTrailer(_ metadata.MD)       {}

func (s *fakeProgressStream) Recv() (*pb.ProgressUpdate, error) {
	if s.recvErr != nil {
		return nil, s.recvErr
	}
	if s.recvIdx >= len(s.updates) {
		return nil, io.EOF
	}
	u := s.updates[s.recvIdx]
	s.recvIdx++
	return u, nil
}

func (s *fakeProgressStream) SendAndClose(ack *pb.Ack) error {
	s.closed = ack
	return nil
}

// ---------------------------------------------------------------------------
// Stub
// ---------------------------------------------------------------------------

type progressStub struct {
	teststore.Stub
	updateStatusErr     error
	updatedTaskIDs      []string
	updatedTaskStatuses []string
}

func (s *progressStub) UpdateTaskStatus(_ context.Context, id, st string) error {
	s.updatedTaskIDs = append(s.updatedTaskIDs, id)
	s.updatedTaskStatuses = append(s.updatedTaskStatuses, st)
	return s.updateStatusErr
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newProgressServer(stub *progressStub) *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wh := webhooks.New(stub, webhooks.Config{}, logger)
	return &Server{
		store:    stub,
		cfg:      &config.GRPCConfig{},
		agentCfg: &config.AgentConfig{},
		logger:   logger,
		webhooks: wh,
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestReportProgress_EmptyStream(t *testing.T) {
	stub := &progressStub{}
	srv := newProgressServer(stub)

	stream := newFakeProgressStream(nil)
	err := srv.ReportProgress(stream)
	if err != nil {
		t.Fatalf("unexpected error on empty stream: %v", err)
	}
	if stream.closed == nil || !stream.closed.Ok {
		t.Error("expected SendAndClose with ok=true")
	}
	if len(stub.updatedTaskIDs) != 0 {
		t.Errorf("expected 0 UpdateTaskStatus calls, got %d", len(stub.updatedTaskIDs))
	}
}

func TestReportProgress_FirstUpdateSetsRunning(t *testing.T) {
	stub := &progressStub{}
	srv := newProgressServer(stub)

	stream := newFakeProgressStream([]*pb.ProgressUpdate{
		{TaskId: "task-p1", JobId: "job-p1", Frame: 100, TotalFrames: 1000},
	})

	if err := srv.ReportProgress(stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stub.updatedTaskIDs) != 1 {
		t.Fatalf("expected 1 UpdateTaskStatus call, got %d", len(stub.updatedTaskIDs))
	}
	if stub.updatedTaskIDs[0] != "task-p1" {
		t.Errorf("UpdateTaskStatus called with %q, want %q", stub.updatedTaskIDs[0], "task-p1")
	}
	if stub.updatedTaskStatuses[0] != "running" {
		t.Errorf("status = %q, want %q", stub.updatedTaskStatuses[0], "running")
	}
}

func TestReportProgress_DuplicateTaskID_OnlyOneStatusUpdate(t *testing.T) {
	// Multiple progress updates for the same task should only trigger one
	// UpdateTaskStatus call.
	stub := &progressStub{}
	srv := newProgressServer(stub)

	stream := newFakeProgressStream([]*pb.ProgressUpdate{
		{TaskId: "task-dup", Frame: 100, TotalFrames: 1000},
		{TaskId: "task-dup", Frame: 200, TotalFrames: 1000},
		{TaskId: "task-dup", Frame: 300, TotalFrames: 1000},
	})

	if err := srv.ReportProgress(stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stub.updatedTaskIDs) != 1 {
		t.Errorf("expected exactly 1 status update for same task, got %d", len(stub.updatedTaskIDs))
	}
}

func TestReportProgress_MultipleDistinctTasks(t *testing.T) {
	stub := &progressStub{}
	srv := newProgressServer(stub)

	stream := newFakeProgressStream([]*pb.ProgressUpdate{
		{TaskId: "task-a", Frame: 10},
		{TaskId: "task-b", Frame: 20},
		{TaskId: "task-a", Frame: 30}, // second update for task-a — no extra status call
	})

	if err := srv.ReportProgress(stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only task-a and task-b should each trigger one status update.
	if len(stub.updatedTaskIDs) != 2 {
		t.Errorf("expected 2 status updates (one per task), got %d: %v", len(stub.updatedTaskIDs), stub.updatedTaskIDs)
	}
}

func TestReportProgress_RecvError(t *testing.T) {
	stub := &progressStub{}
	srv := newProgressServer(stub)

	stream := newFakeProgressStream(nil)
	stream.recvErr = errors.New("network error")

	err := srv.ReportProgress(stream)
	if err == nil {
		t.Fatal("expected error on Recv failure")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Errorf("expected Internal error, got %v", err)
	}
}

func TestReportProgress_UpdateTaskStatusError(t *testing.T) {
	stub := &progressStub{updateStatusErr: errors.New("db write error")}
	srv := newProgressServer(stub)

	stream := newFakeProgressStream([]*pb.ProgressUpdate{
		{TaskId: "task-err", Frame: 50},
	})

	err := srv.ReportProgress(stream)
	if err == nil {
		t.Fatal("expected error when UpdateTaskStatus fails")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Errorf("expected Internal error, got %v", err)
	}
}

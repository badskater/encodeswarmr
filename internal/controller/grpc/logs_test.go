package grpc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/badskater/distributed-encoder/internal/controller/config"
	"github.com/badskater/distributed-encoder/internal/controller/webhooks"
	"github.com/badskater/distributed-encoder/internal/db"
	"github.com/badskater/distributed-encoder/internal/db/teststore"
	pb "github.com/badskater/distributed-encoder/internal/proto/encoderv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ---------------------------------------------------------------------------
// Fake client-streaming server for StreamLogs
// ---------------------------------------------------------------------------

// fakeLogStream is a minimal grpc.ClientStreamingServer[pb.LogEntry, pb.Ack].
type fakeLogStream struct {
	grpc.ServerStream
	entries  []*pb.LogEntry
	recvIdx  int
	recvErr  error // injected error returned on the next Recv call
	closed   *pb.Ack
	ctx      context.Context
}

func newFakeLogStream(entries []*pb.LogEntry) *fakeLogStream {
	return &fakeLogStream{entries: entries, ctx: context.Background()}
}

func (s *fakeLogStream) Context() context.Context { return s.ctx }
func (s *fakeLogStream) SetHeader(_ metadata.MD) error  { return nil }
func (s *fakeLogStream) SendHeader(_ metadata.MD) error { return nil }
func (s *fakeLogStream) SetTrailer(_ metadata.MD)       {}

func (s *fakeLogStream) Recv() (*pb.LogEntry, error) {
	if s.recvErr != nil {
		return nil, s.recvErr
	}
	if s.recvIdx >= len(s.entries) {
		return nil, io.EOF
	}
	entry := s.entries[s.recvIdx]
	s.recvIdx++
	return entry, nil
}

func (s *fakeLogStream) SendAndClose(ack *pb.Ack) error {
	s.closed = ack
	return nil
}

// ---------------------------------------------------------------------------
// Stub
// ---------------------------------------------------------------------------

type logsStub struct {
	teststore.Stub
	insertErr    error
	insertCalls  int
	insertParams []db.InsertTaskLogParams
}

func (s *logsStub) InsertTaskLog(_ context.Context, p db.InsertTaskLogParams) error {
	s.insertCalls++
	s.insertParams = append(s.insertParams, p)
	return s.insertErr
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newLogsServer(stub *logsStub) *Server {
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

func TestStreamLogs_EmptyStream(t *testing.T) {
	stub := &logsStub{}
	srv := newLogsServer(stub)

	stream := newFakeLogStream(nil)
	err := srv.StreamLogs(stream)
	if err != nil {
		t.Fatalf("unexpected error on empty stream: %v", err)
	}
	if stream.closed == nil || !stream.closed.Ok {
		t.Error("expected SendAndClose with ok=true")
	}
	if stub.insertCalls != 0 {
		t.Errorf("expected 0 inserts for empty stream, got %d", stub.insertCalls)
	}
}

func TestStreamLogs_SingleEntry(t *testing.T) {
	stub := &logsStub{}
	srv := newLogsServer(stub)

	entries := []*pb.LogEntry{
		{
			TaskId:  "task-1",
			JobId:   "job-1",
			Stream:  "stdout",
			Level:   "info",
			Message: "encoding frame 100",
		},
	}
	stream := newFakeLogStream(entries)

	err := srv.StreamLogs(stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.insertCalls != 1 {
		t.Errorf("expected 1 insert, got %d", stub.insertCalls)
	}
	p := stub.insertParams[0]
	if p.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", p.TaskID, "task-1")
	}
	if p.Stream != "stdout" {
		t.Errorf("Stream = %q, want %q", p.Stream, "stdout")
	}
	if p.Message != "encoding frame 100" {
		t.Errorf("Message = %q, want %q", p.Message, "encoding frame 100")
	}
}

func TestStreamLogs_MultipleEntries(t *testing.T) {
	stub := &logsStub{}
	srv := newLogsServer(stub)

	entries := make([]*pb.LogEntry, 5)
	for i := range entries {
		entries[i] = &pb.LogEntry{
			TaskId:  "task-multi",
			JobId:   "job-multi",
			Stream:  "stderr",
			Message: "line",
		}
	}

	stream := newFakeLogStream(entries)
	if err := srv.StreamLogs(stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.insertCalls != 5 {
		t.Errorf("expected 5 inserts, got %d", stub.insertCalls)
	}
}

func TestStreamLogs_WithTimestamp(t *testing.T) {
	stub := &logsStub{}
	srv := newLogsServer(stub)

	ts := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	entries := []*pb.LogEntry{
		{
			TaskId:    "task-ts",
			Message:   "with timestamp",
			Timestamp: timestamppb.New(ts),
		},
	}

	stream := newFakeLogStream(entries)
	if err := srv.StreamLogs(stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.insertParams[0].LoggedAt == nil {
		t.Fatal("expected non-nil LoggedAt")
	}
	if !stub.insertParams[0].LoggedAt.Equal(ts) {
		t.Errorf("LoggedAt = %v, want %v", stub.insertParams[0].LoggedAt, ts)
	}
}

func TestStreamLogs_NoTimestamp_UsesNow(t *testing.T) {
	stub := &logsStub{}
	srv := newLogsServer(stub)

	before := time.Now().Add(-time.Second)
	entries := []*pb.LogEntry{
		{TaskId: "task-nots", Message: "no timestamp"},
	}

	stream := newFakeLogStream(entries)
	if err := srv.StreamLogs(stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	loggedAt := stub.insertParams[0].LoggedAt
	if loggedAt == nil {
		t.Fatal("expected non-nil LoggedAt even without proto timestamp")
	}
	if loggedAt.Before(before) {
		t.Errorf("LoggedAt %v should be after %v", loggedAt, before)
	}
}

func TestStreamLogs_InsertError(t *testing.T) {
	stub := &logsStub{insertErr: errors.New("db write error")}
	srv := newLogsServer(stub)

	entries := []*pb.LogEntry{
		{TaskId: "task-err", Message: "fails on insert"},
	}
	stream := newFakeLogStream(entries)

	err := srv.StreamLogs(stream)
	if err == nil {
		t.Fatal("expected error when InsertTaskLog fails")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Errorf("expected Internal error, got %v", err)
	}
}

func TestStreamLogs_RecvError(t *testing.T) {
	stub := &logsStub{}
	srv := newLogsServer(stub)

	stream := newFakeLogStream(nil)
	stream.recvErr = errors.New("network error")

	err := srv.StreamLogs(stream)
	if err == nil {
		t.Fatal("expected error on Recv failure")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Errorf("expected Internal error, got %v", err)
	}
}

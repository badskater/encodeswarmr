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
// Stub
// ---------------------------------------------------------------------------

type heartbeatStub struct {
	teststore.Stub
	// UpdateAgentHeartbeat control
	heartbeatErr     error
	heartbeatCalled  bool
	heartbeatParams  db.UpdateAgentHeartbeatParams
	// GetAgentByID control
	agent    *db.Agent
	agentErr error
}

func (s *heartbeatStub) UpdateAgentHeartbeat(_ context.Context, p db.UpdateAgentHeartbeatParams) error {
	s.heartbeatCalled = true
	s.heartbeatParams = p
	return s.heartbeatErr
}

func (s *heartbeatStub) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return s.agent, s.agentErr
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newHeartbeatServer(stub *heartbeatStub) *Server {
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

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestHeartbeat_MissingAgentID(t *testing.T) {
	srv := newHeartbeatServer(&heartbeatStub{})
	_, err := srv.Heartbeat(context.Background(), &pb.HeartbeatReq{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestHeartbeat_SuccessPath_IdleAgent(t *testing.T) {
	stub := &heartbeatStub{
		agent: &db.Agent{ID: "agent-1", Status: "idle"},
	}
	srv := newHeartbeatServer(stub)

	resp, err := srv.Heartbeat(context.Background(), &pb.HeartbeatReq{
		AgentId: "agent-1",
		State:   pb.AgentState_AGENT_STATE_IDLE,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Drain || resp.Disabled {
		t.Errorf("expected drain=false disabled=false for idle agent, got drain=%v disabled=%v", resp.Drain, resp.Disabled)
	}
	if resp.ServerTime == nil {
		t.Error("expected non-nil ServerTime")
	}
	if !stub.heartbeatCalled {
		t.Error("UpdateAgentHeartbeat was not called")
	}
	if stub.heartbeatParams.ID != "agent-1" {
		t.Errorf("heartbeat params ID = %q, want %q", stub.heartbeatParams.ID, "agent-1")
	}
	if stub.heartbeatParams.Status != "idle" {
		t.Errorf("heartbeat params Status = %q, want %q", stub.heartbeatParams.Status, "idle")
	}
}

func TestHeartbeat_DrainFlag(t *testing.T) {
	stub := &heartbeatStub{
		agent: &db.Agent{ID: "agent-2", Status: "draining"},
	}
	srv := newHeartbeatServer(stub)

	resp, err := srv.Heartbeat(context.Background(), &pb.HeartbeatReq{
		AgentId: "agent-2",
		State:   pb.AgentState_AGENT_STATE_BUSY,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Drain {
		t.Error("expected Drain=true for draining agent")
	}
	if resp.Disabled {
		t.Error("expected Disabled=false for draining agent")
	}
}

func TestHeartbeat_DisabledFlag(t *testing.T) {
	stub := &heartbeatStub{
		agent: &db.Agent{ID: "agent-3", Status: "disabled"},
	}
	srv := newHeartbeatServer(stub)

	resp, err := srv.Heartbeat(context.Background(), &pb.HeartbeatReq{
		AgentId: "agent-3",
		State:   pb.AgentState_AGENT_STATE_IDLE,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Drain {
		t.Error("expected Drain=false for disabled agent")
	}
	if !resp.Disabled {
		t.Error("expected Disabled=true for disabled agent")
	}
}

func TestHeartbeat_WithMetrics(t *testing.T) {
	stub := &heartbeatStub{
		agent: &db.Agent{ID: "agent-4", Status: "busy"},
	}
	srv := newHeartbeatServer(stub)

	_, err := srv.Heartbeat(context.Background(), &pb.HeartbeatReq{
		AgentId: "agent-4",
		State:   pb.AgentState_AGENT_STATE_BUSY,
		Metrics: &pb.AgentMetrics{
			CpuPercent:  45.5,
			RamPercent:  60.0,
			GpuPercent:  80.0,
			GpuVramPct:  50.0,
			DiskFreeMib: 10240,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify metrics were included in heartbeat params.
	m := stub.heartbeatParams.Metrics
	if m == nil {
		t.Fatal("expected non-nil Metrics in heartbeat params")
	}
	if m["cpu_percent"] != float32(45.5) {
		t.Errorf("cpu_percent = %v, want %v", m["cpu_percent"], float32(45.5))
	}
}

func TestHeartbeat_UpdateAgentHeartbeatError(t *testing.T) {
	stub := &heartbeatStub{
		heartbeatErr: errors.New("db write error"),
	}
	srv := newHeartbeatServer(stub)

	_, err := srv.Heartbeat(context.Background(), &pb.HeartbeatReq{AgentId: "agent-5"})
	if err == nil {
		t.Fatal("expected error when UpdateAgentHeartbeat fails")
	}
}

func TestHeartbeat_GetAgentByIDError(t *testing.T) {
	stub := &heartbeatStub{
		agentErr: errors.New("agent not found"),
	}
	srv := newHeartbeatServer(stub)

	_, err := srv.Heartbeat(context.Background(), &pb.HeartbeatReq{AgentId: "agent-6"})
	if err == nil {
		t.Fatal("expected error when GetAgentByID fails")
	}
}

// ---------------------------------------------------------------------------
// mapAgentState coverage
// ---------------------------------------------------------------------------

func TestMapAgentState(t *testing.T) {
	cases := []struct {
		input pb.AgentState
		want  string
	}{
		{pb.AgentState_AGENT_STATE_IDLE, "idle"},
		{pb.AgentState_AGENT_STATE_BUSY, "busy"},
		{pb.AgentState_AGENT_STATE_DRAINING, "busy"},
		{pb.AgentState_AGENT_STATE_OFFLINE, "offline"},
		{pb.AgentState_AGENT_STATE_UNSPECIFIED, "idle"}, // default branch
	}
	for _, tc := range cases {
		got := mapAgentState(tc.input)
		if got != tc.want {
			t.Errorf("mapAgentState(%v) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

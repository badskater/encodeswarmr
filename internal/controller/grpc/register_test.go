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
)

// ---------------------------------------------------------------------------
// Stub
// ---------------------------------------------------------------------------

type registerStub struct {
	teststore.Stub
	// UpsertAgent control
	agent        *db.Agent
	upsertErr    error
	upsertParams db.UpsertAgentParams
	// UpdateAgentStatus control
	updateStatusErr    error
	updatedAgentID     string
	updatedAgentStatus string
}

func (s *registerStub) UpsertAgent(_ context.Context, p db.UpsertAgentParams) (*db.Agent, error) {
	s.upsertParams = p
	return s.agent, s.upsertErr
}

func (s *registerStub) UpdateAgentStatus(_ context.Context, id, st string) error {
	s.updatedAgentID = id
	s.updatedAgentStatus = st
	return s.updateStatusErr
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newRegisterServer(stub *registerStub, autoApprove bool) *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wh := webhooks.New(stub, webhooks.Config{}, logger)
	return &Server{
		store:    stub,
		cfg:      &config.GRPCConfig{},
		agentCfg: &config.AgentConfig{AutoApprove: autoApprove},
		logger:   logger,
		webhooks: wh,
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestRegister_Success_PendingApproval(t *testing.T) {
	stub := &registerStub{
		agent: &db.Agent{
			ID:       "agent-reg-1",
			Name:     "enc-01",
			Hostname: "enc-01",
			Status:   "pending_approval",
		},
	}
	srv := newRegisterServer(stub, false)

	resp, err := srv.Register(context.Background(), &pb.AgentInfo{
		Hostname:     "enc-01",
		IpAddress:    "10.0.0.1",
		AgentVersion: "1.0.0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Ok {
		t.Error("expected resp.Ok=true")
	}
	if resp.Approved {
		t.Error("expected Approved=false for pending agent")
	}
	if resp.AgentId != "agent-reg-1" {
		t.Errorf("AgentId = %q, want %q", resp.AgentId, "agent-reg-1")
	}
	if resp.Message != "pending approval" {
		t.Errorf("Message = %q, want %q", resp.Message, "pending approval")
	}
}

func TestRegister_Success_ApprovedAgent(t *testing.T) {
	stub := &registerStub{
		agent: &db.Agent{
			ID:       "agent-reg-2",
			Hostname: "enc-02",
			Status:   "idle", // already approved
		},
	}
	srv := newRegisterServer(stub, false)

	resp, err := srv.Register(context.Background(), &pb.AgentInfo{
		Hostname: "enc-02",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Approved {
		t.Error("expected Approved=true for idle agent")
	}
	if resp.Message != "registered" {
		t.Errorf("Message = %q, want %q", resp.Message, "registered")
	}
}

func TestRegister_AutoApprove(t *testing.T) {
	stub := &registerStub{
		agent: &db.Agent{
			ID:       "agent-reg-3",
			Hostname: "enc-03",
			Status:   "pending_approval",
		},
	}
	srv := newRegisterServer(stub, true) // auto-approve enabled

	resp, err := srv.Register(context.Background(), &pb.AgentInfo{
		Hostname: "enc-03",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Approved {
		t.Error("expected Approved=true after auto-approve")
	}
	if stub.updatedAgentID != "agent-reg-3" {
		t.Errorf("UpdateAgentStatus not called with correct ID, got %q", stub.updatedAgentID)
	}
	if stub.updatedAgentStatus != "idle" {
		t.Errorf("UpdateAgentStatus not called with 'idle', got %q", stub.updatedAgentStatus)
	}
}

func TestRegister_AutoApprove_UpdateStatusError(t *testing.T) {
	stub := &registerStub{
		agent: &db.Agent{
			ID:     "agent-reg-4",
			Status: "pending_approval",
		},
		updateStatusErr: errors.New("db write error"),
	}
	srv := newRegisterServer(stub, true)

	_, err := srv.Register(context.Background(), &pb.AgentInfo{Hostname: "enc-04"})
	if err == nil {
		t.Fatal("expected error when UpdateAgentStatus fails")
	}
}

func TestRegister_UpsertAgentError(t *testing.T) {
	stub := &registerStub{
		upsertErr: errors.New("db constraint error"),
	}
	srv := newRegisterServer(stub, false)

	_, err := srv.Register(context.Background(), &pb.AgentInfo{Hostname: "enc-05"})
	if err == nil {
		t.Fatal("expected error when UpsertAgent fails")
	}
}

func TestRegister_WithGPU(t *testing.T) {
	stub := &registerStub{
		agent: &db.Agent{
			ID:       "agent-gpu",
			Hostname: "enc-gpu",
			Status:   "idle",
		},
	}
	srv := newRegisterServer(stub, false)

	_, err := srv.Register(context.Background(), &pb.AgentInfo{
		Hostname: "enc-gpu",
		Gpu: &pb.GPUCapabilities{
			Vendor: "nvidia",
			Model:  "RTX 4090",
			Nvenc:  true,
			Qsv:    false,
			Amf:    false,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.upsertParams.GPUVendor != "nvidia" {
		t.Errorf("GPUVendor = %q, want %q", stub.upsertParams.GPUVendor, "nvidia")
	}
	if !stub.upsertParams.NVENC {
		t.Error("expected NVENC=true")
	}
	if !stub.upsertParams.GPUEnabled {
		t.Error("expected GPUEnabled=true")
	}
}

func TestRegister_VNCPortTag(t *testing.T) {
	stub := &registerStub{
		agent: &db.Agent{ID: "agent-vnc", Hostname: "enc-vnc", Status: "idle"},
	}
	srv := newRegisterServer(stub, false)

	_, err := srv.Register(context.Background(), &pb.AgentInfo{
		Hostname: "enc-vnc",
		Tags:     []string{"gpu", "__vnc_port=5900", "4k"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// __vnc_port must be stripped from visible tags.
	for _, tag := range stub.upsertParams.Tags {
		if tag == "__vnc_port=5900" {
			t.Error("__vnc_port tag should be stripped from visible tags")
		}
	}
	// The remaining visible tags must still be present.
	found := false
	for _, tag := range stub.upsertParams.Tags {
		if tag == "gpu" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'gpu' tag to remain in visible tags")
	}
	if stub.upsertParams.VNCPort != 5900 {
		t.Errorf("VNCPort = %d, want 5900", stub.upsertParams.VNCPort)
	}
}

func TestRegister_InvalidVNCPortTag(t *testing.T) {
	// An invalid __vnc_port value should not panic; vncPort stays 0.
	stub := &registerStub{
		agent: &db.Agent{ID: "agent-vncbad", Status: "idle"},
	}
	srv := newRegisterServer(stub, false)

	_, err := srv.Register(context.Background(), &pb.AgentInfo{
		Hostname: "enc-vncbad",
		Tags:     []string{"__vnc_port=notanumber"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.upsertParams.VNCPort != 0 {
		t.Errorf("VNCPort = %d, want 0 for invalid port", stub.upsertParams.VNCPort)
	}
}

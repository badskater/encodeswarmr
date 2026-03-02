package grpc

import (
	"context"
	"fmt"

	pb "github.com/badskater/distributed-encoder/internal/proto/encoderv1"

	"github.com/badskater/distributed-encoder/internal/db"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Heartbeat handles the periodic heartbeat RPC from agents.
func (s *Server) Heartbeat(ctx context.Context, req *pb.HeartbeatReq) (*pb.HeartbeatResp, error) {
	if req.GetAgentId() == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}

	agentStatus := mapAgentState(req.GetState())

	var metricsMap map[string]any
	if m := req.GetMetrics(); m != nil {
		metricsMap = map[string]any{
			"cpu_percent":  m.GetCpuPercent(),
			"ram_percent":  m.GetRamPercent(),
			"gpu_percent":  m.GetGpuPercent(),
			"gpu_vram_pct": m.GetGpuVramPct(),
			"disk_free_mib": m.GetDiskFreeMib(),
		}
	}

	err := s.store.UpdateAgentHeartbeat(ctx, db.UpdateAgentHeartbeatParams{
		ID:      req.GetAgentId(),
		Status:  agentStatus,
		Metrics: metricsMap,
	})
	if err != nil {
		return nil, fmt.Errorf("grpc heartbeat: %w", err)
	}

	// Fetch agent to check drain/disabled flags.
	agent, err := s.store.GetAgentByID(ctx, req.GetAgentId())
	if err != nil {
		return nil, fmt.Errorf("grpc heartbeat get agent: %w", err)
	}

	return &pb.HeartbeatResp{
		Drain:      agent.Status == "draining",
		Disabled:   agent.Status == "disabled",
		ServerTime: timestamppb.Now(),
	}, nil
}

// mapAgentState converts a proto AgentState enum to a database status string.
func mapAgentState(state pb.AgentState) string {
	switch state {
	case pb.AgentState_AGENT_STATE_IDLE:
		return "idle"
	case pb.AgentState_AGENT_STATE_BUSY:
		return "busy"
	case pb.AgentState_AGENT_STATE_DRAINING:
		return "busy" // draining maps to busy in DB
	case pb.AgentState_AGENT_STATE_OFFLINE:
		return "offline"
	default:
		return "idle"
	}
}

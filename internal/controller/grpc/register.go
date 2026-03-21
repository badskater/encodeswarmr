package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	pb "github.com/badskater/encodeswarmr/internal/proto/encoderv1"

	"github.com/badskater/encodeswarmr/internal/controller/webhooks"
	"github.com/badskater/encodeswarmr/internal/db"
)

// Register handles the agent registration RPC. It upserts the agent in the
// database and optionally auto-approves it based on the controller config.
func (s *Server) Register(ctx context.Context, req *pb.AgentInfo) (*pb.RegisterResponse, error) {
	// Extract the reserved __vnc_port=<port> tag if present, then remove it
	// from the visible tags stored in the DB. This avoids a proto change while
	// still allowing agents to report their VNC port.
	rawTags := req.GetTags()
	vncPort := 0
	visibleTags := make([]string, 0, len(rawTags))
	for _, t := range rawTags {
		if strings.HasPrefix(t, "__vnc_port=") {
			if p, err := strconv.Atoi(strings.TrimPrefix(t, "__vnc_port=")); err == nil {
				vncPort = p
			}
		} else {
			visibleTags = append(visibleTags, t)
		}
	}

	params := db.UpsertAgentParams{
		Name:         req.GetHostname(), // use hostname as the agent name
		Hostname:     req.GetHostname(),
		IPAddress:    req.GetIpAddress(),
		Tags:         visibleTags,
		AgentVersion: req.GetAgentVersion(),
		OSVersion:    req.GetOsVersion(),
		CPUCount:     req.GetCpuCount(),
		RAMMIB:       req.GetRamMib(),
		VNCPort:      vncPort,
	}

	if gpu := req.GetGpu(); gpu != nil {
		params.GPUVendor = gpu.GetVendor()
		params.GPUModel = gpu.GetModel()
		params.GPUEnabled = true
		params.NVENC = gpu.GetNvenc()
		params.QSV = gpu.GetQsv()
		params.AMF = gpu.GetAmf()
	}

	agent, err := s.store.UpsertAgent(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("grpc register: %w", err)
	}

	// Auto-approve if configured and agent is still pending.
	if s.agentCfg.AutoApprove && agent.Status == "pending_approval" {
		if err := s.store.UpdateAgentStatus(ctx, agent.ID, "idle"); err != nil {
			return nil, fmt.Errorf("grpc register auto-approve: %w", err)
		}
		agent.Status = "idle"
	}

	approved := agent.Status != "pending_approval"
	msg := "registered"
	if !approved {
		msg = "pending approval"
	}

	s.logger.Log(ctx, slog.LevelInfo, "agent registered",
		"agent_id", agent.ID,
		"hostname", agent.Hostname,
		"approved", approved,
	)

	s.webhooks.Emit(ctx, webhooks.Event{
		Type: "agent.registered",
		Payload: map[string]any{
			"agent_id":  agent.ID,
			"agent_name": agent.Name,
			"hostname":  agent.Hostname,
			"approved":  approved,
		},
	})

	return &pb.RegisterResponse{
		Ok:       true,
		AgentId:  agent.ID,
		Approved: approved,
		Message:  msg,
	}, nil
}

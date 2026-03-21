package service

import (
	"context"
	"log/slog"

	agentcfg "github.com/badskater/distributed-encoder/internal/agent/config"
	pb "github.com/badskater/distributed-encoder/internal/proto/encoderv1"
)

// RunForTest starts the agent runner lifecycle in the calling goroutine.
// It is intended for integration tests that need to exercise the full
// agent→controller flow without going through the CLI/service layer.
// The function blocks until ctx is cancelled or a fatal error occurs.
func RunForTest(ctx context.Context, cfg *agentcfg.Config, logger *slog.Logger) error {
	r := &runner{
		cfg:   cfg,
		log:   logger,
		state: pb.AgentState_AGENT_STATE_IDLE,
	}
	return r.run(ctx)
}

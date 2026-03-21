//go:build integration

package testharness

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	agentcfg "github.com/badskater/distributed-encoder/internal/agent/config"
	"github.com/badskater/distributed-encoder/internal/agent/service"
)

// TestAgent holds references to a running in-process agent.
type TestAgent struct {
	// Cancel stops the agent's context, causing it to shut down.
	Cancel context.CancelFunc
	// Done receives the return value of service.RunForTest when the agent exits.
	Done <-chan error
}

// StartAgent boots an in-process agent that connects to the given gRPC address.
// The agent is stopped automatically via t.Cleanup when the test ends.
func StartAgent(t *testing.T, grpcAddr string, name string) *TestAgent {
	t.Helper()
	offlineDB := filepath.Join(t.TempDir(), "offline.db")
	return StartAgentWithOfflineDB(t, grpcAddr, name, offlineDB)
}

// StartAgentWithOfflineDB boots an in-process agent using the given offline DB
// path. This allows tests to pre-seed the journal before the agent starts.
// The agent is stopped automatically via t.Cleanup when the test ends.
func StartAgentWithOfflineDB(t *testing.T, grpcAddr string, name string, offlineDBPath string) *TestAgent {
	t.Helper()

	cfg := &agentcfg.Config{
		Controller: agentcfg.ControllerConfig{
			Address: grpcAddr,
			TLS:     agentcfg.TLSConfig{}, // empty → plaintext / dev mode
			Reconnect: agentcfg.ReconnectConfig{
				InitialDelay: 100 * time.Millisecond,
				MaxDelay:     1 * time.Second,
				Multiplier:   2.0,
			},
		},
		Agent: agentcfg.AgentConfig{
			Hostname:          name,
			WorkDir:           t.TempDir(),
			OfflineDB:         offlineDBPath,
			HeartbeatInterval: 1 * time.Second,
			PollInterval:      500 * time.Millisecond,
			CleanupOnSuccess:  false,
		},
		GPU: agentcfg.GPUConfig{
			Enabled: false,
		},
		AllowedShares: []string{}, // empty = allow all paths (no restriction in tests)
		Logging: agentcfg.LoggingConfig{
			Level: "debug",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- service.RunForTest(ctx, cfg, logger)
	}()

	t.Cleanup(cancel)

	return &TestAgent{
		Cancel: cancel,
		Done:   done,
	}
}

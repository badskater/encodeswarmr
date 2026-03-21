//go:build integration

package testharness

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"log/slog"

	"github.com/badskater/distributed-encoder/internal/controller/api"
	"github.com/badskater/distributed-encoder/internal/controller/auth"
	"github.com/badskater/distributed-encoder/internal/controller/config"
	"github.com/badskater/distributed-encoder/internal/controller/engine"
	controllergrpc "github.com/badskater/distributed-encoder/internal/controller/grpc"
	"github.com/badskater/distributed-encoder/internal/controller/ha"
	"github.com/badskater/distributed-encoder/internal/controller/webhooks"
	"github.com/badskater/distributed-encoder/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestController holds references to a running controller stack.
type TestController struct {
	Store       db.Store
	Pool        *pgxpool.Pool
	HTTPBaseURL string // e.g., "http://127.0.0.1:54321"
	GRPCAddr    string // e.g., "127.0.0.1:54322"
	AuthSvc     *auth.Service
	Config      *config.Config
	Cancel      context.CancelFunc
}

// StartController boots a full controller (HTTP + gRPC + engine + webhooks)
// against the test Postgres. Uses random free ports.
func StartController(t *testing.T) *TestController {
	t.Helper()

	// 1. Set up Postgres: get dsn, store, pool.
	dsn, store, pool := SetupPostgres(t)

	// 2. Truncate all tables for isolation.
	TruncateAll(t, pool)

	// 3. Find two free ports.
	httpPort := freePort(t)
	grpcPort := freePort(t)

	// 4. Build config programmatically.
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:           "127.0.0.1",
			Port:           httpPort,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			AllowedOrigins: []string{"*"},
		},
		Database: config.DatabaseConfig{
			URL: dsn,
		},
		GRPC: config.GRPCConfig{
			Host: "127.0.0.1",
			Port: grpcPort,
			TLS:  config.TLSConfig{}, // plaintext
		},
		Auth: config.AuthConfig{
			SessionTTL: 1 * time.Hour,
			OIDC:       config.OIDCConfig{Enabled: false},
		},
		Agent: config.AgentConfig{
			AutoApprove:      true,
			HeartbeatTimeout: 30 * time.Second,
			DispatchInterval: 500 * time.Millisecond,
		},
		Logging: config.LoggingConfig{
			Level: "debug",
		},
	}

	// 5. Create context with cancel.
	ctx, cancel := context.WithCancel(context.Background())

	logger := slog.Default()

	// 6. Create auth service.
	authSvc, err := auth.NewService(ctx, store, &cfg.Auth, logger)
	if err != nil {
		cancel()
		t.Fatalf("testharness: new auth service: %v", err)
	}

	// 7. Create and start webhook service.
	whSvc := webhooks.New(store, webhooks.Config{
		WorkerCount:     1,
		DeliveryTimeout: 5 * time.Second,
		MaxRetries:      1,
	}, logger)
	whSvc.Start(ctx)

	// 8. Create HA leader (single-node, always leader for tests).
	leader := ha.NewLeader(pool, "test-node", logger)
	leader.Start(ctx)

	// 9 (HTTP). Create and start HTTP API server.
	apiSrv, err := api.New(store, authSvc, cfg, logger, whSvc, leader)
	if err != nil {
		cancel()
		t.Fatalf("testharness: new api server: %v", err)
	}
	go func() {
		if serveErr := apiSrv.Serve(ctx); serveErr != nil {
			// Context cancellation is expected at cleanup; only log unexpected errors.
			if ctx.Err() == nil {
				t.Logf("testharness: api serve error: %v", serveErr)
			}
		}
	}()

	// 9. Create and start gRPC server.
	grpcSrv := controllergrpc.New(store, &cfg.GRPC, &cfg.Agent, logger, whSvc)
	go func() {
		if serveErr := grpcSrv.Serve(ctx); serveErr != nil {
			if ctx.Err() == nil {
				t.Logf("testharness: grpc serve error: %v", serveErr)
			}
		}
	}()

	// 10. Create and start engine.
	eng := engine.New(store, engine.Config{
		DispatchInterval: 500 * time.Millisecond,
		StaleThreshold:   90 * time.Second,
		ScriptBaseDir:    t.TempDir(),
	}, logger)
	eng.Start(ctx)

	// 11. Poll /health until 200 (5s timeout, 50ms interval).
	httpBase := fmt.Sprintf("http://127.0.0.1:%d", httpPort)
	waitForHealth(t, httpBase, 5*time.Second)

	// 12. Register cleanup.
	t.Cleanup(func() {
		cancel()
		leader.Stop()
	})

	// 13. Return TestController.
	return &TestController{
		Store:       store,
		Pool:        pool,
		HTTPBaseURL: httpBase,
		GRPCAddr:    fmt.Sprintf("127.0.0.1:%d", grpcPort),
		AuthSvc:     authSvc,
		Config:      cfg,
		Cancel:      cancel,
	}
}

// freePort finds a free TCP port on 127.0.0.1 and returns it.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("testharness: find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

// waitForHealth polls GET <base>/health until 200 or timeout.
func waitForHealth(t *testing.T, base string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	url := base + "/health"
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:gosec,noctx
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("testharness: health check at %s did not return 200 within %s", url, timeout)
}

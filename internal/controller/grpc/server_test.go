package grpc

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/config"
	"github.com/badskater/encodeswarmr/internal/controller/webhooks"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
)

// ---------------------------------------------------------------------------
// TestNew
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	store := &resultStub{}
	grpcCfg := &config.GRPCConfig{Host: "127.0.0.1", Port: 0}
	agentCfg := &config.AgentConfig{TaskTimeoutSec: 3600}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wh := webhooks.New(store, webhooks.Config{}, logger)

	srv := New(store, grpcCfg, agentCfg, logger, wh)

	if srv == nil {
		t.Fatal("expected non-nil Server")
	}
	if srv.store == nil {
		t.Error("expected store to be set")
	}
	if srv.logger == nil {
		t.Error("expected logger to be set")
	}
}

// ---------------------------------------------------------------------------
// TestBuildServerOptions_NoTLS
// ---------------------------------------------------------------------------

func TestBuildServerOptions_NoTLS(t *testing.T) {
	cfg := &config.GRPCConfig{
		TLS: config.TLSConfig{CertFile: ""}, // empty = no TLS
	}

	opts, err := buildServerOptions(cfg)
	if err != nil {
		t.Fatalf("buildServerOptions: %v", err)
	}
	if opts == nil {
		t.Error("expected non-nil server options")
	}
	// Should have at least keepalive options.
	if len(opts) < 2 {
		t.Errorf("expected at least 2 server options, got %d", len(opts))
	}
}

// ---------------------------------------------------------------------------
// TestBuildTLSCredentials
// ---------------------------------------------------------------------------

func TestBuildTLSCredentials_EmptyCertFile(t *testing.T) {
	cfg := &config.TLSConfig{CertFile: ""}

	creds, err := buildTLSCredentials(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds != nil {
		t.Error("expected nil credentials when CertFile is empty")
	}
}

func TestBuildTLSCredentials_MissingCertFile(t *testing.T) {
	cfg := &config.TLSConfig{
		CertFile: "/nonexistent/cert.pem",
		KeyFile:  "/nonexistent/key.pem",
		CAFile:   "/nonexistent/ca.pem",
	}

	_, err := buildTLSCredentials(cfg)
	if err == nil {
		t.Fatal("expected error for missing cert file, got nil")
	}
}

// ---------------------------------------------------------------------------
// TestLoggingUnaryInterceptor
// ---------------------------------------------------------------------------

func TestLoggingUnaryInterceptor_Success(t *testing.T) {
	srv := newResultServer(&resultStub{})
	ctx := context.Background()

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
	handler := func(ctx context.Context, req any) (any, error) {
		return "response", nil
	}

	resp, err := srv.loggingUnaryInterceptor(ctx, "request", info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "response" {
		t.Errorf("resp = %v, want response", resp)
	}
}

func TestLoggingUnaryInterceptor_WithError(t *testing.T) {
	srv := newResultServer(&resultStub{})
	ctx := context.Background()

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/FailingMethod"}
	handler := func(ctx context.Context, req any) (any, error) {
		return nil, context.DeadlineExceeded
	}

	_, err := srv.loggingUnaryInterceptor(ctx, "request", info, handler)
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestLoggingStreamInterceptor
// ---------------------------------------------------------------------------

// mockServerStream is a minimal grpc.ServerStream for testing.
type mockServerStream struct {
	grpc.ServerStream
}

func (m *mockServerStream) Context() context.Context { return context.Background() }

func TestLoggingStreamInterceptor_Success(t *testing.T) {
	srv := newResultServer(&resultStub{})

	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}
	handler := func(srv any, stream grpc.ServerStream) error {
		return nil
	}

	err := srv.loggingStreamInterceptor(nil, &mockServerStream{}, info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoggingStreamInterceptor_WithError(t *testing.T) {
	srv := newResultServer(&resultStub{})

	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/FailingStream"}
	handler := func(srv any, stream grpc.ServerStream) error {
		return context.Canceled
	}

	err := srv.loggingStreamInterceptor(nil, &mockServerStream{}, info, handler)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestStructToMap
// ---------------------------------------------------------------------------

func TestStructToMap_Nil(t *testing.T) {
	result := structToMap(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
}

func TestStructToMap_WithData(t *testing.T) {
	s, err := structpb.NewStruct(map[string]any{"key": "value", "num": 42.0})
	if err != nil {
		t.Fatalf("structpb.NewStruct: %v", err)
	}

	result := structToMap(s)
	if result == nil {
		t.Fatal("expected non-nil map")
	}
	if result["key"] != "value" {
		t.Errorf("result[\"key\"] = %v, want value", result["key"])
	}
}

// ---------------------------------------------------------------------------
// TestServe_CancelledContextExits
// ---------------------------------------------------------------------------

func TestServe_CancelledContextExits(t *testing.T) {
	store := &resultStub{}
	grpcCfg := &config.GRPCConfig{
		Host: "127.0.0.1",
		Port: 0, // OS will assign a free port
	}
	agentCfg := &config.AgentConfig{TaskTimeoutSec: 3600}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wh := webhooks.New(store, webhooks.Config{}, logger)

	srv := New(store, grpcCfg, agentCfg, logger, wh)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Serve should return nil when context is cancelled gracefully.
	err := srv.Serve(ctx)
	if err != nil {
		t.Logf("Serve returned (expected on graceful shutdown): %v", err)
	}
}

package engine

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/db"
	"github.com/badskater/encodeswarmr/internal/db/teststore"
)

// ---------------------------------------------------------------------------
// Stubs for stale-agent tests
// ---------------------------------------------------------------------------

// staleStub embeds teststore.Stub and overrides MarkStaleAgents.
type staleStub struct {
	teststore.Stub
	staleCount int64
	staleErr   error
	// Capture the threshold passed to MarkStaleAgents.
	capturedThreshold time.Duration
}

func (s *staleStub) MarkStaleAgents(_ context.Context, threshold time.Duration) (int64, error) {
	s.capturedThreshold = threshold
	return s.staleCount, s.staleErr
}

// newStaleEngine creates a minimal Engine for stale-agent tests.
func newStaleEngine(t *testing.T, store db.Store, threshold time.Duration) *Engine {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := Config{
		ScriptBaseDir:  t.TempDir(),
		StaleThreshold: threshold,
	}
	return New(store, cfg, logger)
}

// ---------------------------------------------------------------------------
// checkStaleAgents tests
// ---------------------------------------------------------------------------

func TestCheckStaleAgents_StoreError(t *testing.T) {
	stub := &staleStub{staleErr: errors.New("db down")}
	e := newStaleEngine(t, stub, 2*time.Minute)

	err := e.checkStaleAgents(context.Background())
	if err == nil {
		t.Fatal("expected error from MarkStaleAgents, got nil")
	}
}

func TestCheckStaleAgents_NoStaleAgents(t *testing.T) {
	stub := &staleStub{staleCount: 0}
	e := newStaleEngine(t, stub, 90*time.Second)

	err := e.checkStaleAgents(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckStaleAgents_WithStaleAgents(t *testing.T) {
	stub := &staleStub{staleCount: 3}
	e := newStaleEngine(t, stub, 90*time.Second)

	err := e.checkStaleAgents(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckStaleAgents_PassesThreshold(t *testing.T) {
	wantThreshold := 5 * time.Minute
	stub := &staleStub{staleCount: 0}
	e := newStaleEngine(t, stub, wantThreshold)

	if err := e.checkStaleAgents(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.capturedThreshold != wantThreshold {
		t.Errorf("MarkStaleAgents called with threshold %v, want %v",
			stub.capturedThreshold, wantThreshold)
	}
}

func TestCheckStaleAgents_ZeroThreshold(t *testing.T) {
	// A zero StaleThreshold is a valid (if unusual) configuration — the
	// function must still call MarkStaleAgents without panicking.
	stub := &staleStub{staleCount: 0}
	e := newStaleEngine(t, stub, 0)

	if err := e.checkStaleAgents(context.Background()); err != nil {
		t.Fatalf("unexpected error with zero threshold: %v", err)
	}
	if stub.capturedThreshold != 0 {
		t.Errorf("expected threshold 0, got %v", stub.capturedThreshold)
	}
}

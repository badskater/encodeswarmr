package engine

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/badskater/distributed-encoder/internal/db"
	"github.com/badskater/distributed-encoder/internal/db/teststore"
)

// ---------------------------------------------------------------------------
// engineLoopStub — minimal store for Start / loop tests
// ---------------------------------------------------------------------------

// engineLoopStub satisfies db.Store via teststore.Stub.
// Its GetJobsNeedingExpansion and MarkStaleAgents methods count calls so that
// tests can verify the loop ticks correctly.
type engineLoopStub struct {
	teststore.Stub
	expandCalls int
	staleCalls  int
}

func (s *engineLoopStub) GetJobsNeedingExpansion(_ context.Context) ([]*db.Job, error) {
	s.expandCalls++
	return nil, nil
}

func (s *engineLoopStub) MarkStaleAgents(_ context.Context, _ time.Duration) (int64, error) {
	s.staleCalls++
	return 0, nil
}

// ---------------------------------------------------------------------------
// TestNew_Engine
// ---------------------------------------------------------------------------

func TestNew_Engine(t *testing.T) {
	stub := &engineLoopStub{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := Config{
		DispatchInterval: time.Millisecond,
		StaleThreshold:   time.Minute,
		ScriptBaseDir:    t.TempDir(),
	}

	e := New(stub, cfg, logger)

	if e == nil {
		t.Fatal("New() returned nil Engine")
	}
	if e.store == nil {
		t.Error("Engine.store is nil")
	}
	if e.gen == nil {
		t.Error("Engine.gen is nil")
	}
	if e.logger == nil {
		t.Error("Engine.logger is nil")
	}
}

// ---------------------------------------------------------------------------
// TestStart_LoopExitsOnContextCancel
// ---------------------------------------------------------------------------

func TestStart_LoopExitsOnContextCancel(t *testing.T) {
	stub := &engineLoopStub{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := Config{
		DispatchInterval: time.Millisecond,
		StaleThreshold:   time.Minute,
		ScriptBaseDir:    t.TempDir(),
	}

	e := New(stub, cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Start should return immediately; the loop runs in the background.
	e.Start(ctx)

	// Wait for context to expire, giving the loop time to tick at least once.
	<-ctx.Done()

	// Give the goroutine a moment to finish.
	time.Sleep(10 * time.Millisecond)

	// The loop must have called expandPendingJobs at least once during the
	// 50 ms window with a 1 ms tick interval.
	if stub.expandCalls == 0 {
		t.Error("expected expandPendingJobs to be called at least once")
	}
	if stub.staleCalls == 0 {
		t.Error("expected MarkStaleAgents to be called at least once")
	}
}

// ---------------------------------------------------------------------------
// TestStart_MultipleStartsRunIndependently
// ---------------------------------------------------------------------------

func TestStart_DoesNotBlockCaller(t *testing.T) {
	stub := &engineLoopStub{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := Config{
		DispatchInterval: time.Hour, // very long — no ticks expected
		StaleThreshold:   time.Minute,
		ScriptBaseDir:    t.TempDir(),
	}

	e := New(stub, cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// If Start blocks it will never reach the line below — test will time-out.
	done := make(chan struct{})
	go func() {
		e.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
		// good: Start returned promptly
	case <-time.After(time.Second):
		t.Fatal("Start() did not return within 1 second")
	}
}

// ---------------------------------------------------------------------------
// TestSetAnalysisRunner
// ---------------------------------------------------------------------------

func TestSetAnalysisRunner(t *testing.T) {
	stub := &engineLoopStub{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := Config{ScriptBaseDir: t.TempDir()}

	e := New(stub, cfg, logger)

	if e.analysis != nil {
		t.Error("expected nil analysis runner before SetAnalysisRunner")
	}

	runner := &noopAnalysisRunner{}
	e.SetAnalysisRunner(runner)

	if e.analysis == nil {
		t.Error("expected non-nil analysis runner after SetAnalysisRunner")
	}
}

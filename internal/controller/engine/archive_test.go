package engine

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/db/teststore"
)

// errTestEngineDB is a generic sentinel DB error shared across engine test files.
var errTestEngineDB = errors.New("db error")

// ---------------------------------------------------------------------------
// archiveStub
// ---------------------------------------------------------------------------

// archiveStub records calls to ArchiveOldJobs.
type archiveStub struct {
	teststore.Stub
	callCount int64
	returnErr error
}

func (s *archiveStub) ArchiveOldJobs(_ context.Context, _ time.Duration) (int64, error) {
	atomic.AddInt64(&s.callCount, 1)
	return 0, s.returnErr
}

// ---------------------------------------------------------------------------
// TestStartArchivalLoop_Disabled
// ---------------------------------------------------------------------------

func TestStartArchivalLoop_Disabled(t *testing.T) {
	stub := &archiveStub{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	e := New(stub, Config{ScriptBaseDir: t.TempDir()}, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	e.StartArchivalLoop(ctx, ArchiveConfig{Enabled: false})

	// Wait a bit to confirm nothing was called.
	time.Sleep(50 * time.Millisecond)
	if got := atomic.LoadInt64(&stub.callCount); got != 0 {
		t.Errorf("ArchiveOldJobs call count = %d, want 0 (disabled)", got)
	}
}

// ---------------------------------------------------------------------------
// TestStartArchivalLoop_EnabledCallsStore
// ---------------------------------------------------------------------------

func TestStartArchivalLoop_EnabledCallsStore(t *testing.T) {
	stub := &archiveStub{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	e := New(stub, Config{ScriptBaseDir: t.TempDir()}, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	e.StartArchivalLoop(ctx, ArchiveConfig{
		Enabled:       true,
		RetentionDays: 30,
	})

	// The loop calls ArchiveOldJobs immediately on start.
	// Give the goroutine time to execute.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&stub.callCount) >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if got := atomic.LoadInt64(&stub.callCount); got < 1 {
		t.Errorf("ArchiveOldJobs call count = %d, want >= 1", got)
	}
}

// ---------------------------------------------------------------------------
// TestStartArchivalLoop_DefaultRetention
// ---------------------------------------------------------------------------

func TestStartArchivalLoop_DefaultRetention(t *testing.T) {
	// When RetentionDays is 0, the loop should use the default 30 days.
	stub := &archiveStub{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	e := New(stub, Config{ScriptBaseDir: t.TempDir()}, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	e.StartArchivalLoop(ctx, ArchiveConfig{
		Enabled:       true,
		RetentionDays: 0, // should default to 30 days
	})

	// The loop should still run without panicking.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&stub.callCount) >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := atomic.LoadInt64(&stub.callCount); got < 1 {
		t.Errorf("ArchiveOldJobs not called with default retention; count = %d", got)
	}
}

// ---------------------------------------------------------------------------
// TestArchiveCompletedJobs_StoreError
// ---------------------------------------------------------------------------

func TestArchiveCompletedJobs_StoreError(t *testing.T) {
	stub := &archiveStub{returnErr: errTestEngineDB}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	e := New(stub, Config{ScriptBaseDir: t.TempDir()}, logger)

	// archiveCompletedJobs should return the store error.
	err := e.archiveCompletedJobs(context.Background(), 30*24*time.Hour)
	if err == nil {
		t.Error("expected error from ArchiveOldJobs, got nil")
	}
}

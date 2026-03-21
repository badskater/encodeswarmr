package webhooks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/badskater/distributed-encoder/internal/db"
)

// ---------------------------------------------------------------------------
// Stub store that controls ListWebhooksByEvent
// ---------------------------------------------------------------------------

type serviceStubStore struct {
	senderStubStore
	hooks    []*db.Webhook
	hooksErr error
}

func (s *serviceStubStore) ListWebhooksByEvent(_ context.Context, _ string) ([]*db.Webhook, error) {
	return s.hooks, s.hooksErr
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func startTestHTTPServer(t *testing.T, onRequest func()) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		onRequest()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ts.Close)
	return ts
}

// ---------------------------------------------------------------------------
// TestNew
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	t.Run("default worker count applied when zero", func(t *testing.T) {
		store := &serviceStubStore{}
		svc := New(store, Config{WorkerCount: 0}, discardLogger())

		if svc == nil {
			t.Fatal("expected non-nil Service")
		}
		if svc.queue == nil {
			t.Error("expected queue channel to be initialized")
		}
		// Default is 4 workers × 10 = 40 capacity
		if cap(svc.queue) != 40 {
			t.Errorf("queue capacity = %d, want 40", cap(svc.queue))
		}
	})

	t.Run("custom worker count respected", func(t *testing.T) {
		store := &serviceStubStore{}
		svc := New(store, Config{WorkerCount: 2}, discardLogger())

		// 2 workers × 10 = 20 capacity
		if cap(svc.queue) != 20 {
			t.Errorf("queue capacity = %d, want 20", cap(svc.queue))
		}
	})
}

// ---------------------------------------------------------------------------
// TestStart
// ---------------------------------------------------------------------------

func TestStart(t *testing.T) {
	t.Run("workers start and receive deliveries", func(t *testing.T) {
		var received atomic.Int32
		ts := startTestHTTPServer(t, func() { received.Add(1) })

		store := &serviceStubStore{
			hooks: []*db.Webhook{
				{ID: "wh-1", URL: ts.URL, Provider: "generic"},
			},
		}
		svc := New(store, Config{WorkerCount: 1, MaxRetries: 1, DeliveryTimeout: 5 * time.Second}, discardLogger())

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		svc.Start(ctx)

		svc.Emit(ctx, Event{Type: "test.started", Payload: map[string]any{"x": 1}})

		// Wait for the delivery to arrive.
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if received.Load() > 0 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		if received.Load() == 0 {
			t.Error("expected at least one delivery to the test server")
		}
	})
}

// ---------------------------------------------------------------------------
// TestEmit
// ---------------------------------------------------------------------------

func TestEmit_NoWebhooks(t *testing.T) {
	store := &serviceStubStore{hooks: nil}
	svc := New(store, Config{}, discardLogger())

	// Should not panic and queue remains empty.
	svc.Emit(context.Background(), Event{Type: "job.completed", Payload: map[string]any{}})

	if len(svc.queue) != 0 {
		t.Errorf("queue len = %d, want 0", len(svc.queue))
	}
}

func TestEmit_OneWebhook(t *testing.T) {
	store := &serviceStubStore{
		hooks: []*db.Webhook{
			{ID: "wh-1", URL: "http://example.com", Provider: "generic"},
		},
	}
	svc := New(store, Config{WorkerCount: 1}, discardLogger())

	svc.Emit(context.Background(), Event{Type: "job.completed", Payload: map[string]any{}})

	if len(svc.queue) != 1 {
		t.Errorf("queue len = %d, want 1", len(svc.queue))
	}
}

func TestEmit_QueueFull_Drops(t *testing.T) {
	store := &serviceStubStore{
		hooks: []*db.Webhook{
			{ID: "wh-1", URL: "http://example.com", Provider: "generic"},
		},
	}
	// WorkerCount=1 → queue capacity = 10.
	svc := New(store, Config{WorkerCount: 1}, discardLogger())

	// Fill queue completely (10 items).
	for i := 0; i < 10; i++ {
		svc.Emit(context.Background(), Event{Type: "fill", Payload: map[string]any{}})
	}

	// This emit should be silently dropped.
	svc.Emit(context.Background(), Event{Type: "dropped", Payload: map[string]any{}})

	if len(svc.queue) != 10 {
		t.Errorf("queue len = %d, want 10 (drop on full)", len(svc.queue))
	}
}

func TestEmit_StoreError_DoesNotPanic(t *testing.T) {
	store := &serviceStubStore{hooksErr: db.ErrNotFound}
	svc := New(store, Config{}, discardLogger())

	// Should log warning and return, not panic.
	svc.Emit(context.Background(), Event{Type: "job.completed", Payload: map[string]any{}})
}

// ---------------------------------------------------------------------------
// TestWorker
// ---------------------------------------------------------------------------

func TestWorker_ExitsOnContextCancel(t *testing.T) {
	store := &senderStubStore{}
	svc := New(store, Config{WorkerCount: 1}, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	snd := &sender{store: store, cfg: svc.cfg, logger: svc.logger}

	done := make(chan struct{})
	go func() {
		svc.worker(ctx, snd)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// good
	case <-time.After(time.Second):
		t.Fatal("worker did not exit after context cancellation")
	}
}

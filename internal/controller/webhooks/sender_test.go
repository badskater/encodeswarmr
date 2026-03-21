package webhooks

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/badskater/distributed-encoder/internal/db"
	"github.com/badskater/distributed-encoder/internal/db/teststore"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ---------------------------------------------------------------------------
// Minimal store stub for sender tests
// ---------------------------------------------------------------------------

type senderStubStore struct {
	teststore.Stub
	deliveries []db.InsertWebhookDeliveryParams
}

func (s *senderStubStore) InsertWebhookDelivery(_ context.Context, p db.InsertWebhookDeliveryParams) error {
	s.deliveries = append(s.deliveries, p)
	return nil
}

// ---------------------------------------------------------------------------
// TestSenderSend_success — delivers on first attempt to a real HTTP server
// ---------------------------------------------------------------------------

func TestSenderSend_success(t *testing.T) {
	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store := &senderStubStore{}
	snd := &sender{
		store:  store,
		cfg:    Config{MaxRetries: 3, DeliveryTimeout: 5 * time.Second},
		logger: discardLogger(),
	}

	secret := "test-secret"
	wh := &db.Webhook{ID: "wh-1", URL: srv.URL, Provider: "generic", Secret: &secret}
	event := Event{Type: "job.completed", Payload: map[string]any{"job_id": "j-1"}}

	snd.send(context.Background(), wh, event)

	if received.Load() != 1 {
		t.Errorf("server received %d requests, want 1", received.Load())
	}
	if len(store.deliveries) != 1 {
		t.Fatalf("logged %d deliveries, want 1", len(store.deliveries))
	}
	d := store.deliveries[0]
	if !d.Success {
		t.Error("delivery logged as failed, want success")
	}
	if d.Attempt != 1 {
		t.Errorf("attempt = %d, want 1", d.Attempt)
	}
	if d.Event != "job.completed" {
		t.Errorf("event = %q, want job.completed", d.Event)
	}
}

// ---------------------------------------------------------------------------
// TestSenderSend_HMAC — X-Signature header is set when secret is configured
// ---------------------------------------------------------------------------

func TestSenderSend_HMAC(t *testing.T) {
	var sigHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sigHeader = r.Header.Get("X-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store := &senderStubStore{}
	snd := &sender{store: store, cfg: Config{MaxRetries: 1, DeliveryTimeout: 5 * time.Second}, logger: discardLogger()}

	secret := "hmac-key"
	wh := &db.Webhook{ID: "wh-2", URL: srv.URL, Provider: "generic", Secret: &secret}
	event := Event{Type: "agent.online", Payload: map[string]any{"agent_id": "a-1"}}

	snd.send(context.Background(), wh, event)

	if sigHeader == "" {
		t.Fatal("X-Signature header not set")
	}
	if len(sigHeader) < 7 || sigHeader[:7] != "sha256=" {
		t.Errorf("X-Signature format wrong: %q", sigHeader)
	}
}

// ---------------------------------------------------------------------------
// TestSenderSend_noSecret — no X-Signature when secret is nil
// ---------------------------------------------------------------------------

func TestSenderSend_noSecret(t *testing.T) {
	var sigHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sigHeader = r.Header.Get("X-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store := &senderStubStore{}
	snd := &sender{store: store, cfg: Config{MaxRetries: 1, DeliveryTimeout: 5 * time.Second}, logger: discardLogger()}

	wh := &db.Webhook{ID: "wh-3", URL: srv.URL, Provider: "generic", Secret: nil}
	event := Event{Type: "job.failed", Payload: map[string]any{"job_id": "j-2"}}

	snd.send(context.Background(), wh, event)

	if sigHeader != "" {
		t.Errorf("expected no X-Signature, got %q", sigHeader)
	}
}

// ---------------------------------------------------------------------------
// TestSenderSend_serverError — 500 response logs failure, does NOT retry in
// the test (retry delays are skipped by cancelling context)
// ---------------------------------------------------------------------------

func TestSenderSend_serverError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	store := &senderStubStore{}
	snd := &sender{store: store, cfg: Config{MaxRetries: 1, DeliveryTimeout: 5 * time.Second}, logger: discardLogger()}

	// Override retry delays to zero so the test doesn't sleep.
	orig := retryDelays
	retryDelays = []time.Duration{0, 0, 0}
	t.Cleanup(func() { retryDelays = orig })

	wh := &db.Webhook{ID: "wh-4", URL: srv.URL, Provider: "generic"}
	event := Event{Type: "job.failed", Payload: map[string]any{"job_id": "j-3"}}

	snd.send(context.Background(), wh, event)

	// MaxRetries=1 → 2 attempts total
	if len(store.deliveries) != 2 {
		t.Fatalf("logged %d deliveries, want 2", len(store.deliveries))
	}
	for _, d := range store.deliveries {
		if d.Success {
			t.Error("delivery logged as success, want failure")
		}
	}
}

// ---------------------------------------------------------------------------
// TestSenderSend_unreachableURL — network error is recorded as failure
// ---------------------------------------------------------------------------

func TestSenderSend_unreachableURL(t *testing.T) {
	// Zero out retry delays so the test completes quickly.
	orig := retryDelays
	retryDelays = []time.Duration{0, 0, 0}
	t.Cleanup(func() { retryDelays = orig })

	store := &senderStubStore{}
	// MaxRetries: 1 → 2 total attempts.
	snd := &sender{store: store, cfg: Config{MaxRetries: 1, DeliveryTimeout: 100 * time.Millisecond}, logger: discardLogger()}

	wh := &db.Webhook{ID: "wh-5", URL: "http://127.0.0.1:1", Provider: "generic"}
	event := Event{Type: "job.completed", Payload: map[string]any{}}

	snd.send(context.Background(), wh, event)

	if len(store.deliveries) != 2 {
		t.Fatalf("logged %d deliveries, want 2", len(store.deliveries))
	}
	for _, d := range store.deliveries {
		if d.Success {
			t.Error("expected failure delivery, got success")
		}
	}
}

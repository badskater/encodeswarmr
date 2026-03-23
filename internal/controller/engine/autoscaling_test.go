package engine

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/config"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newTestHook(cfg config.AutoScalingConfig) *AutoScalingHook {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewAutoScalingHook(func() config.AutoScalingConfig { return cfg }, logger)
}

// ---------------------------------------------------------------------------
// TestNewAutoScalingHook
// ---------------------------------------------------------------------------

func TestNewAutoScalingHook_NotNil(t *testing.T) {
	h := newTestHook(config.AutoScalingConfig{Enabled: true, WebhookURL: "http://example.com"})
	if h == nil {
		t.Fatal("NewAutoScalingHook returned nil")
	}
}

func TestNewAutoScalingHook_ClientInitialised(t *testing.T) {
	h := newTestHook(config.AutoScalingConfig{})
	if h.client == nil {
		t.Error("http.Client is nil")
	}
}

// ---------------------------------------------------------------------------
// TestCheck_Disabled
// ---------------------------------------------------------------------------

func TestCheck_Disabled_DoesNotFire(t *testing.T) {
	var called atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	h := newTestHook(config.AutoScalingConfig{
		Enabled:          false,
		WebhookURL:       ts.URL,
		ScaleUpThreshold: 0,
	})
	h.Check(context.Background(), 100, 0, 0)
	time.Sleep(50 * time.Millisecond)

	if called.Load() != 0 {
		t.Errorf("webhook called %d times, want 0 (hook disabled)", called.Load())
	}
}

func TestCheck_EmptyWebhookURL_DoesNotFire(t *testing.T) {
	var called atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	h := newTestHook(config.AutoScalingConfig{
		Enabled:    true,
		WebhookURL: "", // empty
	})
	h.Check(context.Background(), 100, 0, 0)
	time.Sleep(50 * time.Millisecond)

	if called.Load() != 0 {
		t.Errorf("webhook called %d times, want 0 (no url)", called.Load())
	}
}

// ---------------------------------------------------------------------------
// TestCheck_ScaleUp
// ---------------------------------------------------------------------------

func TestCheck_ScaleUp_FiresWhenPendingExceedsThresholdAndNoIdleAgents(t *testing.T) {
	type payload struct {
		Action       string `json:"action"`
		PendingTasks int64  `json:"pending_tasks"`
		ActiveAgents int    `json:"active_agents"`
	}

	received := make(chan payload, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p payload
		_ = json.NewDecoder(r.Body).Decode(&p)
		received <- p
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	h := newTestHook(config.AutoScalingConfig{
		Enabled:          true,
		WebhookURL:       ts.URL,
		ScaleUpThreshold: 5,
		CooldownSeconds:  0,
	})

	// pendingTasks=10 > threshold=5, idleAgents=0 → scale_up
	h.Check(context.Background(), 10, 3, 0)

	select {
	case p := <-received:
		if p.Action != "scale_up" {
			t.Errorf("action = %q, want scale_up", p.Action)
		}
		if p.PendingTasks != 10 {
			t.Errorf("pending_tasks = %d, want 10", p.PendingTasks)
		}
		if p.ActiveAgents != 3 {
			t.Errorf("active_agents = %d, want 3", p.ActiveAgents)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for scale_up webhook")
	}
}

func TestCheck_ScaleUp_NotFiredWhenBelowThreshold(t *testing.T) {
	var called atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	h := newTestHook(config.AutoScalingConfig{
		Enabled:          true,
		WebhookURL:       ts.URL,
		ScaleUpThreshold: 10,
		CooldownSeconds:  0,
	})

	// pendingTasks=3 <= threshold=10, should NOT fire
	h.Check(context.Background(), 3, 0, 0)
	time.Sleep(50 * time.Millisecond)

	if called.Load() != 0 {
		t.Errorf("webhook called %d times, want 0 (below threshold)", called.Load())
	}
}

func TestCheck_ScaleUp_NotFiredWhenIdleAgentsExist(t *testing.T) {
	var called atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	h := newTestHook(config.AutoScalingConfig{
		Enabled:            true,
		WebhookURL:         ts.URL,
		ScaleUpThreshold:   5,
		ScaleDownThreshold: 10, // high threshold so scale_down also doesn't fire
		CooldownSeconds:    0,
	})

	// pending=10 > threshold=5 but idleAgents=2 → scale_up should NOT fire.
	// idle=2 < ScaleDownThreshold=10 → scale_down should NOT fire either.
	h.Check(context.Background(), 10, 3, 2)
	time.Sleep(100 * time.Millisecond)

	if called.Load() != 0 {
		t.Errorf("webhook called %d times, want 0 (idle agents present, scale-down threshold not met)", called.Load())
	}
}

// ---------------------------------------------------------------------------
// TestCheck_ScaleDown
// ---------------------------------------------------------------------------

func TestCheck_ScaleDown_FiresWhenIdleExceedsThreshold(t *testing.T) {
	type payload struct {
		Action     string `json:"action"`
		IdleAgents int    `json:"idle_agents"`
	}

	received := make(chan payload, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p payload
		_ = json.NewDecoder(r.Body).Decode(&p)
		received <- p
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	h := newTestHook(config.AutoScalingConfig{
		Enabled:            true,
		WebhookURL:         ts.URL,
		ScaleUpThreshold:   100,
		ScaleDownThreshold: 3,
		CooldownSeconds:    0,
	})

	// idleAgents=5 > threshold=3, pendingTasks=0 → scale_down
	h.Check(context.Background(), 0, 5, 5)

	select {
	case p := <-received:
		if p.Action != "scale_down" {
			t.Errorf("action = %q, want scale_down", p.Action)
		}
		if p.IdleAgents != 5 {
			t.Errorf("idle_agents = %d, want 5", p.IdleAgents)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for scale_down webhook")
	}
}

func TestCheck_ScaleDown_NotFiredWhenBelowThreshold(t *testing.T) {
	var called atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	h := newTestHook(config.AutoScalingConfig{
		Enabled:            true,
		WebhookURL:         ts.URL,
		ScaleDownThreshold: 5,
		CooldownSeconds:    0,
	})

	// idle=3 <= threshold=5, should NOT fire
	h.Check(context.Background(), 0, 0, 3)
	time.Sleep(50 * time.Millisecond)

	if called.Load() != 0 {
		t.Errorf("webhook called %d times, want 0 (below scale-down threshold)", called.Load())
	}
}

// ---------------------------------------------------------------------------
// TestCheck_Cooldown
// ---------------------------------------------------------------------------

func TestCheck_Cooldown_PreventsRapidRepeatFire(t *testing.T) {
	var callCount atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	h := newTestHook(config.AutoScalingConfig{
		Enabled:          true,
		WebhookURL:       ts.URL,
		ScaleUpThreshold: 0,
		CooldownSeconds:  60, // very long cooldown
	})

	ctx := context.Background()
	// Both calls should qualify for scale_up, but only the first should fire.
	h.Check(ctx, 10, 0, 0)
	h.Check(ctx, 10, 0, 0)
	time.Sleep(100 * time.Millisecond)

	if callCount.Load() > 1 {
		t.Errorf("webhook called %d times, want 1 (cooldown should suppress second call)", callCount.Load())
	}
}

func TestCheck_Cooldown_AllowsFireAfterCooldownExpires(t *testing.T) {
	var callCount atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	h := newTestHook(config.AutoScalingConfig{
		Enabled:          true,
		WebhookURL:       ts.URL,
		ScaleUpThreshold: 0,
		CooldownSeconds:  0, // zero cooldown — always allowed
	})

	ctx := context.Background()
	h.Check(ctx, 10, 0, 0)
	h.Check(ctx, 10, 0, 0)
	time.Sleep(200 * time.Millisecond)

	if callCount.Load() < 2 {
		t.Errorf("webhook called %d times, want >= 2 (no cooldown)", callCount.Load())
	}
}

// ---------------------------------------------------------------------------
// TestWebhook
// ---------------------------------------------------------------------------

func TestTestWebhook_SendsCorrectPayload(t *testing.T) {
	type payload struct {
		Action string `json:"action"`
	}

	received := make(chan payload, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		ua := r.Header.Get("User-Agent")
		if !containsStr(ua, "EncodeSwarmr") {
			t.Errorf("User-Agent = %q, want EncodeSwarmr prefix", ua)
		}
		var p payload
		_ = json.NewDecoder(r.Body).Decode(&p)
		received <- p
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	h := newTestHook(config.AutoScalingConfig{})
	if err := h.TestWebhook(context.Background(), ts.URL); err != nil {
		t.Fatalf("TestWebhook: %v", err)
	}

	select {
	case p := <-received:
		if p.Action != "test" {
			t.Errorf("action = %q, want test", p.Action)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for test webhook")
	}
}

func TestTestWebhook_Non2xx_ReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	h := newTestHook(config.AutoScalingConfig{})
	if err := h.TestWebhook(context.Background(), ts.URL); err == nil {
		t.Error("expected error for non-2xx response")
	}
}

func TestTestWebhook_UnreachableURL_ReturnsError(t *testing.T) {
	h := newTestHook(config.AutoScalingConfig{})
	// Port 1 is almost certainly not listening.
	if err := h.TestWebhook(context.Background(), "http://127.0.0.1:1/webhook"); err == nil {
		t.Error("expected error for unreachable URL")
	}
}

// containsStr reports whether s contains sub.
func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}

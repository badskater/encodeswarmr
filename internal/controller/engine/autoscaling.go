package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/config"
)

// AutoScalingHook fires outbound webhook calls when queue depth or idle agent
// count crosses the configured thresholds.  It is safe for concurrent use.
type AutoScalingHook struct {
	cfgFn  func() config.AutoScalingConfig // reads live config (may change at runtime)
	logger *slog.Logger
	client *http.Client

	mu              sync.Mutex
	lastScaleUp     time.Time
	lastScaleDown   time.Time
	idleConsecutive int // ticks the idle threshold has been exceeded
}

// NewAutoScalingHook creates a hook that reads auto-scaling config via cfgFn
// on each invocation so runtime updates to the config take effect immediately.
func NewAutoScalingHook(cfgFn func() config.AutoScalingConfig, logger *slog.Logger) *AutoScalingHook {
	return &AutoScalingHook{
		cfgFn:  cfgFn,
		logger: logger,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// scalePayload is the JSON body sent to the auto-scaling webhook URL.
type scalePayload struct {
	Action       string `json:"action"`
	PendingTasks int64  `json:"pending_tasks,omitempty"`
	ActiveAgents int    `json:"active_agents,omitempty"`
	IdleAgents   int    `json:"idle_agents,omitempty"`
}

// Check evaluates current queue and agent state and fires the auto-scaling
// webhook if thresholds are exceeded.  Intended to be called from the engine
// dispatch loop on each tick.
func (h *AutoScalingHook) Check(ctx context.Context, pendingTasks int64, activeAgents, idleAgents int) {
	cfg := h.cfgFn()
	if !cfg.Enabled || cfg.WebhookURL == "" {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	cooldown := time.Duration(cfg.CooldownSeconds) * time.Second

	// Scale-up: pending tasks exceed threshold and no agents are idle.
	if pendingTasks > int64(cfg.ScaleUpThreshold) && idleAgents == 0 {
		if now.Sub(h.lastScaleUp) >= cooldown {
			h.lastScaleUp = now
			go h.fire(ctx, cfg.WebhookURL, scalePayload{
				Action:       "scale_up",
				PendingTasks: pendingTasks,
				ActiveAgents: activeAgents,
			})
		}
		return
	}

	// Scale-down: idle agents exceed threshold.
	if idleAgents > cfg.ScaleDownThreshold {
		if now.Sub(h.lastScaleDown) >= cooldown {
			h.lastScaleDown = now
			go h.fire(ctx, cfg.WebhookURL, scalePayload{
				Action:     "scale_down",
				IdleAgents: idleAgents,
			})
		}
	}
}

// fire sends a single POST to the webhook URL with the given payload.
func (h *AutoScalingHook) fire(ctx context.Context, url string, payload scalePayload) {
	body, err := json.Marshal(payload)
	if err != nil {
		h.logger.Warn("auto-scaling: marshal payload", slog.String("error", err.Error()))
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		h.logger.Warn("auto-scaling: build request", slog.String("error", err.Error()))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "EncodeSwarmr-AutoScaling/1.0")

	resp, err := h.client.Do(req)
	if err != nil {
		h.logger.Warn("auto-scaling: webhook call failed",
			slog.String("action", payload.Action),
			slog.String("error", err.Error()),
		)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		h.logger.Warn("auto-scaling: webhook non-2xx",
			slog.String("action", payload.Action),
			slog.Int("status", resp.StatusCode),
		)
		return
	}
	h.logger.Info("auto-scaling: webhook fired",
		slog.String("action", payload.Action),
		slog.Int64("pending_tasks", payload.PendingTasks),
		slog.Int("active_agents", payload.ActiveAgents),
		slog.Int("idle_agents", payload.IdleAgents),
		slog.Int("status", resp.StatusCode),
	)
}

// TestWebhook sends a test payload to the configured webhook URL.
// Returns an error if the call fails or returns a non-2xx status.
func (h *AutoScalingHook) TestWebhook(ctx context.Context, url string) error {
	payload := scalePayload{Action: "test"}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("auto-scaling test: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "EncodeSwarmr-AutoScaling/1.0")

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("auto-scaling test: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("auto-scaling test: non-2xx status %d", resp.StatusCode)
	}
	return nil
}

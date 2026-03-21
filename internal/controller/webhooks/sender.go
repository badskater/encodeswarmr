package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/badskater/encodeswarmr/internal/db"
)

// retryDelays are the wait durations between delivery attempts (10s, 30s, 90s).
var retryDelays = []time.Duration{10 * time.Second, 30 * time.Second, 90 * time.Second}

// sender performs HTTP delivery of webhook events with retries.
type sender struct {
	store  db.Store
	cfg    Config
	logger *slog.Logger
}

// send formats the payload for the webhook's provider, then attempts delivery
// with up to MaxRetries retries. Each attempt is logged to webhook_deliveries.
func (s *sender) send(ctx context.Context, wh *db.Webhook, event Event) {
	payload, err := formatPayload(wh.Provider, event)
	if err != nil {
		s.logger.Warn("webhooks: format payload",
			slog.String("webhook_id", wh.ID),
			slog.String("event", event.Type),
			slog.String("error", err.Error()),
		)
		return
	}

	maxAttempts := s.cfg.MaxRetries + 1
	if maxAttempts <= 1 {
		maxAttempts = 4
	}

	timeout := s.cfg.DeliveryTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	client := &http.Client{Timeout: timeout}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			idx := attempt - 2
			if idx >= len(retryDelays) {
				idx = len(retryDelays) - 1
			}
			delay := retryDelays[idx]
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}

		code, errMsg := s.attempt(ctx, client, wh, payload)
		success := errMsg == "" && code >= 200 && code < 300

		p := db.InsertWebhookDeliveryParams{
			WebhookID: wh.ID,
			Event:     event.Type,
			Payload:   payload,
			Success:   success,
			Attempt:   attempt,
		}
		if code > 0 {
			p.ResponseCode = &code
		}
		if !success && errMsg != "" {
			p.ErrorMsg = &errMsg
		}

		if logErr := s.store.InsertWebhookDelivery(ctx, p); logErr != nil {
			s.logger.Warn("webhooks: log delivery",
				slog.String("webhook_id", wh.ID),
				slog.String("error", logErr.Error()),
			)
		}

		if success {
			s.logger.Info("webhook delivered",
				slog.String("webhook_id", wh.ID),
				slog.String("event", event.Type),
				slog.Int("attempt", attempt),
				slog.Int("status_code", code),
			)
			return
		}

		s.logger.Warn("webhook delivery failed",
			slog.String("webhook_id", wh.ID),
			slog.String("event", event.Type),
			slog.Int("attempt", attempt),
			slog.String("error", errMsg),
		)
	}
}

// attempt performs a single HTTP POST and returns the status code and error message.
func (s *sender) attempt(ctx context.Context, client *http.Client, wh *db.Webhook, payload []byte) (int, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wh.URL,
		bytes.NewReader(payload))
	if err != nil {
		return 0, fmt.Sprintf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "DistEncoder-Webhooks/1.0")

	// Sign the payload with HMAC-SHA256 if a secret is configured.
	if wh.Secret != nil && *wh.Secret != "" {
		sig := computeHMAC(payload, *wh.Secret)
		req.Header.Set("X-Signature", "sha256="+sig)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer resp.Body.Close()
	return resp.StatusCode, ""
}

// computeHMAC returns a hex-encoded HMAC-SHA256 of payload using key.
func computeHMAC(payload []byte, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// Package notifications provides delivery adapters for push notification channels.
// Each sender is constructed from its configuration block; when the configuration
// is disabled or missing required fields the constructor returns nil and all
// methods no-op safely.
package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// httpClient is a shared client with a reasonable timeout used by all channel senders.
var httpClient = &http.Client{Timeout: 10 * time.Second}

// ---------------------------------------------------------------------------
// Channel config types — imported by config package via the notifications package.
// These are duplicated here so the notifications package stays self-contained;
// the config package defines parallel types with mapstructure tags.
// ---------------------------------------------------------------------------

// TelegramChannelConfig holds credentials for the Telegram Bot API.
type TelegramChannelConfig struct {
	Enabled  bool
	BotToken string
	ChatID   string
}

// PushoverChannelConfig holds credentials for the Pushover API.
type PushoverChannelConfig struct {
	Enabled  bool
	AppToken string
	UserKey  string
	Priority int // -2..2; 0 = normal
}

// NtfyChannelConfig holds settings for an ntfy push notification server.
type NtfyChannelConfig struct {
	Enabled   bool
	ServerURL string
	Topic     string
}

// ---------------------------------------------------------------------------
// Telegram
// ---------------------------------------------------------------------------

// TelegramSender delivers messages via the Telegram Bot API.
type TelegramSender struct {
	cfg    TelegramChannelConfig
	logger *slog.Logger
}

// NewTelegramSender returns a TelegramSender, or nil when Telegram is disabled
// or required fields are missing.
func NewTelegramSender(cfg TelegramChannelConfig, logger *slog.Logger) *TelegramSender {
	if !cfg.Enabled || cfg.BotToken == "" || cfg.ChatID == "" {
		return nil
	}
	return &TelegramSender{cfg: cfg, logger: logger}
}

// Send posts a message to the configured Telegram chat.
// text may contain Telegram Markdown formatting.
func (t *TelegramSender) Send(text string) error {
	if t == nil {
		return nil
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.cfg.BotToken)
	payload, err := json.Marshal(map[string]string{
		"chat_id":    t.cfg.ChatID,
		"text":       text,
		"parse_mode": "Markdown",
	})
	if err != nil {
		return fmt.Errorf("telegram: marshal payload: %w", err)
	}

	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("telegram: post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		t.logger.Warn("telegram: unexpected status",
			slog.Int("status", resp.StatusCode),
			slog.String("body", string(body)),
		)
		return fmt.Errorf("telegram: unexpected status %d", resp.StatusCode)
	}
	t.logger.Info("telegram: message sent")
	return nil
}

// ---------------------------------------------------------------------------
// Pushover
// ---------------------------------------------------------------------------

// PushoverSender delivers messages via the Pushover API.
type PushoverSender struct {
	cfg    PushoverChannelConfig
	logger *slog.Logger
}

// NewPushoverSender returns a PushoverSender, or nil when Pushover is disabled
// or required fields are missing.
func NewPushoverSender(cfg PushoverChannelConfig, logger *slog.Logger) *PushoverSender {
	if !cfg.Enabled || cfg.AppToken == "" || cfg.UserKey == "" {
		return nil
	}
	return &PushoverSender{cfg: cfg, logger: logger}
}

// Send posts a notification to Pushover.
func (p *PushoverSender) Send(message string) error {
	if p == nil {
		return nil
	}
	payload, err := json.Marshal(map[string]any{
		"token":    p.cfg.AppToken,
		"user":     p.cfg.UserKey,
		"message":  message,
		"priority": p.cfg.Priority,
	})
	if err != nil {
		return fmt.Errorf("pushover: marshal payload: %w", err)
	}

	resp, err := httpClient.Post("https://api.pushover.net/1/messages.json", "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("pushover: post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		p.logger.Warn("pushover: unexpected status",
			slog.Int("status", resp.StatusCode),
			slog.String("body", string(body)),
		)
		return fmt.Errorf("pushover: unexpected status %d", resp.StatusCode)
	}
	p.logger.Info("pushover: message sent")
	return nil
}

// ---------------------------------------------------------------------------
// ntfy
// ---------------------------------------------------------------------------

// NtfySender delivers messages via the ntfy push notification service.
type NtfySender struct {
	cfg    NtfyChannelConfig
	logger *slog.Logger
}

// NewNtfySender returns an NtfySender, or nil when ntfy is disabled or
// required fields are missing.
func NewNtfySender(cfg NtfyChannelConfig, logger *slog.Logger) *NtfySender {
	if !cfg.Enabled || cfg.Topic == "" {
		return nil
	}
	serverURL := cfg.ServerURL
	if serverURL == "" {
		serverURL = "https://ntfy.sh"
	}
	return &NtfySender{
		cfg: NtfyChannelConfig{
			Enabled:   cfg.Enabled,
			ServerURL: serverURL,
			Topic:     cfg.Topic,
		},
		logger: logger,
	}
}

// Send posts a message to the configured ntfy topic.
// title is sent as the ntfy Title header; message is the body.
func (n *NtfySender) Send(title, message string) error {
	if n == nil {
		return nil
	}
	url := fmt.Sprintf("%s/%s", strings.TrimRight(n.cfg.ServerURL, "/"), n.cfg.Topic)
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(message))
	if err != nil {
		return fmt.Errorf("ntfy: build request: %w", err)
	}
	req.Header.Set("Title", title)
	req.Header.Set("Content-Type", "text/plain")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ntfy: post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		n.logger.Warn("ntfy: unexpected status",
			slog.Int("status", resp.StatusCode),
			slog.String("body", string(body)),
		)
		return fmt.Errorf("ntfy: unexpected status %d", resp.StatusCode)
	}
	n.logger.Info("ntfy: message sent", slog.String("topic", n.cfg.Topic))
	return nil
}

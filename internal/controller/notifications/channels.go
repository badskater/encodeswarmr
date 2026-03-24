// Package notifications provides notification channel senders for Telegram,
// Pushover, and ntfy.io.
package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/config"
)

// ---------------------------------------------------------------------------
// TelegramSender
// ---------------------------------------------------------------------------

// TelegramSender delivers messages to a Telegram chat via the Bot API.
type TelegramSender struct {
	cfg    config.TelegramConfig
	client *http.Client
	logger *slog.Logger
}

// NewTelegramSender creates a TelegramSender.
// Returns nil when the bot token or chat ID is empty (Telegram disabled).
func NewTelegramSender(cfg config.TelegramConfig, logger *slog.Logger) *TelegramSender {
	if cfg.BotToken == "" || cfg.ChatID == "" {
		return nil
	}
	return &TelegramSender{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
		logger: logger,
	}
}

// Send delivers a plain-text message to the configured Telegram chat.
func (t *TelegramSender) Send(text string) error {
	if t == nil {
		return nil
	}
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.cfg.BotToken)
	payload := map[string]string{
		"chat_id":    t.cfg.ChatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram: marshal payload: %w", err)
	}
	resp, err := t.client.Post(apiURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram: http post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram: unexpected status %d", resp.StatusCode)
	}
	t.logger.Info("telegram: message sent", slog.String("chat_id", t.cfg.ChatID))
	return nil
}

// ---------------------------------------------------------------------------
// PushoverSender
// ---------------------------------------------------------------------------

// PushoverSender delivers messages via the Pushover API.
type PushoverSender struct {
	cfg    config.PushoverConfig
	client *http.Client
	logger *slog.Logger
}

// NewPushoverSender creates a PushoverSender.
// Returns nil when the app token or user key is empty (Pushover disabled).
func NewPushoverSender(cfg config.PushoverConfig, logger *slog.Logger) *PushoverSender {
	if cfg.AppToken == "" || cfg.UserKey == "" {
		return nil
	}
	return &PushoverSender{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
		logger: logger,
	}
}

// Send delivers a message via Pushover.
func (p *PushoverSender) Send(title, message string) error {
	if p == nil {
		return nil
	}
	form := url.Values{}
	form.Set("token", p.cfg.AppToken)
	form.Set("user", p.cfg.UserKey)
	form.Set("title", title)
	form.Set("message", message)

	resp, err := p.client.PostForm("https://api.pushover.net/1/messages.json", form)
	if err != nil {
		return fmt.Errorf("pushover: http post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pushover: unexpected status %d", resp.StatusCode)
	}
	p.logger.Info("pushover: message sent", slog.String("user_key", p.cfg.UserKey))
	return nil
}

// ---------------------------------------------------------------------------
// NtfySender
// ---------------------------------------------------------------------------

// NtfySender delivers messages via ntfy.sh or a self-hosted ntfy server.
type NtfySender struct {
	cfg    config.NtfyConfig
	client *http.Client
	logger *slog.Logger
}

// NewNtfySender creates a NtfySender.
// Returns nil when the topic is empty (ntfy disabled).
func NewNtfySender(cfg config.NtfyConfig, logger *slog.Logger) *NtfySender {
	if cfg.Topic == "" {
		return nil
	}
	return &NtfySender{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
		logger: logger,
	}
}

// Send publishes a message to the configured ntfy topic.
func (n *NtfySender) Send(title, message string) error {
	if n == nil {
		return nil
	}
	serverURL := n.cfg.ServerURL
	if serverURL == "" {
		serverURL = "https://ntfy.sh"
	}
	serverURL = strings.TrimRight(serverURL, "/")
	endpoint := fmt.Sprintf("%s/%s", serverURL, n.cfg.Topic)

	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(message))
	if err != nil {
		return fmt.Errorf("ntfy: build request: %w", err)
	}
	req.Header.Set("Title", title)
	req.Header.Set("Content-Type", "text/plain")
	if n.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+n.cfg.Token)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("ntfy: http post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy: unexpected status %d", resp.StatusCode)
	}
	n.logger.Info("ntfy: message sent", slog.String("topic", n.cfg.Topic))
	return nil
}

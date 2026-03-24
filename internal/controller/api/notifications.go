package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/badskater/encodeswarmr/internal/controller/auth"
	"github.com/badskater/encodeswarmr/internal/controller/notifications"
	"github.com/badskater/encodeswarmr/internal/db"
)

const testNotificationMessage = "Test notification from EncodeSwarmr — if you received this, your configuration is correct."

// defaultNotificationPrefs returns the default notification preferences used
// when a user has not yet saved any explicit preferences.
func defaultNotificationPrefs(userID string) *db.NotificationPrefs {
	return &db.NotificationPrefs{
		UserID:              userID,
		NotifyOnJobComplete: true,
		NotifyOnJobFailed:   true,
		NotifyOnAgentStale:  false,
		NotifyEmail:         false,
		EmailAddress:        "",
	}
}

// handleGetNotificationPrefs returns the notification preferences for the
// authenticated user.
// GET /api/v1/me/notifications
func (s *Server) handleGetNotificationPrefs(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeProblem(w, r, http.StatusUnauthorized, "Unauthorized", "no active session")
		return
	}

	prefs, err := s.store.GetNotificationPrefs(r.Context(), claims.UserID)
	if errors.Is(err, db.ErrNotFound) {
		// No preferences stored yet — return defaults without persisting.
		writeJSON(w, r, http.StatusOK, defaultNotificationPrefs(claims.UserID))
		return
	}
	if err != nil {
		s.logger.Error("get notification prefs", "err", err, "user_id", claims.UserID)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, prefs)
}

// handleUpdateNotificationPrefs creates or updates the notification preferences
// for the authenticated user.
// PUT /api/v1/me/notifications
func (s *Server) handleUpdateNotificationPrefs(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeProblem(w, r, http.StatusUnauthorized, "Unauthorized", "no active session")
		return
	}

	var req struct {
		NotifyOnJobComplete   bool   `json:"notify_on_job_complete"`
		NotifyOnJobFailed     bool   `json:"notify_on_job_failed"`
		NotifyOnAgentStale    bool   `json:"notify_on_agent_stale"`
		WebhookFilterUserOnly bool   `json:"webhook_filter_user_only"`
		EmailAddress          string `json:"email_address"`
		NotifyEmail           bool   `json:"notify_email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}

	if err := s.store.UpsertNotificationPrefs(r.Context(), db.UpsertNotificationPrefsParams{
		UserID:                claims.UserID,
		NotifyOnJobComplete:   req.NotifyOnJobComplete,
		NotifyOnJobFailed:     req.NotifyOnJobFailed,
		NotifyOnAgentStale:    req.NotifyOnAgentStale,
		WebhookFilterUserOnly: req.WebhookFilterUserOnly,
		EmailAddress:          req.EmailAddress,
		NotifyEmail:           req.NotifyEmail,
	}); err != nil {
		s.logger.Error("upsert notification prefs", "err", err, "user_id", claims.UserID)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Return the saved preferences.
	prefs, err := s.store.GetNotificationPrefs(r.Context(), claims.UserID)
	if err != nil {
		s.logger.Error("get notification prefs after upsert", "err", err, "user_id", claims.UserID)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, prefs)
}

// handleTestEmail sends a test email to verify SMTP configuration.
// POST /api/v1/notifications/test-email  (admin only)
func (s *Server) handleTestEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To string `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.To == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "to address is required")
		return
	}

	if s.email == nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "SMTP Not Configured",
			"no SMTP host configured; set smtp.host in the controller config")
		return
	}

	body, err := notifications.RenderJobCompleted("test-job-id", "/path/to/test-source.mkv")
	if err != nil {
		s.logger.Error("test-email: render template", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	if err := s.email.Send(req.To, "Test Email — EncodeSwarmr", body); err != nil {
		s.logger.Warn("test-email: send failed", "to", req.To, "err", err)
		writeProblem(w, r, http.StatusBadGateway, "Email Delivery Failed", err.Error())
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true, "to": req.To})
}

// handleTestTelegram sends a test message to verify Telegram configuration.
// POST /api/v1/notifications/test-telegram  (admin only)
func (s *Server) handleTestTelegram(w http.ResponseWriter, r *http.Request) {
	if s.telegram == nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "Telegram Not Configured",
			"no Telegram bot token or chat ID configured; set notifications.telegram in the controller config")
		return
	}
	if err := s.telegram.Send("<b>EncodeSwarmr</b> — " + testNotificationMessage); err != nil {
		s.logger.Warn("test-telegram: send failed", "err", err)
		writeProblem(w, r, http.StatusBadGateway, "Telegram Delivery Failed", err.Error())
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true})
}

// handleTestPushover sends a test message to verify Pushover configuration.
// POST /api/v1/notifications/test-pushover  (admin only)
func (s *Server) handleTestPushover(w http.ResponseWriter, r *http.Request) {
	if s.pushover == nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "Pushover Not Configured",
			"no Pushover app token or user key configured; set notifications.pushover in the controller config")
		return
	}
	if err := s.pushover.Send("EncodeSwarmr Test", testNotificationMessage); err != nil {
		s.logger.Warn("test-pushover: send failed", "err", err)
		writeProblem(w, r, http.StatusBadGateway, "Pushover Delivery Failed", err.Error())
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true})
}

// handleTestNtfy sends a test message to verify ntfy configuration.
// POST /api/v1/notifications/test-ntfy  (admin only)
func (s *Server) handleTestNtfy(w http.ResponseWriter, r *http.Request) {
	if s.ntfy == nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "ntfy Not Configured",
			"no ntfy topic configured; set notifications.ntfy in the controller config")
		return
	}
	if err := s.ntfy.Send("EncodeSwarmr Test", testNotificationMessage); err != nil {
		s.logger.Warn("test-ntfy: send failed", "err", err)
		writeProblem(w, r, http.StatusBadGateway, "ntfy Delivery Failed", err.Error())
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true})
}

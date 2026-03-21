package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/badskater/distributed-encoder/internal/controller/auth"
	"github.com/badskater/distributed-encoder/internal/db"
)

// defaultNotificationPrefs returns the default notification preferences used
// when a user has not yet saved any explicit preferences.
func defaultNotificationPrefs(userID string) *db.NotificationPrefs {
	return &db.NotificationPrefs{
		UserID:              userID,
		NotifyOnJobComplete: true,
		NotifyOnJobFailed:   true,
		NotifyOnAgentStale:  false,
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
		NotifyOnJobComplete   bool `json:"notify_on_job_complete"`
		NotifyOnJobFailed     bool `json:"notify_on_job_failed"`
		NotifyOnAgentStale    bool `json:"notify_on_agent_stale"`
		WebhookFilterUserOnly bool `json:"webhook_filter_user_only"`
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

package api

import (
	"errors"
	"net/http"

	"github.com/badskater/encodeswarmr/internal/controller/auth"
	"github.com/badskater/encodeswarmr/internal/db"
)

// handleListSessions returns all active sessions for the authenticated user.
// GET /api/v1/sessions
func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeProblem(w, r, http.StatusUnauthorized, "Unauthorized", "")
		return
	}

	sessions, err := s.store.ListSessionsByUser(r.Context(), claims.UserID)
	if err != nil {
		s.logger.Error("list sessions", "err", err, "user_id", claims.UserID)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	if sessions == nil {
		sessions = []*db.Session{}
	}

	// Return sessions without the token field (it is json:"-").
	writeJSON(w, r, http.StatusOK, sessions)
}

// handleDeleteSession revokes a specific session by its token/ID.
// DELETE /api/v1/sessions/{id}
func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeProblem(w, r, http.StatusUnauthorized, "Unauthorized", "")
		return
	}

	sessionID := r.PathValue("id")
	if sessionID == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "missing session id")
		return
	}

	// Ensure the session belongs to the requesting user before deleting.
	sessions, err := s.store.ListSessionsByUser(r.Context(), claims.UserID)
	if err != nil {
		s.logger.Error("list sessions for delete", "err", err, "user_id", claims.UserID)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	owned := false
	for _, sess := range sessions {
		if sess.Token == sessionID {
			owned = true
			break
		}
	}
	if !owned {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "session not found")
		return
	}

	if err := s.store.DeleteSessionByID(r.Context(), sessionID); errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "session not found")
		return
	} else if err != nil {
		s.logger.Error("delete session", "err", err, "session_id", sessionID)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true})
}

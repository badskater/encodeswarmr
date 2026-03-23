package api

import (
	"errors"
	"net/http"

	"github.com/badskater/encodeswarmr/internal/db"
)

// handleListSessions returns all active (non-expired) sessions.
// GET /api/v1/sessions
func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.store.ListActiveSessions(r.Context())
	if err != nil {
		s.logger.Error("list active sessions", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	if sessions == nil {
		sessions = []*db.ActiveSession{}
	}
	writeJSON(w, r, http.StatusOK, sessions)
}

// handleTerminateSession deletes a specific active session by its display ID.
// DELETE /api/v1/sessions/{id}
func (s *Server) handleTerminateSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "missing session id")
		return
	}

	if err := s.store.DeleteSessionByID(r.Context(), id); errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "session not found")
		return
	} else if err != nil {
		s.logger.Error("terminate session", "err", err, "session_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true})
}

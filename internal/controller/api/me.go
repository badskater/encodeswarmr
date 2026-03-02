package api

import (
	"errors"
	"net/http"

	"github.com/badskater/distributed-encoder/internal/controller/auth"
	"github.com/badskater/distributed-encoder/internal/db"
)

// handleGetMe returns the currently authenticated user's profile.
func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeProblem(w, r, http.StatusUnauthorized, "Unauthorized", "no active session")
		return
	}
	user, err := s.store.GetUserByID(r.Context(), claims.UserID)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "user not found")
		return
	}
	if err != nil {
		s.logger.Error("get me", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, toUserResponse(user))
}

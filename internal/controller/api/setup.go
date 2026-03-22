package api

import (
	"encoding/json"
	"net/http"
	"sync/atomic"

	"github.com/badskater/encodeswarmr/internal/controller/auth"
	"github.com/badskater/encodeswarmr/internal/db"
)

// setupDone is set to true after setup completes within this process lifetime.
// The DB is always the authoritative source; this is a performance optimisation
// that avoids a COUNT query on every page load once setup is complete.
var setupDone atomic.Bool

// handleSetupStatus reports whether initial setup is still required.
//
// GET /setup/status — always unauthenticated.
func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	if setupDone.Load() {
		writeJSON(w, r, http.StatusOK, map[string]bool{"required": false})
		return
	}
	n, err := s.store.CountAdminUsers(r.Context())
	if err != nil {
		s.logger.Error("setup status: count admin users", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]bool{"required": n == 0})
}

// handleSetup creates the first admin user.
// It is only callable when no admin users exist; subsequent calls return 409.
//
// POST /setup — unauthenticated.
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	// Always check the DB — the atomic flag is not sufficient on its own.
	n, err := s.store.CountAdminUsers(r.Context())
	if err != nil {
		s.logger.Error("setup: count admin users", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	if n > 0 {
		writeProblem(w, r, http.StatusConflict, "Conflict", "setup already completed")
		return
	}

	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Username == "" || req.Email == "" || req.Password == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error",
			"username, email, and password are required")
		return
	}
	if len(req.Password) < 8 {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error",
			"password must be at least 8 characters")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		s.logger.Error("setup: hash password", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	user, err := s.store.CreateUser(r.Context(), db.CreateUserParams{
		Username:     req.Username,
		Email:        req.Email,
		Role:         "admin",
		PasswordHash: &hash,
	})
	if err != nil {
		s.logger.Error("setup: create admin user", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	setupDone.Store(true)
	s.logger.Info("setup completed", "admin_user", user.Username)
	writeJSON(w, r, http.StatusCreated, toUserResponse(user))
}

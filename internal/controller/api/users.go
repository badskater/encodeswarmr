package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/auth"
	"github.com/badskater/encodeswarmr/internal/db"
)

// userResponse is the public representation of a user (no password hash).
type userResponse struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func toUserResponse(u *db.User) userResponse {
	return userResponse{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		Role:      u.Role,
		CreatedAt: u.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: u.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		s.logger.Error("list users", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	out := make([]userResponse, len(users))
	for i, u := range users {
		out[i] = toUserResponse(u)
	}
	writeJSON(w, r, http.StatusOK, out)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Role     string `json:"role"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Username == "" || req.Email == "" || req.Role == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "username, email, and role are required")
		return
	}
	if !isValidRole(req.Role) {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "role must be one of: viewer, operator, admin")
		return
	}

	params := db.CreateUserParams{
		Username: req.Username,
		Email:    req.Email,
		Role:     req.Role,
	}
	if req.Password != "" {
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			s.logger.Error("hash password", "err", err)
			writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
			return
		}
		params.PasswordHash = &hash
	}

	user, err := s.store.CreateUser(r.Context(), params)
	if err != nil {
		s.logger.Error("create user", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Emit audit entry — best-effort, never fail the request.
	auditParams := db.CreateAuditEntryParams{
		Action:     "user.create",
		Resource:   "user",
		ResourceID: user.ID,
		IPAddress:  r.RemoteAddr,
	}
	if claims, ok := auth.FromContext(r.Context()); ok {
		auditParams.UserID = &claims.UserID
		auditParams.Username = claims.Username
	}
	if err := s.store.CreateAuditEntry(r.Context(), auditParams); err != nil {
		s.logger.Warn("audit log: create user", "err", err, "user_id", user.ID)
	}

	writeJSON(w, r, http.StatusCreated, toUserResponse(user))
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteUser(r.Context(), id); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "user not found")
			return
		}
		s.logger.Error("delete user", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Emit audit entry — best-effort, never fail the request.
	auditParams := db.CreateAuditEntryParams{
		Action:     "user.delete",
		Resource:   "user",
		ResourceID: id,
		IPAddress:  r.RemoteAddr,
	}
	if claims, ok := auth.FromContext(r.Context()); ok {
		auditParams.UserID = &claims.UserID
		auditParams.Username = claims.Username
	}
	if err := s.store.CreateAuditEntry(r.Context(), auditParams); err != nil {
		s.logger.Warn("audit log: delete user", "err", err, "user_id", id)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if !isValidRole(req.Role) {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "role must be one of: viewer, operator, admin")
		return
	}
	if err := s.store.UpdateUserRole(r.Context(), id, req.Role); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "user not found")
			return
		}
		s.logger.Error("update user role", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true})
}

func isValidRole(role string) bool {
	return role == "viewer" || role == "operator" || role == "admin"
}

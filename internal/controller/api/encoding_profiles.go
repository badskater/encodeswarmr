package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/badskater/encodeswarmr/internal/controller/auth"
	"github.com/badskater/encodeswarmr/internal/db"
)

// handleListEncodingProfiles returns all encoding profiles.
// GET /api/v1/encoding-profiles
func (s *Server) handleListEncodingProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := s.store.ListEncodingProfiles(r.Context())
	if err != nil {
		s.logger.Error("list encoding profiles", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	if profiles == nil {
		profiles = []*db.EncodingProfile{}
	}
	writeJSON(w, r, http.StatusOK, profiles)
}

// handleGetEncodingProfile returns a single encoding profile by ID.
// GET /api/v1/encoding-profiles/{id}
func (s *Server) handleGetEncodingProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	profile, err := s.store.GetEncodingProfileByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "encoding profile not found")
		return
	}
	if err != nil {
		s.logger.Error("get encoding profile", "err", err, "id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, profile)
}

// handleCreateEncodingProfile creates a new encoding profile.
// POST /api/v1/encoding-profiles
func (s *Server) handleCreateEncodingProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Container   string          `json:"container"`
		Settings    json.RawMessage `json:"settings"`
		AudioConfig json.RawMessage `json:"audio_config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "name is required")
		return
	}
	if req.Container == "" {
		req.Container = "mkv"
	}
	if len(req.Settings) == 0 {
		req.Settings = json.RawMessage(`{}`)
	}

	createdBy := ""
	if claims, ok := auth.FromContext(r.Context()); ok {
		createdBy = claims.Username
	}

	profile, err := s.store.CreateEncodingProfile(r.Context(), db.CreateEncodingProfileParams{
		Name:        req.Name,
		Description: req.Description,
		Container:   req.Container,
		Settings:    req.Settings,
		AudioConfig: req.AudioConfig,
		CreatedBy:   createdBy,
	})
	if err != nil {
		s.logger.Error("create encoding profile", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Audit
	if claims, ok := auth.FromContext(r.Context()); ok {
		_ = s.store.CreateAuditEntry(r.Context(), db.CreateAuditEntryParams{
			UserID:     &claims.UserID,
			Username:   claims.Username,
			Action:     "encoding_profile.create",
			Resource:   "encoding_profile",
			ResourceID: profile.ID,
			IPAddress:  r.RemoteAddr,
		})
	}

	writeJSON(w, r, http.StatusCreated, profile)
}

// handleUpdateEncodingProfile updates an existing encoding profile.
// PUT /api/v1/encoding-profiles/{id}
func (s *Server) handleUpdateEncodingProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Verify it exists first.
	if _, err := s.store.GetEncodingProfileByID(r.Context(), id); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "encoding profile not found")
			return
		}
		s.logger.Error("get encoding profile for update", "err", err, "id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	var req struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Container   string          `json:"container"`
		Settings    json.RawMessage `json:"settings"`
		AudioConfig json.RawMessage `json:"audio_config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "name is required")
		return
	}
	if req.Container == "" {
		req.Container = "mkv"
	}
	if len(req.Settings) == 0 {
		req.Settings = json.RawMessage(`{}`)
	}

	profile, err := s.store.UpdateEncodingProfile(r.Context(), db.UpdateEncodingProfileParams{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Container:   req.Container,
		Settings:    req.Settings,
		AudioConfig: req.AudioConfig,
	})
	if err != nil {
		s.logger.Error("update encoding profile", "err", err, "id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, profile)
}

// handleDeleteEncodingProfile deletes an encoding profile by ID.
// DELETE /api/v1/encoding-profiles/{id}
func (s *Server) handleDeleteEncodingProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteEncodingProfile(r.Context(), id); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "encoding profile not found")
			return
		}
		s.logger.Error("delete encoding profile", "err", err, "id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true})
}

package api

import (
	"encoding/json"
	"errors"
	"net/http"

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
		Name                  string   `json:"name"`
		Description           string   `json:"description"`
		RunTemplateID         string   `json:"run_template_id"`
		FrameserverTemplateID string   `json:"frameserver_template_id"`
		AudioCodec            string   `json:"audio_codec"`
		AudioBitrate          string   `json:"audio_bitrate"`
		OutputExtension       string   `json:"output_extension"`
		OutputPathPattern     string   `json:"output_path_pattern"`
		TargetTags            []string `json:"target_tags"`
		Priority              int      `json:"priority"`
		Enabled               bool     `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "name is required")
		return
	}
	if req.RunTemplateID == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "run_template_id is required")
		return
	}
	if req.TargetTags == nil {
		req.TargetTags = []string{}
	}
	if req.OutputExtension == "" {
		req.OutputExtension = "mkv"
	}
	if req.Priority == 0 {
		req.Priority = 5
	}

	profile, err := s.store.CreateEncodingProfile(r.Context(), db.CreateEncodingProfileParams{
		Name:                  req.Name,
		Description:           req.Description,
		RunTemplateID:         req.RunTemplateID,
		FrameserverTemplateID: req.FrameserverTemplateID,
		AudioCodec:            req.AudioCodec,
		AudioBitrate:          req.AudioBitrate,
		OutputExtension:       req.OutputExtension,
		OutputPathPattern:     req.OutputPathPattern,
		TargetTags:            req.TargetTags,
		Priority:              req.Priority,
		Enabled:               req.Enabled,
	})
	if err != nil {
		s.logger.Error("create encoding profile", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusCreated, profile)
}

// handleUpdateEncodingProfile replaces all fields on an existing encoding profile.
// PUT /api/v1/encoding-profiles/{id}
func (s *Server) handleUpdateEncodingProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

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
		Name                  string   `json:"name"`
		Description           string   `json:"description"`
		RunTemplateID         string   `json:"run_template_id"`
		FrameserverTemplateID string   `json:"frameserver_template_id"`
		AudioCodec            string   `json:"audio_codec"`
		AudioBitrate          string   `json:"audio_bitrate"`
		OutputExtension       string   `json:"output_extension"`
		OutputPathPattern     string   `json:"output_path_pattern"`
		TargetTags            []string `json:"target_tags"`
		Priority              int      `json:"priority"`
		Enabled               bool     `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "name is required")
		return
	}
	if req.RunTemplateID == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "run_template_id is required")
		return
	}
	if req.TargetTags == nil {
		req.TargetTags = []string{}
	}

	profile, err := s.store.UpdateEncodingProfile(r.Context(), db.UpdateEncodingProfileParams{
		ID:                    id,
		Name:                  req.Name,
		Description:           req.Description,
		RunTemplateID:         req.RunTemplateID,
		FrameserverTemplateID: req.FrameserverTemplateID,
		AudioCodec:            req.AudioCodec,
		AudioBitrate:          req.AudioBitrate,
		OutputExtension:       req.OutputExtension,
		OutputPathPattern:     req.OutputPathPattern,
		TargetTags:            req.TargetTags,
		Priority:              req.Priority,
		Enabled:               req.Enabled,
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

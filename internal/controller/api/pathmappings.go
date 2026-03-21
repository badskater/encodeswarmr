package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/badskater/encodeswarmr/internal/db"
)

// handleListPathMappings returns all path mappings.
// GET /api/v1/path-mappings
func (s *Server) handleListPathMappings(w http.ResponseWriter, r *http.Request) {
	mappings, err := s.store.ListPathMappings(r.Context())
	if err != nil {
		s.logger.Error("list path mappings", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, mappings)
}

// handleCreatePathMapping creates a new path mapping.
// POST /api/v1/path-mappings
func (s *Server) handleCreatePathMapping(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name          string `json:"name"`
		WindowsPrefix string `json:"windows_prefix"`
		LinuxPrefix   string `json:"linux_prefix"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Name == "" || req.WindowsPrefix == "" || req.LinuxPrefix == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error",
			"name, windows_prefix, and linux_prefix are required")
		return
	}

	m, err := s.store.CreatePathMapping(r.Context(), db.CreatePathMappingParams{
		Name:          req.Name,
		WindowsPrefix: req.WindowsPrefix,
		LinuxPrefix:   req.LinuxPrefix,
	})
	if err != nil {
		s.logger.Error("create path mapping", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusCreated, m)
}

// handleGetPathMapping returns a single path mapping by ID.
// GET /api/v1/path-mappings/{id}
func (s *Server) handleGetPathMapping(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := s.store.GetPathMappingByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "path mapping not found")
		return
	}
	if err != nil {
		s.logger.Error("get path mapping", "err", err, "id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, m)
}

// handleUpdatePathMapping updates an existing path mapping.
// PUT /api/v1/path-mappings/{id}
func (s *Server) handleUpdatePathMapping(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Name          string `json:"name"`
		WindowsPrefix string `json:"windows_prefix"`
		LinuxPrefix   string `json:"linux_prefix"`
		Enabled       bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Name == "" || req.WindowsPrefix == "" || req.LinuxPrefix == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error",
			"name, windows_prefix, and linux_prefix are required")
		return
	}

	m, err := s.store.UpdatePathMapping(r.Context(), db.UpdatePathMappingParams{
		ID:            id,
		Name:          req.Name,
		WindowsPrefix: req.WindowsPrefix,
		LinuxPrefix:   req.LinuxPrefix,
		Enabled:       req.Enabled,
	})
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "path mapping not found")
		return
	}
	if err != nil {
		s.logger.Error("update path mapping", "err", err, "id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, m)
}

// handleDeletePathMapping deletes a path mapping.
// DELETE /api/v1/path-mappings/{id}
func (s *Server) handleDeletePathMapping(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.store.DeletePathMapping(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "path mapping not found")
		return
	}
	if err != nil {
		s.logger.Error("delete path mapping", "err", err, "id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

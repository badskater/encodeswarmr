package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/badskater/encodeswarmr/internal/db"
)

func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	templateType := r.URL.Query().Get("type")
	templates, err := s.store.ListTemplates(r.Context(), templateType)
	if err != nil {
		s.logger.Error("list templates", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, templates)
}

func (s *Server) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tmpl, err := s.store.GetTemplateByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "template not found")
			return
		}
		s.logger.Error("get template", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, tmpl)
}

func (s *Server) handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Type        string `json:"type"`
		Extension   string `json:"extension"`
		Content     string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Name == "" || req.Type == "" || req.Extension == "" || req.Content == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "name, type, extension, and content are required")
		return
	}
	validTypes := map[string]bool{"run": true, "run_script": true, "frameserver": true, "avs": true, "vpy": true, "bat": true}
	if !validTypes[req.Type] {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "invalid template type: must be one of run, run_script, frameserver, avs, vpy, bat")
		return
	}

	tmpl, err := s.store.CreateTemplate(r.Context(), db.CreateTemplateParams{
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Extension:   req.Extension,
		Content:     req.Content,
	})
	if err != nil {
		s.logger.Error("create template", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusCreated, tmpl)
}

func (s *Server) handleUpdateTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Content     string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}

	if err := s.store.UpdateTemplate(r.Context(), db.UpdateTemplateParams{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Content:     req.Content,
	}); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "template not found")
			return
		}
		s.logger.Error("update template", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	tmpl, err := s.store.GetTemplateByID(r.Context(), id)
	if err != nil {
		s.logger.Error("get template after update", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, tmpl)
}

func (s *Server) handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteTemplate(r.Context(), id); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "template not found")
			return
		}
		s.logger.Error("delete template", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

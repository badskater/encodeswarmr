package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/badskater/encodeswarmr/internal/controller/engine"
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

// handlePreviewTemplate renders a template with the provided source ID and
// extra variables, returning the rendered script content without executing it.
// POST /api/v1/templates/{id}/preview
func (s *Server) handlePreviewTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		SourceID  string            `json:"source_id"`
		Variables map[string]string `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}

	tmpl, err := s.store.GetTemplateByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "template not found")
			return
		}
		s.logger.Error("preview template: get template", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Build data map: global variables from DB, then request-supplied variables.
	globalVars, err := s.store.ListVariables(r.Context(), "")
	if err != nil {
		s.logger.Error("preview template: list variables", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	data := make(map[string]string, len(globalVars)+len(req.Variables)+10)
	for _, v := range globalVars {
		data[v.Name] = v.Value
	}

	// Overlay request-supplied variables (simulates job extra_vars + task vars).
	for k, v := range req.Variables {
		data[k] = v
	}

	// Seed placeholder built-ins if not already supplied by the caller.
	if req.SourceID != "" {
		src, srcErr := s.store.GetSourceByID(r.Context(), req.SourceID)
		if srcErr == nil {
			if _, ok := data["SOURCE_PATH"]; !ok {
				data["SOURCE_PATH"] = src.UNCPath
			}
			if _, ok := data["HDR_TYPE"]; !ok {
				data["HDR_TYPE"] = src.HDRType
			}
		}
	}

	// Provide defaults for built-ins that callers typically don't supply.
	defaults := map[string]string{
		"SOURCE_PATH":  "\\\\NAS01\\media\\source.mkv",
		"OUTPUT_PATH":  "\\\\NAS01\\media\\output\\chunk_0000.mkv",
		"START_FRAME":  "0",
		"END_FRAME":    "23975",
		"CHUNK_INDEX":  "0",
		"TOTAL_CHUNKS": "1",
		"JOB_ID":       "00000000-0000-0000-0000-000000000000",
		"TASK_ID":      "00000000-0000-0000-0000-000000000001",
		"HDR_TYPE":     "",
		"DV_PROFILE":   "0",
	}
	for k, v := range defaults {
		if _, ok := data[k]; !ok {
			data[k] = v
		}
	}

	rendered, renderErr := engine.RenderTemplatePreview(tmpl.Name, tmpl.Content, data)
	if renderErr != nil {
		// Return the error as a 422 so the UI can display it to the user.
		type previewError struct {
			Error string `json:"error"`
		}
		writeJSON(w, r, http.StatusUnprocessableEntity, previewError{Error: renderErr.Error()})
		return
	}

	type previewResponse struct {
		TemplateName string `json:"template_name"`
		Extension    string `json:"extension"`
		Content      string `json:"content"`
	}
	writeJSON(w, r, http.StatusOK, previewResponse{
		TemplateName: tmpl.Name,
		Extension:    tmpl.Extension,
		Content:      rendered,
	})
}

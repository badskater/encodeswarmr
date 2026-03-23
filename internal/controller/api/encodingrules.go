package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/badskater/encodeswarmr/internal/controller/rules"
	"github.com/badskater/encodeswarmr/internal/db"
)

// handleListEncodingRules returns all encoding rules ordered by priority.
//
// GET /api/v1/encoding-rules
func (s *Server) handleListEncodingRules(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListEncodingRules(r.Context())
	if err != nil {
		s.logger.Error("list encoding rules", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, items)
}

// handleGetEncodingRule returns a single encoding rule by ID.
//
// GET /api/v1/encoding-rules/{id}
func (s *Server) handleGetEncodingRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rule, err := s.store.GetEncodingRuleByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "encoding rule not found")
		return
	}
	if err != nil {
		s.logger.Error("get encoding rule", "err", err, "id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, rule)
}

// handleCreateEncodingRule creates a new encoding rule.
//
// POST /api/v1/encoding-rules
func (s *Server) handleCreateEncodingRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string              `json:"name"`
		Priority   int                 `json:"priority"`
		Conditions []db.RuleCondition  `json:"conditions"`
		Actions    db.RuleAction       `json:"actions"`
		Enabled    bool                `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "name is required")
		return
	}

	rule, err := s.store.CreateEncodingRule(r.Context(), db.CreateEncodingRuleParams{
		Name:       req.Name,
		Priority:   req.Priority,
		Conditions: req.Conditions,
		Actions:    req.Actions,
		Enabled:    req.Enabled,
	})
	if err != nil {
		s.logger.Error("create encoding rule", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusCreated, rule)
}

// handleUpdateEncodingRule replaces an encoding rule.
//
// PUT /api/v1/encoding-rules/{id}
func (s *Server) handleUpdateEncodingRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Name       string              `json:"name"`
		Priority   int                 `json:"priority"`
		Conditions []db.RuleCondition  `json:"conditions"`
		Actions    db.RuleAction       `json:"actions"`
		Enabled    bool                `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "name is required")
		return
	}

	rule, err := s.store.UpdateEncodingRule(r.Context(), db.UpdateEncodingRuleParams{
		ID:         id,
		Name:       req.Name,
		Priority:   req.Priority,
		Conditions: req.Conditions,
		Actions:    req.Actions,
		Enabled:    req.Enabled,
	})
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "encoding rule not found")
		return
	}
	if err != nil {
		s.logger.Error("update encoding rule", "err", err, "id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, rule)
}

// handleDeleteEncodingRule deletes an encoding rule by ID.
//
// DELETE /api/v1/encoding-rules/{id}
func (s *Server) handleDeleteEncodingRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.store.DeleteEncodingRule(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "encoding rule not found")
		return
	}
	if err != nil {
		s.logger.Error("delete encoding rule", "err", err, "id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleEvaluateEncodingRules evaluates the rules engine against a source's
// properties and returns a suggested RuleAction (if any rule matches).
// This is a read-only suggestion; it never creates jobs.
//
// POST /api/v1/encoding-rules/evaluate
// Body: { "source_id": "...", "resolution": "...", "hdr_type": "...",
//         "codec": "...", "file_size_gb": 0.0, "duration_min": 0.0 }
func (s *Server) handleEvaluateEncodingRules(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceID    string  `json:"source_id"`
		Resolution  string  `json:"resolution"`
		HDRType     string  `json:"hdr_type"`
		Codec       string  `json:"codec"`
		FileSizeGB  float64 `json:"file_size_gb"`
		DurationMin float64 `json:"duration_min"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}

	// If source_id is provided, attempt to fill missing fields from the DB.
	if req.SourceID != "" {
		if src, err := s.store.GetSourceByID(r.Context(), req.SourceID); err == nil {
			if req.HDRType == "" {
				req.HDRType = src.HDRType
			}
			if req.FileSizeGB == 0 && src.SizeBytes > 0 {
				req.FileSizeGB = float64(src.SizeBytes) / (1024 * 1024 * 1024)
			}
		}
	}

	props := rules.SourceProperties{
		Resolution:  req.Resolution,
		HDRType:     req.HDRType,
		Codec:       req.Codec,
		FileSizeGB:  req.FileSizeGB,
		DurationMin: req.DurationMin,
	}

	action, err := s.rulesEngine.Evaluate(r.Context(), props)
	if err != nil {
		s.logger.Error("evaluate encoding rules", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	if action == nil {
		writeJSON(w, r, http.StatusOK, map[string]any{"matched": false, "suggestion": nil})
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"matched":    true,
		"suggestion": action,
	})
}

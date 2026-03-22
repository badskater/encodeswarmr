package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/badskater/encodeswarmr/internal/db"
)

func (s *Server) handleListVariables(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	vars, err := s.store.ListVariables(r.Context(), category)
	if err != nil {
		s.logger.Error("list variables", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, vars)
}

func (s *Server) handleGetVariable(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	v, err := s.store.GetVariableByName(r.Context(), name)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "variable not found")
			return
		}
		s.logger.Error("get variable", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, v)
}

func (s *Server) handleUpsertVariable(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Value       string `json:"value"`
		Description string `json:"description"`
		Category    string `json:"category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}

	v, err := s.store.UpsertVariable(r.Context(), db.UpsertVariableParams{
		Name:        name,
		Value:       req.Value,
		Description: req.Description,
		Category:    req.Category,
	})
	if err != nil {
		s.logger.Error("upsert variable", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, v)
}

func (s *Server) handleDeleteVariable(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteVariable(r.Context(), id); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "variable not found")
			return
		}
		s.logger.Error("delete variable", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

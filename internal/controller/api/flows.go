package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/badskater/distributed-encoder/internal/db"
)

// handleListFlows returns all flows ordered by most-recently-updated.
func (s *Server) handleListFlows(w http.ResponseWriter, r *http.Request) {
	flows, err := s.store.ListFlows(r.Context())
	if err != nil {
		s.logger.Error("list flows", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, flows)
}

// handleGetFlow returns a single flow by ID.
func (s *Server) handleGetFlow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	flow, err := s.store.GetFlowByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "flow not found")
			return
		}
		s.logger.Error("get flow", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, flow)
}

// handleCreateFlow creates a new flow pipeline.
func (s *Server) handleCreateFlow(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Graph       json.RawMessage `json:"graph"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "name is required")
		return
	}
	if len(req.Graph) == 0 {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "graph is required")
		return
	}

	flow, err := s.store.CreateFlow(r.Context(), db.CreateFlowParams{
		Name:        req.Name,
		Description: req.Description,
		Graph:       req.Graph,
	})
	if err != nil {
		s.logger.Error("create flow", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusCreated, flow)
}

// handleUpdateFlow replaces a flow's name, description, and graph.
func (s *Server) handleUpdateFlow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Graph       json.RawMessage `json:"graph"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "name is required")
		return
	}
	if len(req.Graph) == 0 {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "graph is required")
		return
	}

	flow, err := s.store.UpdateFlow(r.Context(), db.UpdateFlowParams{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Graph:       req.Graph,
	})
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "flow not found")
			return
		}
		s.logger.Error("update flow", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, flow)
}

// handleDeleteFlow removes a flow by ID.
func (s *Server) handleDeleteFlow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteFlow(r.Context(), id); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "flow not found")
			return
		}
		s.logger.Error("delete flow", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

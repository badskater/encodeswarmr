package api

import (
	"errors"
	"net/http"

	"github.com/badskater/distributed-encoder/internal/db"
)

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := s.store.ListAgents(r.Context())
	if err != nil {
		s.logger.Error("list agents", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, agents)
}

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	agent, err := s.store.GetAgentByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "agent not found")
		return
	}
	if err != nil {
		s.logger.Error("get agent", "err", err, "agent_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, agent)
}

func (s *Server) handleDrainAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := s.store.GetAgentByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "agent not found")
		return
	}
	if err != nil {
		s.logger.Error("get agent for drain", "err", err, "agent_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	if err := s.store.UpdateAgentStatus(r.Context(), id, "draining"); err != nil {
		s.logger.Error("drain agent", "err", err, "agent_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleApproveAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	agent, err := s.store.GetAgentByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "agent not found")
		return
	}
	if err != nil {
		s.logger.Error("get agent for approve", "err", err, "agent_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	if agent.Status != "pending_approval" {
		writeProblem(w, r, http.StatusConflict, "Conflict", "agent is not pending approval")
		return
	}
	if err := s.store.UpdateAgentStatus(r.Context(), id, "idle"); err != nil {
		s.logger.Error("approve agent", "err", err, "agent_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true})
}

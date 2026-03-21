package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/badskater/distributed-encoder/internal/controller/auth"
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

	// Emit audit entry — best-effort, never fail the request.
	auditParams := db.CreateAuditEntryParams{
		Action:     "agent.approve",
		Resource:   "agent",
		ResourceID: id,
		IPAddress:  r.RemoteAddr,
	}
	if claims, ok := auth.FromContext(r.Context()); ok {
		auditParams.UserID = &claims.UserID
		auditParams.Username = claims.Username
	}
	if err := s.store.CreateAuditEntry(r.Context(), auditParams); err != nil {
		s.logger.Warn("audit log: approve agent", "err", err, "agent_id", id)
	}

	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true})
}

// handleGetAgentMetrics returns time-series CPU/GPU/memory samples for an agent.
// GET /api/v1/agents/{id}/metrics
// Optional query param: window=1h (default) — duration string parsed by time.ParseDuration.
func (s *Server) handleGetAgentMetrics(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Verify agent exists.
	if _, err := s.store.GetAgentByID(r.Context(), id); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "agent not found")
			return
		}
		s.logger.Error("get agent for metrics", "err", err, "agent_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	window := time.Hour
	if wq := r.URL.Query().Get("window"); wq != "" {
		if d, err := time.ParseDuration(wq); err == nil && d > 0 {
			window = d
		}
	}

	since := time.Now().Add(-window)
	metrics, err := s.store.ListAgentMetrics(r.Context(), id, since)
	if err != nil {
		s.logger.Error("list agent metrics", "err", err, "agent_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	if metrics == nil {
		metrics = []*db.AgentMetric{}
	}
	writeJSON(w, r, http.StatusOK, metrics)
}

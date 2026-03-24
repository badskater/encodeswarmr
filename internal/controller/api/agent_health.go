package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/badskater/encodeswarmr/internal/db"
)

// handleGetAgentHealth returns deep-dive health statistics for a single agent.
// GET /api/v1/agents/{id}/health
func (s *Server) handleGetAgentHealth(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	agent, err := s.store.GetAgentByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "agent not found")
		return
	}
	if err != nil {
		s.logger.Error("get agent for health", "err", err, "agent_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	stats, err := s.store.GetAgentEncodingStats(r.Context(), id)
	if err != nil {
		s.logger.Error("get agent encoding stats", "err", err, "agent_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"agent":          agent,
		"encoding_stats": stats,
	})
}

// handleListAgentRecentTasks returns the most recent tasks that ran on an agent.
// GET /api/v1/agents/{id}/recent-tasks
// Optional query param: limit (default 20, max 100)
func (s *Server) handleListAgentRecentTasks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if _, err := s.store.GetAgentByID(r.Context(), id); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "agent not found")
			return
		}
		s.logger.Error("get agent for recent tasks", "err", err, "agent_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	tasks, err := s.store.ListRecentTasksByAgent(r.Context(), id, limit)
	if err != nil {
		s.logger.Error("list recent tasks by agent", "err", err, "agent_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	if tasks == nil {
		tasks = []*db.Task{}
	}
	writeJSON(w, r, http.StatusOK, tasks)
}

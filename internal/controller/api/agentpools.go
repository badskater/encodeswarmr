package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/badskater/encodeswarmr/internal/db"
)

// handleListAgentPools returns all agent pools ordered by name.
// GET /api/v1/agent-pools
func (s *Server) handleListAgentPools(w http.ResponseWriter, r *http.Request) {
	pools, err := s.store.ListAgentPools(r.Context())
	if err != nil {
		s.logger.Error("list agent pools", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	if pools == nil {
		pools = []*db.AgentPool{}
	}
	writeJSON(w, r, http.StatusOK, pools)
}

// handleCreateAgentPool creates a new agent pool.
// POST /api/v1/agent-pools
func (s *Server) handleCreateAgentPool(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
		Color       string   `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "name is required")
		return
	}
	if req.Color == "" {
		req.Color = "#6366f1"
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}

	pool, err := s.store.CreateAgentPool(r.Context(), db.CreateAgentPoolParams{
		Name:        req.Name,
		Description: req.Description,
		Tags:        req.Tags,
		Color:       req.Color,
	})
	if err != nil {
		s.logger.Error("create agent pool", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusCreated, pool)
}

// handleUpdateAgentPool updates an existing agent pool.
// PUT /api/v1/agent-pools/{id}
func (s *Server) handleUpdateAgentPool(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if _, err := s.store.GetAgentPoolByID(r.Context(), id); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "agent pool not found")
			return
		}
		s.logger.Error("get agent pool for update", "err", err, "pool_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
		Color       string   `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "name is required")
		return
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}

	pool, err := s.store.UpdateAgentPool(r.Context(), db.UpdateAgentPoolParams{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Tags:        req.Tags,
		Color:       req.Color,
	})
	if err != nil {
		s.logger.Error("update agent pool", "err", err, "pool_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, pool)
}

// handleDeleteAgentPool removes an agent pool.
// DELETE /api/v1/agent-pools/{id}
func (s *Server) handleDeleteAgentPool(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteAgentPool(r.Context(), id); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "agent pool not found")
			return
		}
		s.logger.Error("delete agent pool", "err", err, "pool_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true})
}

// handleAssignAgentToPool adds a pool's tags to an agent's tags.
// POST /api/v1/agents/{id}/pools
func (s *Server) handleAssignAgentToPool(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("id")

	var req struct {
		PoolID string `json:"pool_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.PoolID == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "pool_id is required")
		return
	}

	agent, err := s.store.GetAgentByID(r.Context(), agentID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "agent not found")
			return
		}
		s.logger.Error("get agent for pool assignment", "err", err, "agent_id", agentID)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	pool, err := s.store.GetAgentPoolByID(r.Context(), req.PoolID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "agent pool not found")
			return
		}
		s.logger.Error("get pool for assignment", "err", err, "pool_id", req.PoolID)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Merge pool tags into agent tags (deduplicating).
	merged := mergeTagSlices(agent.Tags, pool.Tags)
	if err := s.store.UpdateAgentTags(r.Context(), db.UpdateAgentTagsParams{ID: agentID, Tags: merged}); err != nil {
		s.logger.Error("update agent tags for pool assignment", "err", err, "agent_id", agentID)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true})
}

// handleRemoveAgentFromPool removes a pool's tags from an agent's tags.
// DELETE /api/v1/agents/{id}/pools/{pool_id}
func (s *Server) handleRemoveAgentFromPool(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("id")
	poolID := r.PathValue("pool_id")

	agent, err := s.store.GetAgentByID(r.Context(), agentID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "agent not found")
			return
		}
		s.logger.Error("get agent for pool removal", "err", err, "agent_id", agentID)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	pool, err := s.store.GetAgentPoolByID(r.Context(), poolID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "agent pool not found")
			return
		}
		s.logger.Error("get pool for removal", "err", err, "pool_id", poolID)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Remove pool tags from agent tags.
	poolTagSet := make(map[string]bool, len(pool.Tags))
	for _, t := range pool.Tags {
		poolTagSet[t] = true
	}
	var remaining []string
	for _, t := range agent.Tags {
		if !poolTagSet[t] {
			remaining = append(remaining, t)
		}
	}
	if remaining == nil {
		remaining = []string{}
	}

	if err := s.store.UpdateAgentTags(r.Context(), db.UpdateAgentTagsParams{ID: agentID, Tags: remaining}); err != nil {
		s.logger.Error("update agent tags for pool removal", "err", err, "agent_id", agentID)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true})
}

// mergeTagSlices combines two tag slices, deduplicating while preserving order.
func mergeTagSlices(base, additional []string) []string {
	seen := make(map[string]bool, len(base)+len(additional))
	out := make([]string, 0, len(base)+len(additional))
	for _, t := range base {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	for _, t := range additional {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

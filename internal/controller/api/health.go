package api

import (
	"context"
	"net/http"
	"time"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if err := s.store.Ping(ctx); err != nil {
		s.logger.Warn("health: db ping failed", "error", err)
		writeJSON(w, r, http.StatusServiceUnavailable, map[string]any{
			"status": "degraded",
			"db":     "unreachable",
			"leader": s.leader.IsLeader(),
		})
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"status": "ok",
		"db":     "ok",
		"leader": s.leader.IsLeader(),
	})
}

func (s *Server) handleHAStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, map[string]any{
		"leader":  s.leader.IsLeader(),
		"node_id": s.leader.NodeID(),
	})
}

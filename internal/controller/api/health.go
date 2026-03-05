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
		})
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]any{"status": "ok", "db": "ok"})
}

package api

import (
	"fmt"
	"net/http"
)

// handleQueueStatus returns current queue state including pause status, pending
// count, running count, and estimated completion.
// GET /api/v1/queue/status
func (s *Server) handleQueueStatus(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.GetQueueStats(r.Context())
	if err != nil {
		s.logger.Error("get queue stats", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	paused := false
	if s.eng != nil {
		paused = s.eng.IsPaused()
	}

	var estCompletion string
	if stats.EstimatedMinutes > 0 {
		h := int(stats.EstimatedMinutes) / 60
		m := int(stats.EstimatedMinutes) % 60
		if h > 0 {
			estCompletion = fmt.Sprintf("%dh %dm", h, m)
		} else {
			estCompletion = fmt.Sprintf("%dm", m)
		}
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"paused":               paused,
		"pending":              stats.Pending,
		"running":              stats.Running,
		"estimated_completion": estCompletion,
	})
}

// handlePauseQueue pauses the dispatch engine.
// POST /api/v1/queue/pause
func (s *Server) handlePauseQueue(w http.ResponseWriter, r *http.Request) {
	if s.eng != nil {
		s.eng.Pause()
		s.logger.Info("queue dispatching paused")
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true, "paused": true})
}

// handleResumeQueue resumes the dispatch engine.
// POST /api/v1/queue/resume
func (s *Server) handleResumeQueue(w http.ResponseWriter, r *http.Request) {
	if s.eng != nil {
		s.eng.Resume()
		s.logger.Info("queue dispatching resumed")
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true, "paused": false})
}

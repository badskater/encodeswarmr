package api

import (
	"net/http"
	"strconv"

	"github.com/badskater/encodeswarmr/internal/db"
)

// handleThroughput returns the number of jobs completed per hour over the last
// N hours (default 24).  Requires viewer role.
func (s *Server) handleThroughput(w http.ResponseWriter, r *http.Request) {
	hours := 24
	if h := r.URL.Query().Get("hours"); h != "" {
		n, err := strconv.Atoi(h)
		if err != nil || n < 1 {
			writeProblem(w, r, http.StatusBadRequest, "Bad Request", "hours must be a positive integer")
			return
		}
		hours = n
	}

	points, err := s.store.GetThroughputStats(r.Context(), hours)
	if err != nil {
		s.logger.Error("dashboard: throughput stats", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Return an empty slice rather than null when there are no data points.
	if points == nil {
		points = []*db.ThroughputPoint{}
	}

	writeJSON(w, r, http.StatusOK, points)
}

// handleQueueSummary returns the current pending/running task counts and an
// estimated completion time in minutes.  Requires viewer role.
func (s *Server) handleQueueSummary(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.GetQueueStats(r.Context())
	if err != nil {
		s.logger.Error("dashboard: queue stats", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, stats)
}

// handleRecentActivity returns recent job status changes, limited to the last
// N events (default 10, max 100).  Requires viewer role.
func (s *Server) handleRecentActivity(w http.ResponseWriter, r *http.Request) {
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err != nil || n < 1 {
			writeProblem(w, r, http.StatusBadRequest, "Bad Request", "limit must be a positive integer")
			return
		}
		if n > 100 {
			n = 100
		}
		limit = n
	}

	events, err := s.store.GetRecentActivity(r.Context(), limit)
	if err != nil {
		s.logger.Error("dashboard: recent activity", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Return an empty slice rather than null when there are no events.
	if events == nil {
		events = []*db.ActivityEvent{}
	}

	writeJSON(w, r, http.StatusOK, events)
}

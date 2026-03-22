package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/scheduler"
	"github.com/badskater/encodeswarmr/internal/db"
)

// scheduleResponse is the public representation of a schedule.
type scheduleResponse struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	CronExpr    string          `json:"cron_expr"`
	JobTemplate json.RawMessage `json:"job_template"`
	Enabled     bool            `json:"enabled"`
	LastRunAt   *string         `json:"last_run_at,omitempty"`
	NextRunAt   *string         `json:"next_run_at,omitempty"`
	CreatedAt   string          `json:"created_at"`
}

func toScheduleResponse(sc *db.Schedule) scheduleResponse {
	r := scheduleResponse{
		ID:          sc.ID,
		Name:        sc.Name,
		CronExpr:    sc.CronExpr,
		JobTemplate: sc.JobTemplate,
		Enabled:     sc.Enabled,
		CreatedAt:   timeStr(sc.CreatedAt),
	}
	if sc.LastRunAt != nil {
		s := timeStr(*sc.LastRunAt)
		r.LastRunAt = &s
	}
	if sc.NextRunAt != nil {
		s := timeStr(*sc.NextRunAt)
		r.NextRunAt = &s
	}
	return r
}

// handleListSchedules handles GET /api/v1/schedules.
func (s *Server) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	schedules, err := s.store.ListSchedules(r.Context())
	if err != nil {
		s.logger.Error("list schedules", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	out := make([]scheduleResponse, len(schedules))
	for i, sc := range schedules {
		out[i] = toScheduleResponse(sc)
	}
	writeJSON(w, r, http.StatusOK, out)
}

// handleGetSchedule handles GET /api/v1/schedules/{id}.
func (s *Server) handleGetSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sc, err := s.store.GetScheduleByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "schedule not found")
			return
		}
		s.logger.Error("get schedule", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, toScheduleResponse(sc))
}

// handleCreateSchedule handles POST /api/v1/schedules.
func (s *Server) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string          `json:"name"`
		CronExpr    string          `json:"cron_expr"`
		JobTemplate json.RawMessage `json:"job_template"`
		Enabled     *bool           `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Name == "" || req.CronExpr == "" || len(req.JobTemplate) == 0 {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error",
			"name, cron_expr, and job_template are required")
		return
	}

	nextRunAt, err := scheduler.NextRunFromExpr(req.CronExpr)
	if err != nil {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error",
			"invalid cron_expr: "+err.Error())
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	// If disabled at creation, do not set a next run time.
	var nextRunPtr *time.Time
	if enabled {
		nextRunPtr = nextRunAt
	}

	sc, err := s.store.CreateSchedule(r.Context(), db.CreateScheduleParams{
		Name:        req.Name,
		CronExpr:    req.CronExpr,
		JobTemplate: req.JobTemplate,
		Enabled:     enabled,
		NextRunAt:   nextRunPtr,
	})
	if err != nil {
		s.logger.Error("create schedule", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusCreated, toScheduleResponse(sc))
}

// handleUpdateSchedule handles PUT /api/v1/schedules/{id}.
func (s *Server) handleUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name        string          `json:"name"`
		CronExpr    string          `json:"cron_expr"`
		JobTemplate json.RawMessage `json:"job_template"`
		Enabled     *bool           `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Name == "" || req.CronExpr == "" || len(req.JobTemplate) == 0 {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error",
			"name, cron_expr, and job_template are required")
		return
	}

	nextRunAt, err := scheduler.NextRunFromExpr(req.CronExpr)
	if err != nil {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error",
			"invalid cron_expr: "+err.Error())
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	var nextRunPtr *time.Time
	if enabled {
		nextRunPtr = nextRunAt
	}

	sc, err := s.store.UpdateSchedule(r.Context(), db.UpdateScheduleParams{
		ID:          id,
		Name:        req.Name,
		CronExpr:    req.CronExpr,
		JobTemplate: req.JobTemplate,
		Enabled:     enabled,
		NextRunAt:   nextRunPtr,
	})
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "schedule not found")
			return
		}
		s.logger.Error("update schedule", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, toScheduleResponse(sc))
}

// handleDeleteSchedule handles DELETE /api/v1/schedules/{id}.
func (s *Server) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteSchedule(r.Context(), id); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "schedule not found")
			return
		}
		s.logger.Error("delete schedule", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

package api

import (
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/badskater/encodeswarmr/internal/db"
)

// handleListArchivedJobs returns a paginated list of archived jobs.
// GET /api/v1/archive/jobs
func (s *Server) handleListArchivedJobs(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")

	pageSize := 50
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		n, err := strconv.Atoi(ps)
		if err != nil || n < 1 {
			writeProblem(w, r, http.StatusBadRequest, "Bad Request", "page_size must be a positive integer")
			return
		}
		pageSize = n
		if pageSize > 200 {
			pageSize = 200
		}
	}

	var cursor string
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid cursor")
			return
		}
		cursor = string(decoded)
	}

	jobs, total, err := s.store.ListArchivedJobs(r.Context(), db.ListJobsFilter{
		Status:   status,
		Cursor:   cursor,
		PageSize: pageSize,
	})
	if err != nil {
		s.logger.Error("list archived jobs", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	writeCollection(w, r, jobs, total, "")
}

// handleExportJobs exports active job history as CSV or JSON.
// GET /api/v1/jobs/export?format=csv&status=completed&from=2024-01-01&to=2024-12-31
func (s *Server) handleExportJobs(w http.ResponseWriter, r *http.Request) {
	s.doExportJobs(w, r, false)
}

// handleExportArchivedJobs exports archived job history.
// GET /api/v1/archive/jobs/export?format=csv
func (s *Server) handleExportArchivedJobs(w http.ResponseWriter, r *http.Request) {
	s.doExportJobs(w, r, true)
}

func (s *Server) doExportJobs(w http.ResponseWriter, r *http.Request, archived bool) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}
	if format != "csv" && format != "json" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "format must be csv or json")
		return
	}

	status := r.URL.Query().Get("status")

	var from, to time.Time
	if f := r.URL.Query().Get("from"); f != "" {
		t, err := time.Parse("2006-01-02", f)
		if err != nil {
			writeProblem(w, r, http.StatusBadRequest, "Bad Request", "from must be YYYY-MM-DD")
			return
		}
		from = t
	}
	if t := r.URL.Query().Get("to"); t != "" {
		parsed, err := time.Parse("2006-01-02", t)
		if err != nil {
			writeProblem(w, r, http.StatusBadRequest, "Bad Request", "to must be YYYY-MM-DD")
			return
		}
		// Include the entire "to" day.
		to = parsed.Add(24*time.Hour - time.Second)
	}

	f := db.ExportJobsFilter{
		Status: status,
		From:   from,
		To:     to,
	}

	var jobs []*db.Job
	var err error
	if archived {
		jobs, err = s.store.ExportArchivedJobs(r.Context(), f)
	} else {
		jobs, err = s.store.ExportJobs(r.Context(), f)
	}
	if err != nil {
		s.logger.Error("export jobs", "err", err, "archived", archived)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	switch format {
	case "csv":
		s.writeJobsCSV(w, jobs, archived)
	default:
		writeJSON(w, r, http.StatusOK, jobs)
	}
}

// writeJobsCSV serialises the jobs slice as a CSV attachment.
func (s *Server) writeJobsCSV(w http.ResponseWriter, jobs []*db.Job, archived bool) {
	filename := "jobs_export.csv"
	if archived {
		filename = "archived_jobs_export.csv"
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"id", "source_id", "source_path", "status", "job_type", "priority",
		"tasks_total", "tasks_completed", "tasks_failed",
		"max_retries",
		"created_at", "updated_at", "completed_at", "failed_at",
	})

	for _, j := range jobs {
		completedAt := ""
		if j.CompletedAt != nil {
			completedAt = j.CompletedAt.Format(time.RFC3339)
		}
		failedAt := ""
		if j.FailedAt != nil {
			failedAt = j.FailedAt.Format(time.RFC3339)
		}
		_ = cw.Write([]string{
			j.ID,
			j.SourceID,
			j.SourcePath,
			j.Status,
			j.JobType,
			strconv.Itoa(j.Priority),
			strconv.Itoa(j.TasksTotal),
			strconv.Itoa(j.TasksCompleted),
			strconv.Itoa(j.TasksFailed),
			strconv.Itoa(j.MaxRetries),
			j.CreatedAt.Format(time.RFC3339),
			j.UpdatedAt.Format(time.RFC3339),
			completedAt,
			failedAt,
		})
	}
	cw.Flush()
}

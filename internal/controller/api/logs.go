package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/badskater/encodeswarmr/internal/db"
)

// handleListTaskLogs returns a paginated list of logs for a task.
func (s *Server) handleListTaskLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "missing task id")
		return
	}

	stream := r.URL.Query().Get("stream")

	pageSize := 100
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		n, err := strconv.Atoi(ps)
		if err != nil || n < 1 {
			writeProblem(w, r, http.StatusBadRequest, "Bad Request", "page_size must be a positive integer")
			return
		}
		pageSize = n
		if pageSize > 1000 {
			pageSize = 1000
		}
	}

	var cursor int64
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeProblem(w, r, http.StatusBadRequest, "Bad Request", "cursor must be a valid integer")
			return
		}
		cursor = n
	}

	logs, err := s.store.ListTaskLogs(r.Context(), db.ListTaskLogsParams{
		TaskID:   id,
		Stream:   stream,
		Cursor:   cursor,
		PageSize: pageSize,
	})
	if err != nil {
		s.logger.Error("list task logs", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	var nextCursor string
	if len(logs) == pageSize {
		nextCursor = strconv.FormatInt(logs[len(logs)-1].ID, 10)
	}
	writeCollection(w, r, logs, int64(len(logs)), nextCursor)
}

// handleTailTaskLogs streams task logs as Server-Sent Events.
func (s *Server) handleTailTaskLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "missing task id")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	var afterID int64
	if raw := r.URL.Query().Get("after_id"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err == nil {
			afterID = n
		}
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			logs, err := s.store.TailTaskLogs(ctx, id, afterID)
			if err != nil {
				s.logger.Error("tail task logs", "err", err)
				return
			}
			for _, lg := range logs {
				data, err := json.Marshal(lg)
				if err != nil {
					s.logger.Error("marshal task log", "err", err)
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", data)
				afterID = lg.ID
			}
			if len(logs) > 0 {
				flusher.Flush()
			}
		}
	}
}

// handleDownloadTaskLogs returns all logs for a task as a plain-text download.
func (s *Server) handleDownloadTaskLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "missing task id")
		return
	}

	logs, err := s.store.ListTaskLogs(r.Context(), db.ListTaskLogsParams{
		TaskID:   id,
		Cursor:   0,
		PageSize: 500000,
	})
	if err != nil {
		s.logger.Error("download task logs", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"task-%s.log\"", id))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	for _, lg := range logs {
		fmt.Fprintf(w, "[%s] [%s/%s] %s\n", lg.LoggedAt.Format(time.RFC3339), lg.Stream, lg.Level, lg.Message)
	}
}

// handleListJobLogs returns a paginated list of logs for all tasks in a job.
func (s *Server) handleListJobLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "missing job id")
		return
	}

	stream := r.URL.Query().Get("stream")

	pageSize := 100
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		n, err := strconv.Atoi(ps)
		if err != nil || n < 1 {
			writeProblem(w, r, http.StatusBadRequest, "Bad Request", "page_size must be a positive integer")
			return
		}
		pageSize = n
		if pageSize > 1000 {
			pageSize = 1000
		}
	}

	var cursor int64
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeProblem(w, r, http.StatusBadRequest, "Bad Request", "cursor must be a valid integer")
			return
		}
		cursor = n
	}

	logs, err := s.store.ListJobLogs(r.Context(), db.ListJobLogsParams{
		JobID:    id,
		Stream:   stream,
		Cursor:   cursor,
		PageSize: pageSize,
	})
	if err != nil {
		s.logger.Error("list job logs", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	var nextCursor string
	if len(logs) == pageSize {
		nextCursor = strconv.FormatInt(logs[len(logs)-1].ID, 10)
	}
	writeCollection(w, r, logs, int64(len(logs)), nextCursor)
}

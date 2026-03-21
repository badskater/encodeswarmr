package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/badskater/distributed-encoder/internal/db"
)

// ---------------------------------------------------------------------------
// handleListTaskLogs
// ---------------------------------------------------------------------------

func TestHandleListTaskLogs_Success(t *testing.T) {
	store := &listTaskLogsStore{
		stubStore: &stubStore{},
		logs: []*db.TaskLog{
			{ID: 1, TaskID: "t1", Stream: "stdout", Level: "info", Message: "Starting encode", LoggedAt: time.Now()},
			{ID: 2, TaskID: "t1", Stream: "stdout", Level: "info", Message: "Done", LoggedAt: time.Now()},
		},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/t1/logs", nil)
	req.SetPathValue("id", "t1")
	srv.handleListTaskLogs(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleListTaskLogs_BadPageSize(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/t1/logs?page_size=abc", nil)
	req.SetPathValue("id", "t1")
	srv.handleListTaskLogs(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleListTaskLogs_ZeroPageSize(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/t1/logs?page_size=0", nil)
	req.SetPathValue("id", "t1")
	srv.handleListTaskLogs(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleListTaskLogs_BadCursor(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/t1/logs?cursor=notanint", nil)
	req.SetPathValue("id", "t1")
	srv.handleListTaskLogs(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleListTaskLogs_StoreError(t *testing.T) {
	store := &listTaskLogsErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/t1/logs", nil)
	req.SetPathValue("id", "t1")
	srv.handleListTaskLogs(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestHandleListTaskLogs_StreamFilter(t *testing.T) {
	store := &listTaskLogsParamsStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/t1/logs?stream=stderr", nil)
	req.SetPathValue("id", "t1")
	srv.handleListTaskLogs(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if store.gotStream != "stderr" {
		t.Errorf("stream = %q, want %q", store.gotStream, "stderr")
	}
}

func TestHandleListTaskLogs_CursorPropagated(t *testing.T) {
	store := &listTaskLogsParamsStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/t1/logs?cursor=42", nil)
	req.SetPathValue("id", "t1")
	srv.handleListTaskLogs(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if store.gotCursor != 42 {
		t.Errorf("cursor = %d, want 42", store.gotCursor)
	}
}

// ---------------------------------------------------------------------------
// handleDownloadTaskLogs
// ---------------------------------------------------------------------------

func TestHandleDownloadTaskLogs_Success(t *testing.T) {
	store := &listTaskLogsStore{
		stubStore: &stubStore{},
		logs: []*db.TaskLog{
			{ID: 1, TaskID: "t1", Stream: "stdout", Level: "info", Message: "line1", LoggedAt: time.Now()},
		},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/t1/logs/download", nil)
	req.SetPathValue("id", "t1")
	srv.handleDownloadTaskLogs(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	disp := rr.Header().Get("Content-Disposition")
	if disp == "" {
		t.Error("Content-Disposition header not set")
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/plain; charset=utf-8")
	}
}

func TestHandleDownloadTaskLogs_StoreError(t *testing.T) {
	store := &listTaskLogsErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/t1/logs/download", nil)
	req.SetPathValue("id", "t1")
	srv.handleDownloadTaskLogs(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleListJobLogs
// ---------------------------------------------------------------------------

func TestHandleListJobLogs_Success(t *testing.T) {
	store := &listJobLogsStore{
		stubStore: &stubStore{},
		logs: []*db.TaskLog{
			{ID: 1, JobID: "j1", TaskID: "t1", Stream: "stdout", Level: "info", Message: "log entry", LoggedAt: time.Now()},
		},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/j1/logs", nil)
	req.SetPathValue("id", "j1")
	srv.handleListJobLogs(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleListJobLogs_BadPageSize(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/j1/logs?page_size=-1", nil)
	req.SetPathValue("id", "j1")
	srv.handleListJobLogs(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleListJobLogs_BadCursor(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/j1/logs?cursor=notanint", nil)
	req.SetPathValue("id", "j1")
	srv.handleListJobLogs(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleListJobLogs_StoreError(t *testing.T) {
	store := &listJobLogsErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/j1/logs", nil)
	req.SetPathValue("id", "j1")
	srv.handleListJobLogs(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestHandleListJobLogs_StreamFilter(t *testing.T) {
	store := &listJobLogsParamsStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/j1/logs?stream=stderr", nil)
	req.SetPathValue("id", "j1")
	srv.handleListJobLogs(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if store.gotStream != "stderr" {
		t.Errorf("stream = %q, want %q", store.gotStream, "stderr")
	}
}

func TestHandleListJobLogs_NextCursorSet(t *testing.T) {
	// Fill exactly pageSize (100) logs so handler sets a next cursor.
	logs := make([]*db.TaskLog, 100)
	for i := range logs {
		logs[i] = &db.TaskLog{ID: int64(i + 1), TaskID: "t1", JobID: "j1", LoggedAt: time.Now()}
	}
	store := &listJobLogsStore{stubStore: &stubStore{}, logs: logs}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/j1/logs", nil)
	req.SetPathValue("id", "j1")
	srv.handleListJobLogs(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

// ---------------------------------------------------------------------------
// store stubs for logs tests
// ---------------------------------------------------------------------------

type listTaskLogsStore struct {
	*stubStore
	logs []*db.TaskLog
}

func (s *listTaskLogsStore) ListTaskLogs(_ context.Context, _ db.ListTaskLogsParams) ([]*db.TaskLog, error) {
	return s.logs, nil
}

type listTaskLogsErrStore struct{ *stubStore }

func (s *listTaskLogsErrStore) ListTaskLogs(_ context.Context, _ db.ListTaskLogsParams) ([]*db.TaskLog, error) {
	return nil, errors.New("db failure")
}

type listTaskLogsParamsStore struct {
	*stubStore
	gotStream string
	gotCursor int64
}

func (s *listTaskLogsParamsStore) ListTaskLogs(_ context.Context, p db.ListTaskLogsParams) ([]*db.TaskLog, error) {
	s.gotStream = p.Stream
	s.gotCursor = p.Cursor
	return []*db.TaskLog{}, nil
}

type listJobLogsStore struct {
	*stubStore
	logs []*db.TaskLog
}

func (s *listJobLogsStore) ListJobLogs(_ context.Context, _ db.ListJobLogsParams) ([]*db.TaskLog, error) {
	return s.logs, nil
}

type listJobLogsErrStore struct{ *stubStore }

func (s *listJobLogsErrStore) ListJobLogs(_ context.Context, _ db.ListJobLogsParams) ([]*db.TaskLog, error) {
	return nil, errors.New("db failure")
}

type listJobLogsParamsStore struct {
	*stubStore
	gotStream string
}

func (s *listJobLogsParamsStore) ListJobLogs(_ context.Context, p db.ListJobLogsParams) ([]*db.TaskLog, error) {
	s.gotStream = p.Stream
	return []*db.TaskLog{}, nil
}

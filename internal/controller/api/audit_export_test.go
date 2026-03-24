package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/db"
)

// ---------------------------------------------------------------------------
// TestHandleExportAuditLog
// ---------------------------------------------------------------------------

func TestHandleExportAuditLog(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	uid := "u1"
	sampleEntries := []*db.AuditEntry{
		{ID: 1, UserID: &uid, Username: "alice", Action: "user.create", Resource: "user", ResourceID: "u2", IPAddress: "10.0.0.1", LoggedAt: now},
		{ID: 2, Username: "system", Action: "agent.approve", Resource: "agent", ResourceID: "a1", IPAddress: "10.0.0.2", LoggedAt: now},
	}

	t.Run("json format", func(t *testing.T) {
		store := &exportAuditLogStore{
			stubStore: &stubStore{},
			entries:   sampleEntries,
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs/export?format=json", nil)
		srv.handleExportAuditLog(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		ct := rr.Header().Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		cd := rr.Header().Get("Content-Disposition")
		if !strings.Contains(cd, "audit-log.json") {
			t.Errorf("Content-Disposition = %q, want containing audit-log.json", cd)
		}
		var body []*db.AuditEntry
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decode JSON: %v", err)
		}
		if len(body) != 2 {
			t.Errorf("len = %d, want 2", len(body))
		}
	})

	t.Run("csv format", func(t *testing.T) {
		store := &exportAuditLogStore{
			stubStore: &stubStore{},
			entries:   sampleEntries,
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs/export?format=csv", nil)
		srv.handleExportAuditLog(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		ct := rr.Header().Get("Content-Type")
		if ct != "text/csv" {
			t.Errorf("Content-Type = %q, want text/csv", ct)
		}
		cd := rr.Header().Get("Content-Disposition")
		if !strings.Contains(cd, "audit-log.csv") {
			t.Errorf("Content-Disposition = %q, want containing audit-log.csv", cd)
		}
		body := rr.Body.String()
		// Header row + 2 data rows.
		lines := strings.Split(strings.TrimSpace(body), "\n")
		if len(lines) != 3 {
			t.Errorf("CSV lines = %d, want 3 (header + 2 rows)", len(lines))
		}
		// Verify header.
		if !strings.HasPrefix(lines[0], "id,") {
			t.Errorf("CSV header = %q, want to start with 'id,'", lines[0])
		}
	})

	t.Run("default format is json", func(t *testing.T) {
		store := &exportAuditLogStore{
			stubStore: &stubStore{},
			entries:   []*db.AuditEntry{},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs/export", nil)
		srv.handleExportAuditLog(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		ct := rr.Header().Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json (default)", ct)
		}
	})

	t.Run("custom limit", func(t *testing.T) {
		store := &exportAuditLogStore{
			stubStore: &stubStore{},
			entries:   []*db.AuditEntry{},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs/export?limit=500", nil)
		srv.handleExportAuditLog(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if store.calledLimit != 500 {
			t.Errorf("limit = %d, want 500", store.calledLimit)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &exportAuditLogErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs/export", nil)
		srv.handleExportAuditLog(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

type exportAuditLogStore struct {
	*stubStore
	entries     []*db.AuditEntry
	calledLimit int
}

func (s *exportAuditLogStore) ExportAuditLog(_ context.Context, limit int) ([]*db.AuditEntry, error) {
	s.calledLimit = limit
	return s.entries, nil
}

type exportAuditLogErrStore struct{ *stubStore }

func (s *exportAuditLogErrStore) ExportAuditLog(_ context.Context, _ int) ([]*db.AuditEntry, error) {
	return nil, errTestDB
}

// ---------------------------------------------------------------------------
// TestHandleGetUserActivity
// ---------------------------------------------------------------------------

func TestHandleGetUserActivity(t *testing.T) {
	now := time.Now()

	t.Run("success with defaults", func(t *testing.T) {
		store := &userActivityStore{
			stubStore: &stubStore{},
			entries: []*db.AuditEntry{
				{ID: 1, Username: "alice", Action: "job.create", Resource: "job", ResourceID: "j1", LoggedAt: now},
			},
			total: 1,
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/activity", nil)
		req.SetPathValue("id", "u1")
		srv.handleGetUserActivity(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data []map[string]any `json:"data"`
			Meta map[string]any   `json:"meta"`
		}
		decodeJSON(t, rr, &body)
		if len(body.Data) != 1 {
			t.Errorf("len(data) = %d, want 1", len(body.Data))
		}
		tc, ok := body.Meta["total_count"].(float64)
		if !ok || tc != 1 {
			t.Errorf("meta.total_count = %v, want 1", body.Meta["total_count"])
		}
	})

	t.Run("custom limit and offset", func(t *testing.T) {
		store := &userActivityStore{
			stubStore: &stubStore{},
			entries:   []*db.AuditEntry{},
			total:     0,
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/activity?limit=25&offset=5", nil)
		req.SetPathValue("id", "u1")
		srv.handleGetUserActivity(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if store.calledLimit != 25 {
			t.Errorf("limit = %d, want 25", store.calledLimit)
		}
		if store.calledOffset != 5 {
			t.Errorf("offset = %d, want 5", store.calledOffset)
		}
	})

	t.Run("limit above 500 uses default", func(t *testing.T) {
		store := &userActivityStore{
			stubStore: &stubStore{},
			entries:   []*db.AuditEntry{},
			total:     0,
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/activity?limit=1000", nil)
		req.SetPathValue("id", "u1")
		srv.handleGetUserActivity(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if store.calledLimit != 100 {
			t.Errorf("limit = %d, want 100 (clamped)", store.calledLimit)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &userActivityErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/activity", nil)
		req.SetPathValue("id", "u1")
		srv.handleGetUserActivity(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

type userActivityStore struct {
	*stubStore
	entries      []*db.AuditEntry
	total        int
	calledLimit  int
	calledOffset int
}

func (s *userActivityStore) ListAuditLogByUser(_ context.Context, _ string, limit, offset int) ([]*db.AuditEntry, int, error) {
	s.calledLimit = limit
	s.calledOffset = offset
	return s.entries, s.total, nil
}

type userActivityErrStore struct{ *stubStore }

func (s *userActivityErrStore) ListAuditLogByUser(_ context.Context, _ string, _, _ int) ([]*db.AuditEntry, int, error) {
	return nil, 0, errTestDB
}

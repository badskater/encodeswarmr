package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/db"
)

// ---------------------------------------------------------------------------
// TestHandleListAuditLog
// ---------------------------------------------------------------------------

func TestHandleListAuditLog(t *testing.T) {
	t.Run("success with defaults", func(t *testing.T) {
		now := time.Now()
		store := &listAuditLogStore{
			stubStore: &stubStore{},
			entries: []*db.AuditEntry{
				{ID: 1, Action: "user.create", Resource: "user", ResourceID: "u1", LoggedAt: now},
				{ID: 2, Action: "agent.approve", Resource: "agent", ResourceID: "a1", LoggedAt: now},
			},
			total: 2,
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-log", nil)
		srv.handleListAuditLog(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data []map[string]any `json:"data"`
			Meta map[string]any   `json:"meta"`
		}
		decodeJSON(t, rr, &body)
		if len(body.Data) != 2 {
			t.Errorf("len(data) = %d, want 2", len(body.Data))
		}
		tc, ok := body.Meta["total_count"].(float64)
		if !ok || tc != 2 {
			t.Errorf("meta.total_count = %v, want 2", body.Meta["total_count"])
		}
	})

	t.Run("custom limit and offset", func(t *testing.T) {
		store := &listAuditLogStore{
			stubStore: &stubStore{},
			entries:   []*db.AuditEntry{},
			total:     0,
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-log?limit=50&offset=10", nil)
		srv.handleListAuditLog(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		// Verify the store received the right parameters.
		if store.calledLimit != 50 {
			t.Errorf("limit = %d, want 50", store.calledLimit)
		}
		if store.calledOffset != 10 {
			t.Errorf("offset = %d, want 10", store.calledOffset)
		}
	})

	t.Run("limit above 500 is ignored — default applies", func(t *testing.T) {
		store := &listAuditLogStore{
			stubStore: &stubStore{},
			entries:   []*db.AuditEntry{},
			total:     0,
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-log?limit=1000", nil)
		srv.handleListAuditLog(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		// Value >500 is ignored; the default of 100 is kept.
		if store.calledLimit != 100 {
			t.Errorf("limit = %d, want 100 (clamped from 1000)", store.calledLimit)
		}
	})

	t.Run("invalid limit param uses default", func(t *testing.T) {
		store := &listAuditLogStore{
			stubStore: &stubStore{},
			entries:   []*db.AuditEntry{},
			total:     0,
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-log?limit=bad", nil)
		srv.handleListAuditLog(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if store.calledLimit != 100 {
			t.Errorf("limit = %d, want default 100 for invalid param", store.calledLimit)
		}
	})

	t.Run("invalid offset param uses default", func(t *testing.T) {
		store := &listAuditLogStore{
			stubStore: &stubStore{},
			entries:   []*db.AuditEntry{},
			total:     0,
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-log?offset=bad", nil)
		srv.handleListAuditLog(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if store.calledOffset != 0 {
			t.Errorf("offset = %d, want default 0 for invalid param", store.calledOffset)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &listAuditLogErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-log", nil)
		srv.handleListAuditLog(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

// ---------------------------------------------------------------------------
// store stubs
// ---------------------------------------------------------------------------

type listAuditLogStore struct {
	*stubStore
	entries      []*db.AuditEntry
	total        int
	calledLimit  int
	calledOffset int
}

func (s *listAuditLogStore) ListAuditLog(_ context.Context, limit, offset int) ([]*db.AuditEntry, int, error) {
	s.calledLimit = limit
	s.calledOffset = offset
	return s.entries, s.total, nil
}

type listAuditLogErrStore struct{ *stubStore }

func (s *listAuditLogErrStore) ListAuditLog(_ context.Context, _, _ int) ([]*db.AuditEntry, int, error) {
	return nil, 0, errTestDB
}

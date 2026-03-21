package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/badskater/encodeswarmr/internal/controller/ha"
	"github.com/badskater/encodeswarmr/internal/controller/webhooks"
	"github.com/badskater/encodeswarmr/internal/db"
)

// newHealthTestServer builds a Server with a non-nil *ha.Leader required by
// handleHealth and handleHAStatus. A zero-value Leader is safe for read-only
// operations (IsLeader/NodeID) since those methods only read atomics and a
// string — no pgxpool is accessed.
func newHealthTestServer(store db.Store) *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wh := webhooks.New(store, webhooks.Config{}, logger)
	// NewLeader requires a pool, so we use a nil pool — only safe because
	// Start() is never called, meaning the background loop never runs, and
	// the read-only methods (IsLeader/NodeID) never touch the pool.
	ldr := ha.NewLeader(nil, "test-node", logger)
	return &Server{
		store:    store,
		logger:   logger,
		webhooks: wh,
		leader:   ldr,
	}
}

// ---------------------------------------------------------------------------
// TestHandleHealth
// ---------------------------------------------------------------------------

func TestHandleHealth(t *testing.T) {
	t.Run("db ok returns 200", func(t *testing.T) {
		srv := newHealthTestServer(&pingOKStore{stubStore: &stubStore{}})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		srv.handleHealth(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data["status"] != "ok" {
			t.Errorf("data.status = %v, want ok", body.Data["status"])
		}
		if body.Data["db"] != "ok" {
			t.Errorf("data.db = %v, want ok", body.Data["db"])
		}
	})

	t.Run("db unreachable returns 503", func(t *testing.T) {
		srv := newHealthTestServer(&pingErrStore{stubStore: &stubStore{}})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		srv.handleHealth(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
		}
		var body struct {
			Data map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data["status"] != "degraded" {
			t.Errorf("data.status = %v, want degraded", body.Data["status"])
		}
		if body.Data["db"] != "unreachable" {
			t.Errorf("data.db = %v, want unreachable", body.Data["db"])
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleHAStatus
// ---------------------------------------------------------------------------

func TestHandleHAStatus(t *testing.T) {
	t.Run("returns leader and node_id", func(t *testing.T) {
		srv := newHealthTestServer(&pingOKStore{stubStore: &stubStore{}})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ha/status", nil)
		srv.handleHAStatus(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if _, ok := body.Data["leader"]; !ok {
			t.Error("expected 'leader' field in response")
		}
		if body.Data["node_id"] != "test-node" {
			t.Errorf("data.node_id = %v, want test-node", body.Data["node_id"])
		}
	})
}

// ---------------------------------------------------------------------------
// store stubs
// ---------------------------------------------------------------------------

type pingOKStore struct{ *stubStore }

func (s *pingOKStore) Ping(_ context.Context) error { return nil }

type pingErrStore struct{ *stubStore }

func (s *pingErrStore) Ping(_ context.Context) error { return errTestDB }

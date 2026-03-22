package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/db"
)

// errTestDB is a generic database error used across multiple test files.
var errTestDB = errors.New("db error")

// ---------------------------------------------------------------------------
// TestHandleListAgents
// ---------------------------------------------------------------------------

func TestHandleListAgents(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := &listAgentsStore{
			stubStore: &stubStore{},
			agents: []*db.Agent{
				{ID: "a1", Name: "agent-1", Status: "idle"},
				{ID: "a2", Name: "agent-2", Status: "running"},
			},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
		srv.handleListAgents(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data []map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if len(body.Data) != 2 {
			t.Errorf("len(data) = %d, want 2", len(body.Data))
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &listAgentsErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
		srv.handleListAgents(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

type listAgentsStore struct {
	*stubStore
	agents []*db.Agent
}

func (s *listAgentsStore) ListAgents(_ context.Context) ([]*db.Agent, error) {
	return s.agents, nil
}

type listAgentsErrStore struct{ *stubStore }

func (s *listAgentsErrStore) ListAgents(_ context.Context) ([]*db.Agent, error) {
	return nil, errTestDB
}

// ---------------------------------------------------------------------------
// TestHandleGetAgent
// ---------------------------------------------------------------------------

func TestHandleGetAgent(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := &getAgentStore{
			stubStore: &stubStore{},
			agent:     &db.Agent{ID: "a1", Name: "agent-1", Status: "idle"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/a1", nil)
		req.SetPathValue("id", "a1")
		srv.handleGetAgent(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data["id"] != "a1" {
			t.Errorf("data.id = %v, want a1", body.Data["id"])
		}
	})

	t.Run("not found", func(t *testing.T) {
		store := &getAgentNotFoundStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/missing", nil)
		req.SetPathValue("id", "missing")
		srv.handleGetAgent(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &getAgentErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/a1", nil)
		req.SetPathValue("id", "a1")
		srv.handleGetAgent(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

type getAgentStore struct {
	*stubStore
	agent *db.Agent
}

func (s *getAgentStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return s.agent, nil
}

type getAgentNotFoundStore struct{ *stubStore }

func (s *getAgentNotFoundStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return nil, db.ErrNotFound
}

type getAgentErrStore struct{ *stubStore }

func (s *getAgentErrStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return nil, errTestDB
}

// ---------------------------------------------------------------------------
// TestHandleDrainAgent
// ---------------------------------------------------------------------------

func TestHandleDrainAgent(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		store := &drainAgentNotFoundStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/missing/drain", nil)
		req.SetPathValue("id", "missing")
		srv.handleDrainAgent(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("get error returns 500", func(t *testing.T) {
		store := &drainAgentGetErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/a1/drain", nil)
		req.SetPathValue("id", "a1")
		srv.handleDrainAgent(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("update error returns 500", func(t *testing.T) {
		store := &drainAgentUpdateErrStore{
			stubStore: &stubStore{},
			agent:     &db.Agent{ID: "a1", Status: "idle"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/a1/drain", nil)
		req.SetPathValue("id", "a1")
		srv.handleDrainAgent(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("success", func(t *testing.T) {
		store := &drainAgentSuccessStore{
			stubStore: &stubStore{},
			agent:     &db.Agent{ID: "a1", Status: "idle"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/a1/drain", nil)
		req.SetPathValue("id", "a1")
		srv.handleDrainAgent(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data["ok"] != true {
			t.Error("expected data.ok to be true")
		}
	})
}

type drainAgentNotFoundStore struct{ *stubStore }

func (s *drainAgentNotFoundStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return nil, db.ErrNotFound
}

type drainAgentGetErrStore struct{ *stubStore }

func (s *drainAgentGetErrStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return nil, errTestDB
}

type drainAgentUpdateErrStore struct {
	*stubStore
	agent *db.Agent
}

func (s *drainAgentUpdateErrStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return s.agent, nil
}

func (s *drainAgentUpdateErrStore) UpdateAgentStatus(_ context.Context, _, _ string) error {
	return errTestDB
}

type drainAgentSuccessStore struct {
	*stubStore
	agent *db.Agent
}

func (s *drainAgentSuccessStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return s.agent, nil
}

func (s *drainAgentSuccessStore) UpdateAgentStatus(_ context.Context, _, _ string) error {
	return nil
}

// ---------------------------------------------------------------------------
// TestHandleApproveAgent
// ---------------------------------------------------------------------------

func TestHandleApproveAgent(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		store := &approveAgentNotFoundStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/missing/approve", nil)
		req.SetPathValue("id", "missing")
		srv.handleApproveAgent(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("get error returns 500", func(t *testing.T) {
		store := &approveAgentGetErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/a1/approve", nil)
		req.SetPathValue("id", "a1")
		srv.handleApproveAgent(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("not pending_approval returns 409", func(t *testing.T) {
		store := &approveAgentWrongStatusStore{
			stubStore: &stubStore{},
			agent:     &db.Agent{ID: "a1", Status: "idle"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/a1/approve", nil)
		req.SetPathValue("id", "a1")
		srv.handleApproveAgent(rr, req)

		if rr.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusConflict)
		}
	})

	t.Run("update error returns 500", func(t *testing.T) {
		store := &approveAgentUpdateErrStore{
			stubStore: &stubStore{},
			agent:     &db.Agent{ID: "a1", Status: "pending_approval"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/a1/approve", nil)
		req.SetPathValue("id", "a1")
		srv.handleApproveAgent(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("success", func(t *testing.T) {
		store := &approveAgentSuccessStore{
			stubStore: &stubStore{},
			agent:     &db.Agent{ID: "a1", Status: "pending_approval"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/a1/approve", nil)
		req.SetPathValue("id", "a1")
		srv.handleApproveAgent(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data["ok"] != true {
			t.Error("expected data.ok to be true")
		}
	})
}

type approveAgentNotFoundStore struct{ *stubStore }

func (s *approveAgentNotFoundStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return nil, db.ErrNotFound
}

type approveAgentGetErrStore struct{ *stubStore }

func (s *approveAgentGetErrStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return nil, errTestDB
}

type approveAgentWrongStatusStore struct {
	*stubStore
	agent *db.Agent
}

func (s *approveAgentWrongStatusStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return s.agent, nil
}

type approveAgentUpdateErrStore struct {
	*stubStore
	agent *db.Agent
}

func (s *approveAgentUpdateErrStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return s.agent, nil
}

func (s *approveAgentUpdateErrStore) UpdateAgentStatus(_ context.Context, _, _ string) error {
	return errTestDB
}

type approveAgentSuccessStore struct {
	*stubStore
	agent *db.Agent
}

func (s *approveAgentSuccessStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return s.agent, nil
}

func (s *approveAgentSuccessStore) UpdateAgentStatus(_ context.Context, _, _ string) error {
	return nil
}

// ---------------------------------------------------------------------------
// TestHandleGetAgentMetrics
// ---------------------------------------------------------------------------

func TestHandleGetAgentMetrics(t *testing.T) {
	t.Run("agent not found returns 404", func(t *testing.T) {
		store := &agentMetricsNotFoundStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/missing/metrics", nil)
		req.SetPathValue("id", "missing")
		srv.handleGetAgentMetrics(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("agent get error returns 500", func(t *testing.T) {
		store := &agentMetricsGetErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/a1/metrics", nil)
		req.SetPathValue("id", "a1")
		srv.handleGetAgentMetrics(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("metrics list error returns 500", func(t *testing.T) {
		store := &agentMetricsListErrStore{
			stubStore: &stubStore{},
			agent:     &db.Agent{ID: "a1"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/a1/metrics", nil)
		req.SetPathValue("id", "a1")
		srv.handleGetAgentMetrics(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("success with default window", func(t *testing.T) {
		store := &agentMetricsSuccessStore{
			stubStore: &stubStore{},
			agent:     &db.Agent{ID: "a1"},
			metrics: []*db.AgentMetric{
				{ID: 1, AgentID: "a1", CPUPct: 50.0},
			},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/a1/metrics", nil)
		req.SetPathValue("id", "a1")
		srv.handleGetAgentMetrics(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data []map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if len(body.Data) != 1 {
			t.Errorf("len(data) = %d, want 1", len(body.Data))
		}
	})

	t.Run("success with custom window", func(t *testing.T) {
		store := &agentMetricsSuccessStore{
			stubStore: &stubStore{},
			agent:     &db.Agent{ID: "a1"},
			metrics:   []*db.AgentMetric{},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/a1/metrics?window=30m", nil)
		req.SetPathValue("id", "a1")
		srv.handleGetAgentMetrics(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("invalid window param uses default", func(t *testing.T) {
		store := &agentMetricsSuccessStore{
			stubStore: &stubStore{},
			agent:     &db.Agent{ID: "a1"},
			metrics:   []*db.AgentMetric{},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/a1/metrics?window=bad", nil)
		req.SetPathValue("id", "a1")
		srv.handleGetAgentMetrics(rr, req)

		// Still succeeds; bad window is silently ignored, default is used.
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("nil metrics returns empty array", func(t *testing.T) {
		store := &agentMetricsNilStore{
			stubStore: &stubStore{},
			agent:     &db.Agent{ID: "a1"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/a1/metrics", nil)
		req.SetPathValue("id", "a1")
		srv.handleGetAgentMetrics(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data []map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data == nil {
			t.Error("expected non-nil empty data array")
		}
	})
}

type agentMetricsNotFoundStore struct{ *stubStore }

func (s *agentMetricsNotFoundStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return nil, db.ErrNotFound
}

type agentMetricsGetErrStore struct{ *stubStore }

func (s *agentMetricsGetErrStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return nil, errTestDB
}

type agentMetricsListErrStore struct {
	*stubStore
	agent *db.Agent
}

func (s *agentMetricsListErrStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return s.agent, nil
}

func (s *agentMetricsListErrStore) ListAgentMetrics(_ context.Context, _ string, _ time.Time) ([]*db.AgentMetric, error) {
	return nil, errTestDB
}

type agentMetricsSuccessStore struct {
	*stubStore
	agent   *db.Agent
	metrics []*db.AgentMetric
}

func (s *agentMetricsSuccessStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return s.agent, nil
}

func (s *agentMetricsSuccessStore) ListAgentMetrics(_ context.Context, _ string, _ time.Time) ([]*db.AgentMetric, error) {
	return s.metrics, nil
}

type agentMetricsNilStore struct {
	*stubStore
	agent *db.Agent
}

func (s *agentMetricsNilStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return s.agent, nil
}

func (s *agentMetricsNilStore) ListAgentMetrics(_ context.Context, _ string, _ time.Time) ([]*db.AgentMetric, error) {
	return nil, nil
}

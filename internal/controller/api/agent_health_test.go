package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/badskater/encodeswarmr/internal/db"
)

// ---------------------------------------------------------------------------
// TestHandleGetAgentHealth
// ---------------------------------------------------------------------------

func TestHandleGetAgentHealth(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := &agentHealthStore{
			stubStore: &stubStore{},
			agent:     &db.Agent{ID: "a1", Name: "worker-1", Status: "idle"},
			stats:     &db.AgentEncodingStats{AgentID: "a1", TotalTasks: 10, CompletedTasks: 8},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/a1/health", nil)
		req.SetPathValue("id", "a1")
		srv.handleGetAgentHealth(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data["agent"] == nil {
			t.Error("expected agent in response")
		}
		if body.Data["encoding_stats"] == nil {
			t.Error("expected encoding_stats in response")
		}
	})

	t.Run("agent not found returns 404", func(t *testing.T) {
		store := &agentHealthNotFoundStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/missing/health", nil)
		req.SetPathValue("id", "missing")
		srv.handleGetAgentHealth(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("agent get error returns 500", func(t *testing.T) {
		store := &agentHealthGetErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/a1/health", nil)
		req.SetPathValue("id", "a1")
		srv.handleGetAgentHealth(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("stats error returns 500", func(t *testing.T) {
		store := &agentHealthStatsErrStore{
			stubStore: &stubStore{},
			agent:     &db.Agent{ID: "a1"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/a1/health", nil)
		req.SetPathValue("id", "a1")
		srv.handleGetAgentHealth(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

type agentHealthStore struct {
	*stubStore
	agent *db.Agent
	stats *db.AgentEncodingStats
}

func (s *agentHealthStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return s.agent, nil
}

func (s *agentHealthStore) GetAgentEncodingStats(_ context.Context, _ string) (*db.AgentEncodingStats, error) {
	return s.stats, nil
}

type agentHealthNotFoundStore struct{ *stubStore }

func (s *agentHealthNotFoundStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return nil, db.ErrNotFound
}

type agentHealthGetErrStore struct{ *stubStore }

func (s *agentHealthGetErrStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return nil, errTestDB
}

type agentHealthStatsErrStore struct {
	*stubStore
	agent *db.Agent
}

func (s *agentHealthStatsErrStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return s.agent, nil
}

func (s *agentHealthStatsErrStore) GetAgentEncodingStats(_ context.Context, _ string) (*db.AgentEncodingStats, error) {
	return nil, errTestDB
}

// ---------------------------------------------------------------------------
// TestHandleListAgentRecentTasks
// ---------------------------------------------------------------------------

func TestHandleListAgentRecentTasks(t *testing.T) {
	t.Run("success with defaults", func(t *testing.T) {
		store := &recentTasksStore{
			stubStore: &stubStore{},
			agent:     &db.Agent{ID: "a1"},
			tasks: []*db.Task{
				{ID: "t1", JobID: "j1", Status: "completed"},
				{ID: "t2", JobID: "j2", Status: "failed"},
			},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/a1/recent-tasks", nil)
		req.SetPathValue("id", "a1")
		srv.handleListAgentRecentTasks(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data []map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if len(body.Data) != 2 {
			t.Errorf("len = %d, want 2", len(body.Data))
		}
		// Default limit should be 20.
		if store.calledLimit != 20 {
			t.Errorf("limit = %d, want 20", store.calledLimit)
		}
	})

	t.Run("custom limit", func(t *testing.T) {
		store := &recentTasksStore{
			stubStore: &stubStore{},
			agent:     &db.Agent{ID: "a1"},
			tasks:     []*db.Task{},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/a1/recent-tasks?limit=50", nil)
		req.SetPathValue("id", "a1")
		srv.handleListAgentRecentTasks(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if store.calledLimit != 50 {
			t.Errorf("limit = %d, want 50", store.calledLimit)
		}
	})

	t.Run("limit above 100 uses default", func(t *testing.T) {
		store := &recentTasksStore{
			stubStore: &stubStore{},
			agent:     &db.Agent{ID: "a1"},
			tasks:     []*db.Task{},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/a1/recent-tasks?limit=200", nil)
		req.SetPathValue("id", "a1")
		srv.handleListAgentRecentTasks(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if store.calledLimit != 20 {
			t.Errorf("limit = %d, want 20 (>100 ignored)", store.calledLimit)
		}
	})

	t.Run("invalid limit uses default", func(t *testing.T) {
		store := &recentTasksStore{
			stubStore: &stubStore{},
			agent:     &db.Agent{ID: "a1"},
			tasks:     []*db.Task{},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/a1/recent-tasks?limit=bad", nil)
		req.SetPathValue("id", "a1")
		srv.handleListAgentRecentTasks(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if store.calledLimit != 20 {
			t.Errorf("limit = %d, want 20 (invalid param)", store.calledLimit)
		}
	})

	t.Run("nil tasks returns empty array", func(t *testing.T) {
		store := &recentTasksNilStore{
			stubStore: &stubStore{},
			agent:     &db.Agent{ID: "a1"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/a1/recent-tasks", nil)
		req.SetPathValue("id", "a1")
		srv.handleListAgentRecentTasks(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data []map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data == nil {
			t.Error("expected non-nil empty array")
		}
	})

	t.Run("agent not found returns 404", func(t *testing.T) {
		store := &agentHealthNotFoundStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/missing/recent-tasks", nil)
		req.SetPathValue("id", "missing")
		srv.handleListAgentRecentTasks(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("agent get error returns 500", func(t *testing.T) {
		store := &agentHealthGetErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/a1/recent-tasks", nil)
		req.SetPathValue("id", "a1")
		srv.handleListAgentRecentTasks(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("tasks store error returns 500", func(t *testing.T) {
		store := &recentTasksErrStore{
			stubStore: &stubStore{},
			agent:     &db.Agent{ID: "a1"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/a1/recent-tasks", nil)
		req.SetPathValue("id", "a1")
		srv.handleListAgentRecentTasks(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

type recentTasksStore struct {
	*stubStore
	agent       *db.Agent
	tasks       []*db.Task
	calledLimit int
}

func (s *recentTasksStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return s.agent, nil
}

func (s *recentTasksStore) ListRecentTasksByAgent(_ context.Context, _ string, limit int) ([]*db.Task, error) {
	s.calledLimit = limit
	return s.tasks, nil
}

type recentTasksNilStore struct {
	*stubStore
	agent *db.Agent
}

func (s *recentTasksNilStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return s.agent, nil
}

func (s *recentTasksNilStore) ListRecentTasksByAgent(_ context.Context, _ string, _ int) ([]*db.Task, error) {
	return nil, nil
}

type recentTasksErrStore struct {
	*stubStore
	agent *db.Agent
}

func (s *recentTasksErrStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return s.agent, nil
}

func (s *recentTasksErrStore) ListRecentTasksByAgent(_ context.Context, _ string, _ int) ([]*db.Task, error) {
	return nil, errTestDB
}

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/badskater/distributed-encoder/internal/db"
)

// ---------------------------------------------------------------------------
// handleMetrics
// ---------------------------------------------------------------------------

func TestHandleMetrics_Success(t *testing.T) {
	store := &metricsStore{
		stubStore: &stubStore{},
		agents: []*db.Agent{
			{ID: "a1", Status: "idle"},
			{ID: "a2", Status: "running"},
			{ID: "a3", Status: "offline"},
		},
		total: 0,
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	srv.handleMetrics(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain prefix", ct)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "distencoder_jobs_total") {
		t.Error("response missing distencoder_jobs_total metric")
	}
	if !strings.Contains(body, "distencoder_agents_total") {
		t.Error("response missing distencoder_agents_total metric")
	}
}

func TestHandleMetrics_ListAgentsError(t *testing.T) {
	// When ListAgents errors, the handler returns early after writing partial
	// output.  The response code is still 200 because headers were already sent.
	store := &metricsAgentsErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	srv.handleMetrics(rr, req)

	// Response is 200 — headers were written before the agent query.
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleMetrics_AgentStatusCounts(t *testing.T) {
	store := &metricsAgentCountStore{
		stubStore: &stubStore{},
		agents: []*db.Agent{
			{ID: "a1", Status: "idle"},
			{ID: "a2", Status: "idle"},
			{ID: "a3", Status: "running"},
		},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	srv.handleMetrics(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	// idle=2, running=1 should be present in the Prometheus text.
	if !strings.Contains(body, `distencoder_agents_total{status="idle"} 2`) {
		t.Errorf("expected idle=2 in body:\n%s", body)
	}
	if !strings.Contains(body, `distencoder_agents_total{status="running"} 1`) {
		t.Errorf("expected running=1 in body:\n%s", body)
	}
}

// ---------------------------------------------------------------------------
// handleOpenAPISpec
// ---------------------------------------------------------------------------

func TestHandleOpenAPISpec_ReturnsJSON(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil)
	srv.handleOpenAPISpec(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	// The embedded spec must be non-empty.
	if rr.Body.Len() == 0 {
		t.Error("openapi response body is empty")
	}
}

// ---------------------------------------------------------------------------
// store stubs for metrics tests
// ---------------------------------------------------------------------------

type metricsStore struct {
	*stubStore
	agents []*db.Agent
	total  int64
}

func (s *metricsStore) ListJobs(_ context.Context, _ db.ListJobsFilter) ([]*db.Job, int64, error) {
	return nil, s.total, nil
}

func (s *metricsStore) ListAgents(_ context.Context) ([]*db.Agent, error) {
	return s.agents, nil
}

type metricsAgentsErrStore struct{ *stubStore }

func (s *metricsAgentsErrStore) ListAgents(_ context.Context) ([]*db.Agent, error) {
	return nil, &metricsTestError{"agents error"}
}

type metricsTestError struct{ msg string }

func (e *metricsTestError) Error() string { return e.msg }

type metricsAgentCountStore struct {
	*stubStore
	agents []*db.Agent
}

func (s *metricsAgentCountStore) ListJobs(_ context.Context, _ db.ListJobsFilter) ([]*db.Job, int64, error) {
	return nil, 0, nil
}

func (s *metricsAgentCountStore) ListAgents(_ context.Context) ([]*db.Agent, error) {
	return s.agents, nil
}


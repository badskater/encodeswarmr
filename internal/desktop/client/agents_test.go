package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// ListAgents
// ---------------------------------------------------------------------------

func TestListAgents_ReturnsList(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	agents := []Agent{
		{ID: "ag-1", Name: "enc-01", Status: AgentIdle, CreatedAt: now},
		{ID: "ag-2", Name: "enc-02", Status: AgentOffline, CreatedAt: now},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agents" {
			t.Errorf("path = %q, want /api/v1/agents", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, agents, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.ListAgents(context.Background())
	if err != nil {
		t.Fatalf("ListAgents() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(agents) = %d, want 2", len(got))
	}
	if got[0].ID != "ag-1" {
		t.Errorf("agents[0].ID = %q, want ag-1", got[0].ID)
	}
	if got[1].Status != AgentOffline {
		t.Errorf("agents[1].Status = %q, want offline", got[1].Status)
	}
}

func TestListAgents_ErrorResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write(problemResponse(t, http.StatusUnauthorized, "Unauthorized", "session expired"))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.ListAgents(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got != nil {
		t.Errorf("expected nil slice on error, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// GetAgent
// ---------------------------------------------------------------------------

func TestGetAgent_ReturnsAgent(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	ag := Agent{ID: "ag-get", Name: "enc-03", Status: AgentRunning, CPUCount: 32, RAMMIB: 65536, CreatedAt: now}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agents/ag-get" {
			t.Errorf("path = %q, want /api/v1/agents/ag-get", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, ag, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.GetAgent(context.Background(), "ag-get")
	if err != nil {
		t.Fatalf("GetAgent() error = %v", err)
	}
	if got.ID != "ag-get" {
		t.Errorf("ID = %q, want ag-get", got.ID)
	}
	if got.CPUCount != 32 {
		t.Errorf("CPUCount = %d, want 32", got.CPUCount)
	}
}

// ---------------------------------------------------------------------------
// DrainAgent / ApproveAgent
// ---------------------------------------------------------------------------

func TestDrainAgent_UsesCorrectEndpoint(t *testing.T) {
	t.Parallel()
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL)
	if err := c.DrainAgent(context.Background(), "ag-drain"); err != nil {
		t.Fatalf("DrainAgent() error = %v", err)
	}
	if gotPath != "/api/v1/agents/ag-drain/drain" {
		t.Errorf("path = %q, want /api/v1/agents/ag-drain/drain", gotPath)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
}

func TestApproveAgent_UsesCorrectEndpoint(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL)
	if err := c.ApproveAgent(context.Background(), "ag-pending"); err != nil {
		t.Fatalf("ApproveAgent() error = %v", err)
	}
	if gotPath != "/api/v1/agents/ag-pending/approve" {
		t.Errorf("path = %q, want /api/v1/agents/ag-pending/approve", gotPath)
	}
}

// ---------------------------------------------------------------------------
// ListAgentMetrics
// ---------------------------------------------------------------------------

func TestListAgentMetrics_PassesWindowParam(t *testing.T) {
	t.Parallel()
	var gotQuery string
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	metrics := []AgentMetric{
		{ID: 1, AgentID: "ag-1", CPUPct: 45.0, GPUPct: 80.0, MemPct: 60.0, RecordedAt: now},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, metrics, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.ListAgentMetrics(context.Background(), "ag-1", "6h")
	if err != nil {
		t.Fatalf("ListAgentMetrics() error = %v", err)
	}
	if !strings.Contains(gotQuery, "window=6h") {
		t.Errorf("query %q missing window=6h", gotQuery)
	}
	if len(got) != 1 {
		t.Fatalf("len(metrics) = %d, want 1", len(got))
	}
	if got[0].CPUPct != 45.0 {
		t.Errorf("CPUPct = %v, want 45.0", got[0].CPUPct)
	}
}

// ---------------------------------------------------------------------------
// GetAgentHealth
// ---------------------------------------------------------------------------

func TestGetAgentHealth_ReturnsHealthResponse(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	health := AgentHealthResponse{
		Agent: Agent{ID: "ag-health", Name: "enc-04", Status: AgentRunning, CreatedAt: now},
		EncodingStats: AgentEncodingStats{
			AgentID:        "ag-health",
			TotalTasks:     100,
			CompletedTasks: 95,
			FailedTasks:    5,
			AvgFPS:         24.0,
			TotalFrames:    2400000,
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agents/ag-health/health" {
			t.Errorf("path = %q, want /api/v1/agents/ag-health/health", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, health, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.GetAgentHealth(context.Background(), "ag-health")
	if err != nil {
		t.Fatalf("GetAgentHealth() error = %v", err)
	}
	if got.Agent.ID != "ag-health" {
		t.Errorf("Agent.ID = %q, want ag-health", got.Agent.ID)
	}
	if got.EncodingStats.AvgFPS != 24.0 {
		t.Errorf("EncodingStats.AvgFPS = %v, want 24.0", got.EncodingStats.AvgFPS)
	}
	if got.EncodingStats.FailedTasks != 5 {
		t.Errorf("FailedTasks = %d, want 5", got.EncodingStats.FailedTasks)
	}
}

// ---------------------------------------------------------------------------
// UpdateAgentChannel
// ---------------------------------------------------------------------------

func TestUpdateAgentChannel_SendsBody(t *testing.T) {
	t.Parallel()
	var gotBody map[string]string
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want PUT", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL)
	if err := c.UpdateAgentChannel(context.Background(), "ag-ch", "beta"); err != nil {
		t.Fatalf("UpdateAgentChannel() error = %v", err)
	}
	if gotPath != "/api/v1/agents/ag-ch/channel" {
		t.Errorf("path = %q, want /api/v1/agents/ag-ch/channel", gotPath)
	}
	if gotBody["channel"] != "beta" {
		t.Errorf("body channel = %q, want beta", gotBody["channel"])
	}
}

// ---------------------------------------------------------------------------
// AssignAgentToPool / RemoveAgentFromPool
// ---------------------------------------------------------------------------

func TestAssignAgentToPool_SendsPoolID(t *testing.T) {
	t.Parallel()
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL)
	if err := c.AssignAgentToPool(context.Background(), "ag-1", "pool-gpu"); err != nil {
		t.Fatalf("AssignAgentToPool() error = %v", err)
	}
	if gotBody["pool_id"] != "pool-gpu" {
		t.Errorf("body pool_id = %q, want pool-gpu", gotBody["pool_id"])
	}
}

func TestRemoveAgentFromPool_UsesDeleteMethod(t *testing.T) {
	t.Parallel()
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL)
	if err := c.RemoveAgentFromPool(context.Background(), "ag-2", "pool-1"); err != nil {
		t.Fatalf("RemoveAgentFromPool() error = %v", err)
	}
	if gotPath != "/api/v1/agents/ag-2/pools/pool-1" {
		t.Errorf("path = %q, want /api/v1/agents/ag-2/pools/pool-1", gotPath)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
}

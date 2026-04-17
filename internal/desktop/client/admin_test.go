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
// AuditLogExportURL — pure URL builder
// ---------------------------------------------------------------------------

func TestAuditLogExportURL_Format(t *testing.T) {
	t.Parallel()
	c := New("http://controller:8080")
	got := c.AuditLogExportURL("csv", 500)
	if !strings.HasPrefix(got, "http://controller:8080/api/v1/audit-logs/export?") {
		t.Errorf("AuditLogExportURL = %q, unexpected prefix", got)
	}
	if !strings.Contains(got, "format=csv") {
		t.Errorf("AuditLogExportURL = %q, want format=csv", got)
	}
	if !strings.Contains(got, "limit=500") {
		t.Errorf("AuditLogExportURL = %q, want limit=500", got)
	}
}

func TestAuditLogExportURL_RespectsBaseURL(t *testing.T) {
	t.Parallel()
	c := New("https://prod.example.com/")
	got := c.AuditLogExportURL("json", 100)
	if !strings.HasPrefix(got, "https://prod.example.com/api/v1/audit-logs/export") {
		t.Errorf("AuditLogExportURL = %q, wrong base URL", got)
	}
}

// ---------------------------------------------------------------------------
// TogglePlugin
// ---------------------------------------------------------------------------

func TestTogglePlugin_Enable_UsesEnablePath(t *testing.T) {
	t.Parallel()
	var gotPath string
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	_ = now
	plugin := Plugin{ID: "p-1", Name: "x265", Enabled: true}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want PUT", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, plugin, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.TogglePlugin(context.Background(), "x265", true)
	if err != nil {
		t.Fatalf("TogglePlugin() error = %v", err)
	}
	if gotPath != "/api/v1/plugins/x265/enable" {
		t.Errorf("path = %q, want /api/v1/plugins/x265/enable", gotPath)
	}
	if !got.Enabled {
		t.Errorf("Enabled = false, want true")
	}
}

func TestTogglePlugin_Disable_UsesDisablePath(t *testing.T) {
	t.Parallel()
	var gotPath string
	plugin := Plugin{ID: "p-2", Name: "x265", Enabled: false}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, plugin, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.TogglePlugin(context.Background(), "x265", false)
	if err != nil {
		t.Fatalf("TogglePlugin() error = %v", err)
	}
	if gotPath != "/api/v1/plugins/x265/disable" {
		t.Errorf("path = %q, want /api/v1/plugins/x265/disable", gotPath)
	}
}

// ---------------------------------------------------------------------------
// GetAutoScaling / UpdateAutoScaling
// ---------------------------------------------------------------------------

func TestGetAutoScaling_ReturnsSettings(t *testing.T) {
	t.Parallel()
	settings := AutoScalingSettings{
		Enabled:            true,
		WebhookURL:         "https://hooks.example.com/scale",
		ScaleUpThreshold:   0.8,
		ScaleDownThreshold: 0.2,
		CooldownSeconds:    300,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/settings/auto-scaling" {
			t.Errorf("path = %q, want /api/v1/settings/auto-scaling", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, settings, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.GetAutoScaling(context.Background())
	if err != nil {
		t.Fatalf("GetAutoScaling() error = %v", err)
	}
	if !got.Enabled {
		t.Error("Enabled = false, want true")
	}
	if got.ScaleUpThreshold != 0.8 {
		t.Errorf("ScaleUpThreshold = %v, want 0.8", got.ScaleUpThreshold)
	}
	if got.CooldownSeconds != 300 {
		t.Errorf("CooldownSeconds = %d, want 300", got.CooldownSeconds)
	}
}

func TestUpdateAutoScaling_SendsBodyAndReturnsUpdated(t *testing.T) {
	t.Parallel()
	var gotBody map[string]any
	updated := AutoScalingSettings{Enabled: false, WebhookURL: "", ScaleUpThreshold: 0.9}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want PUT", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, updated, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	body := map[string]any{"enabled": false, "scale_up_threshold": 0.9}
	got, err := c.UpdateAutoScaling(context.Background(), body)
	if err != nil {
		t.Fatalf("UpdateAutoScaling() error = %v", err)
	}
	if got.Enabled {
		t.Error("Enabled = true, want false")
	}
}

// ---------------------------------------------------------------------------
// ListEnrollmentTokens / CreateEnrollmentToken
// ---------------------------------------------------------------------------

func TestListEnrollmentTokens_ReturnsList(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	tokens := []EnrollmentToken{
		{ID: "tok-1", Token: "abc123", CreatedBy: "admin", CreatedAt: now},
		{ID: "tok-2", Token: "def456", CreatedBy: "admin", CreatedAt: now},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent-tokens" {
			t.Errorf("path = %q, want /api/v1/agent-tokens", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, tokens, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.ListEnrollmentTokens(context.Background())
	if err != nil {
		t.Fatalf("ListEnrollmentTokens() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(tokens) = %d, want 2", len(got))
	}
}

func TestCreateEnrollmentToken_NoExpiry(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	token := EnrollmentToken{ID: "tok-new", Token: "xyz789", CreatedBy: "admin", CreatedAt: now}
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(envelopeResponse(t, token, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.CreateEnrollmentToken(context.Background(), "")
	if err != nil {
		t.Fatalf("CreateEnrollmentToken() error = %v", err)
	}
	if got.Token != "xyz789" {
		t.Errorf("Token = %q, want xyz789", got.Token)
	}
	// Empty expiresAt must not produce an expires_at key in the body.
	if _, ok := gotBody["expires_at"]; ok {
		t.Error("body should not include expires_at when empty")
	}
}

func TestCreateEnrollmentToken_WithExpiry(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	token := EnrollmentToken{ID: "tok-exp", Token: "exp999", CreatedBy: "admin", CreatedAt: now}
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(envelopeResponse(t, token, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	expiry := "2024-12-31T23:59:59Z"
	_, err := c.CreateEnrollmentToken(context.Background(), expiry)
	if err != nil {
		t.Fatalf("CreateEnrollmentToken() error = %v", err)
	}
	if gotBody["expires_at"] != expiry {
		t.Errorf("body expires_at = %q, want %q", gotBody["expires_at"], expiry)
	}
}

// ---------------------------------------------------------------------------
// ListWebhooks / TestWebhook
// ---------------------------------------------------------------------------

func TestListWebhooks_ReturnsList(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	hooks := []Webhook{
		{ID: "wh-1", Name: "Discord", Provider: "discord", URL: "https://discord.com/api/webhooks/...", Enabled: true, CreatedAt: now},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/webhooks" {
			t.Errorf("path = %q, want /api/v1/webhooks", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, hooks, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.ListWebhooks(context.Background())
	if err != nil {
		t.Fatalf("ListWebhooks() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(webhooks) = %d, want 1", len(got))
	}
	if got[0].Provider != "discord" {
		t.Errorf("Provider = %q, want discord", got[0].Provider)
	}
}

func TestTestWebhook_UsesCorrectPath(t *testing.T) {
	t.Parallel()
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL)
	if err := c.TestWebhook(context.Background(), "wh-99"); err != nil {
		t.Fatalf("TestWebhook() error = %v", err)
	}
	if gotPath != "/api/v1/webhooks/wh-99/test" {
		t.Errorf("path = %q, want /api/v1/webhooks/wh-99/test", gotPath)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
}

// ---------------------------------------------------------------------------
// ListAuditLog
// ---------------------------------------------------------------------------

func TestListAuditLog_PassesPaginationParams(t *testing.T) {
	t.Parallel()
	var gotQuery string
	meta := map[string]any{"total_count": float64(200), "request_id": "r1"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, []AuditEntry{}, meta))
	}))
	defer srv.Close()

	c := New(srv.URL)
	col, err := c.ListAuditLog(context.Background(), 50, 100)
	if err != nil {
		t.Fatalf("ListAuditLog() error = %v", err)
	}
	if !strings.Contains(gotQuery, "limit=50") {
		t.Errorf("query %q missing limit=50", gotQuery)
	}
	if !strings.Contains(gotQuery, "offset=100") {
		t.Errorf("query %q missing offset=100", gotQuery)
	}
	if col.TotalCount != 200 {
		t.Errorf("TotalCount = %d, want 200", col.TotalCount)
	}
}

// ---------------------------------------------------------------------------
// CreateAgentPool
// ---------------------------------------------------------------------------

func TestCreateAgentPool_SendsBodyAndReturnsPool(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	pool := AgentPool{
		ID:          "pool-1",
		Name:        "GPU Cluster",
		Description: "All GPU-capable agents",
		Tags:        []string{"gpu", "nvenc"},
		Color:       "#3B82F6",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/agent-pools" {
			t.Errorf("path = %q, want /api/v1/agent-pools", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(envelopeResponse(t, pool, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.CreateAgentPool(context.Background(), "GPU Cluster", "All GPU-capable agents", "#3B82F6", []string{"gpu", "nvenc"})
	if err != nil {
		t.Fatalf("CreateAgentPool() error = %v", err)
	}
	if got.Name != "GPU Cluster" {
		t.Errorf("Name = %q, want GPU Cluster", got.Name)
	}
	if gotBody["color"] != "#3B82F6" {
		t.Errorf("body color = %v, want #3B82F6", gotBody["color"])
	}
}

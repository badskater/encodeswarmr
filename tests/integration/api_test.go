//go:build integration

// Package integration_test contains Layer 2 integration tests that exercise
// the full HTTP API and gRPC surface of the distributed-encoder controller.
//
// Requirements:
//   - A reachable PostgreSQL instance (TEST_DATABASE_URL or a testcontainers-
//     based throwaway Postgres 16 container started automatically).
//   - Run with: go test -tags=integration ./tests/integration/... -v
package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
	"time"

	"github.com/badskater/distributed-encoder/tests/integration/testharness"
)

// cookieJar creates a new RFC 6265-compliant cookie jar.
func cookieJar() (http.CookieJar, error) {
	return cookiejar.New(nil)
}

// --------------------------------------------------------------------------
// helpers
// --------------------------------------------------------------------------

// mustDo issues req and fails the test on any transport error.
func mustDo(t *testing.T, client *http.Client, req *http.Request) *http.Response {
	t.Helper()
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("http %s %s: %v", req.Method, req.URL, err)
	}
	return resp
}

// jsonBody encodes v as JSON and returns a ReadCloser suitable for http.Request.Body.
func jsonBody(t *testing.T, v any) io.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	return bytes.NewReader(b)
}

// decodeJSON reads resp.Body into dst.
func decodeJSON(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("decode json: %v", err)
	}
}

// drainClose discards the body and closes it.
func drainClose(resp *http.Response) {
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

// adminLogin performs a login with the standard test admin credentials and
// returns an authenticated http.Client whose cookie jar carries the session.
func adminLogin(t *testing.T, tc *testharness.TestController) *http.Client {
	t.Helper()

	jar := newJar(t)
	client := &http.Client{Jar: jar}

	body := jsonBody(t, map[string]string{
		"username": "admin",
		"password": "testpassword1",
	})
	req, _ := http.NewRequest(http.MethodPost, tc.HTTPBaseURL+"/auth/login", body)
	req.Header.Set("Content-Type", "application/json")

	resp := mustDo(t, client, req)
	drainClose(resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("adminLogin: expected 200, got %d", resp.StatusCode)
	}
	return client
}

// newJar creates a cookie jar and fails the test on error.
func newJar(t *testing.T) http.CookieJar {
	t.Helper()
	jar, err := cookieJar()
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	return jar
}

// --------------------------------------------------------------------------
// TestSetupWizard
// --------------------------------------------------------------------------

// TestSetupWizard verifies the setup wizard endpoint before and after
// completing initial setup.
func TestSetupWizard(t *testing.T) {
	tc := testharness.StartController(t)

	client := &http.Client{}

	// GET /setup/status before any admin exists → not_done.
	req, _ := http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/setup/status", nil)
	resp := mustDo(t, client, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("setup status: expected 200, got %d", resp.StatusCode)
	}
	var statusResp map[string]any
	decodeJSON(t, resp, &statusResp)
	// Response is wrapped: {"data": {"required": true}, "meta": {...}}
	data, _ := statusResp["data"].(map[string]any)
	if data["required"] != true {
		t.Errorf("setup status before setup: want required=true, got %v", data["required"])
	}

	// POST /setup with admin credentials → 201 Created.
	setupBody := jsonBody(t, map[string]string{
		"username": "admin",
		"email":    "admin@test.local",
		"password": "testpassword1",
	})
	req, _ = http.NewRequest(http.MethodPost, tc.HTTPBaseURL+"/setup", setupBody)
	req.Header.Set("Content-Type", "application/json")
	resp = mustDo(t, client, req)
	drainClose(resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("setup: expected 201, got %d", resp.StatusCode)
	}

	// POST /setup again → 409 or 4xx (already done).
	setupBody2 := jsonBody(t, map[string]string{
		"username": "admin2",
		"email":    "admin2@test.local",
		"password": "testpassword1",
	})
	req, _ = http.NewRequest(http.MethodPost, tc.HTTPBaseURL+"/setup", setupBody2)
	req.Header.Set("Content-Type", "application/json")
	resp = mustDo(t, client, req)
	drainClose(resp)
	if resp.StatusCode < 400 {
		t.Errorf("second setup: expected 4xx, got %d", resp.StatusCode)
	}
}

// --------------------------------------------------------------------------
// TestAuthFlow
// --------------------------------------------------------------------------

// TestAuthFlow exercises login, /me, and logout.
func TestAuthFlow(t *testing.T) {
	tc := testharness.StartController(t)

	// Create admin via setup wizard first.
	client := &http.Client{}
	setupBody := jsonBody(t, map[string]string{
		"username": "admin",
		"email":    "admin@test.local",
		"password": "testpassword1",
	})
	req, _ := http.NewRequest(http.MethodPost, tc.HTTPBaseURL+"/setup", setupBody)
	req.Header.Set("Content-Type", "application/json")
	resp := mustDo(t, client, req)
	drainClose(resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("setup: %d", resp.StatusCode)
	}

	// Login → 200 + session cookie.
	authedClient := adminLogin(t, tc)

	// GET /api/v1/users/me with session cookie → 200 + user info.
	req, _ = http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/users/me", nil)
	resp = mustDo(t, authedClient, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /me: expected 200, got %d", resp.StatusCode)
	}
	var me map[string]any
	decodeJSON(t, resp, &me)
	// Response is wrapped: {"data": {"username": "...", ...}, "meta": {...}}
	meData, _ := me["data"].(map[string]any)
	if meData["username"] != "admin" {
		t.Errorf("GET /me: username want admin, got %v", meData["username"])
	}

	// POST /auth/logout → 200.
	req, _ = http.NewRequest(http.MethodPost, tc.HTTPBaseURL+"/auth/logout", nil)
	resp = mustDo(t, authedClient, req)
	drainClose(resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("logout: expected 200, got %d", resp.StatusCode)
	}

	// GET /api/v1/users/me without valid cookie → 401.
	req, _ = http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/users/me", nil)
	resp = mustDo(t, &http.Client{}, req)
	drainClose(resp)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("GET /me unauthenticated: expected 401, got %d", resp.StatusCode)
	}
}

// --------------------------------------------------------------------------
// TestHealthEndpoint
// --------------------------------------------------------------------------

// TestHealthEndpoint verifies the unauthenticated health endpoint.
func TestHealthEndpoint(t *testing.T) {
	tc := testharness.StartController(t)

	resp, err := http.Get(tc.HTTPBaseURL + "/health") //nolint:gosec,noctx
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	drainClose(resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /health: expected 200, got %d", resp.StatusCode)
	}
}

// --------------------------------------------------------------------------
// TestSourcesCRUD
// --------------------------------------------------------------------------

// TestSourcesCRUD exercises create, list, get, and delete for sources.
func TestSourcesCRUD(t *testing.T) {
	tc := testharness.StartController(t)

	// Setup wizard + login.
	setupAndLogin(t, tc)
	authed := adminLogin(t, tc)

	// POST /api/v1/sources → 201.
	body := jsonBody(t, map[string]any{
		"filename":   "test_movie.mkv",
		"path":       `\\nas01\media\test_movie.mkv`,
		"size_bytes": 1073741824,
	})
	req, _ := http.NewRequest(http.MethodPost, tc.HTTPBaseURL+"/api/v1/sources", body)
	req.Header.Set("Content-Type", "application/json")
	resp := mustDo(t, authed, req)
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("create source: expected 201, got %d: %s", resp.StatusCode, bodyBytes)
	}
	var src map[string]any
	decodeJSON(t, resp, &src)
	// Response is wrapped: {"data": {"id": "...", ...}, "meta": {...}}
	srcData, _ := src["data"].(map[string]any)
	srcID, _ := srcData["id"].(string)
	if srcID == "" {
		t.Fatal("create source: no id returned")
	}

	// GET /api/v1/sources → list contains the new source.
	req, _ = http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/sources", nil)
	resp = mustDo(t, authed, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list sources: expected 200, got %d", resp.StatusCode)
	}
	var list map[string]any
	decodeJSON(t, resp, &list)
	// writeCollection wraps as {"data": [...], "meta": {...}}
	items, _ := list["data"].([]any)
	if len(items) == 0 {
		t.Error("list sources: expected at least 1 item")
	}

	// GET /api/v1/sources/{id} → 200.
	req, _ = http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/sources/"+srcID, nil)
	resp = mustDo(t, authed, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get source: expected 200, got %d", resp.StatusCode)
	}
	drainClose(resp)

	// Delete any jobs that were auto-created for this source (analysis, hdr_detect)
	// so that the source FK constraint (ON DELETE RESTRICT) does not block deletion.
	// There is no DeleteJob API endpoint, so we use the pool directly.
	cleanupCtx := context.Background()
	if _, err := tc.Pool.Exec(cleanupCtx, "DELETE FROM tasks WHERE job_id IN (SELECT id FROM jobs WHERE source_id = $1)", srcID); err != nil {
		t.Logf("delete source tasks: %v", err)
	}
	if _, err := tc.Pool.Exec(cleanupCtx, "DELETE FROM jobs WHERE source_id = $1", srcID); err != nil {
		t.Fatalf("delete source jobs: %v", err)
	}

	// DELETE /api/v1/sources/{id} → 204 No Content.
	req, _ = http.NewRequest(http.MethodDelete, tc.HTTPBaseURL+"/api/v1/sources/"+srcID, nil)
	resp = mustDo(t, authed, req)
	drainClose(resp)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete source: expected 204, got %d", resp.StatusCode)
	}
}

// --------------------------------------------------------------------------
// TestJobLifecycle
// --------------------------------------------------------------------------

// TestJobLifecycle creates a source, creates a job via the encode endpoint,
// retrieves the job, and then cancels it.
func TestJobLifecycle(t *testing.T) {
	tc := testharness.StartController(t)

	setupAndLogin(t, tc)
	authed := adminLogin(t, tc)

	// Create a source first.
	srcBody := jsonBody(t, map[string]any{
		"filename":   "lifecycle.mkv",
		"path":       `\\nas01\media\lifecycle.mkv`,
		"size_bytes": 512000000,
	})
	req, _ := http.NewRequest(http.MethodPost, tc.HTTPBaseURL+"/api/v1/sources", srcBody)
	req.Header.Set("Content-Type", "application/json")
	resp := mustDo(t, authed, req)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create source: %d", resp.StatusCode)
	}
	var src map[string]any
	decodeJSON(t, resp, &src)
	// Response is wrapped: {"data": {"id": "...", ...}, "meta": {...}}
	srcData2, _ := src["data"].(map[string]any)
	srcID, _ := srcData2["id"].(string)

	// POST /api/v1/sources/{id}/encode → 201.
	encBody := jsonBody(t, map[string]any{
		"job_type":    "encode",
		"priority":    5,
		"target_tags": []string{},
		"encode_config": map[string]any{
			"run_script_template_id": "",
			"output_root":            `\\nas01\output`,
			"output_extension":       "mkv",
			"chunk_boundaries": []map[string]any{
				{"start_frame": 0, "end_frame": 999},
			},
		},
	})
	req, _ = http.NewRequest(http.MethodPost, tc.HTTPBaseURL+"/api/v1/sources/"+srcID+"/encode", encBody)
	req.Header.Set("Content-Type", "application/json")
	resp = mustDo(t, authed, req)
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("create job: expected 201, got %d: %s", resp.StatusCode, bodyBytes)
	}
	var job map[string]any
	decodeJSON(t, resp, &job)
	// Response is wrapped: {"data": {"id": "...", ...}, "meta": {...}}
	jobData, _ := job["data"].(map[string]any)
	jobID, _ := jobData["id"].(string)
	if jobID == "" {
		t.Fatal("create job: no id returned")
	}

	// GET /api/v1/jobs/{id} → 200.
	req, _ = http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/jobs/"+jobID, nil)
	resp = mustDo(t, authed, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get job: expected 200, got %d", resp.StatusCode)
	}
	drainClose(resp)

	// POST /api/v1/jobs/{id}/cancel → 200.
	req, _ = http.NewRequest(http.MethodPost, tc.HTTPBaseURL+"/api/v1/jobs/"+jobID+"/cancel", nil)
	resp = mustDo(t, authed, req)
	drainClose(resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("cancel job: expected 200, got %d", resp.StatusCode)
	}
}

// --------------------------------------------------------------------------
// TestTemplatesCRUD
// --------------------------------------------------------------------------

// TestTemplatesCRUD exercises the full CRUD lifecycle for encoding templates
// (admin-only).
func TestTemplatesCRUD(t *testing.T) {
	tc := testharness.StartController(t)

	setupAndLogin(t, tc)
	authed := adminLogin(t, tc)

	// POST /api/v1/templates → 201.
	body := jsonBody(t, map[string]any{
		"name":        fmt.Sprintf("tmpl-%d", time.Now().UnixNano()),
		"description": "integration test template",
		"type":        "run_script",
		"extension":   "bat",
		"content":     "@echo off\necho hello",
	})
	req, _ := http.NewRequest(http.MethodPost, tc.HTTPBaseURL+"/api/v1/templates", body)
	req.Header.Set("Content-Type", "application/json")
	resp := mustDo(t, authed, req)
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("create template: expected 201, got %d: %s", resp.StatusCode, bodyBytes)
	}
	var tmpl map[string]any
	decodeJSON(t, resp, &tmpl)
	tmplID, _ := tmpl["id"].(string)

	// GET /api/v1/templates → list.
	req, _ = http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/templates", nil)
	resp = mustDo(t, authed, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list templates: expected 200, got %d", resp.StatusCode)
	}
	drainClose(resp)

	// GET /api/v1/templates/{id} → 200.
	req, _ = http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/templates/"+tmplID, nil)
	resp = mustDo(t, authed, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get template: expected 200, got %d", resp.StatusCode)
	}
	drainClose(resp)

	// PUT /api/v1/templates/{id} → 200.
	updateBody := jsonBody(t, map[string]any{
		"name":    "updated-template",
		"content": "@echo off\necho updated",
	})
	req, _ = http.NewRequest(http.MethodPut, tc.HTTPBaseURL+"/api/v1/templates/"+tmplID, updateBody)
	req.Header.Set("Content-Type", "application/json")
	resp = mustDo(t, authed, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update template: expected 200, got %d", resp.StatusCode)
	}
	drainClose(resp)

	// DELETE /api/v1/templates/{id} → 200.
	req, _ = http.NewRequest(http.MethodDelete, tc.HTTPBaseURL+"/api/v1/templates/"+tmplID, nil)
	resp = mustDo(t, authed, req)
	drainClose(resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("delete template: expected 200, got %d", resp.StatusCode)
	}
}

// --------------------------------------------------------------------------
// TestVariablesCRUD
// --------------------------------------------------------------------------

// TestVariablesCRUD exercises upsert, get, list, and delete for global variables.
func TestVariablesCRUD(t *testing.T) {
	tc := testharness.StartController(t)

	setupAndLogin(t, tc)
	authed := adminLogin(t, tc)

	varName := fmt.Sprintf("TEST_VAR_%d", time.Now().UnixNano())

	// PUT /api/v1/variables/{name} (upsert) → 200.
	body := jsonBody(t, map[string]any{
		"value":       "test-value",
		"description": "integration test variable",
		"category":    "test",
	})
	req, _ := http.NewRequest(http.MethodPut, tc.HTTPBaseURL+"/api/v1/variables/"+varName, body)
	req.Header.Set("Content-Type", "application/json")
	resp := mustDo(t, authed, req)
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("upsert variable: expected 200, got %d: %s", resp.StatusCode, bodyBytes)
	}
	var variable map[string]any
	decodeJSON(t, resp, &variable)
	varID, _ := variable["id"].(string)

	// GET /api/v1/variables/{name} → 200.
	req, _ = http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/variables/"+varName, nil)
	resp = mustDo(t, authed, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get variable: expected 200, got %d", resp.StatusCode)
	}
	drainClose(resp)

	// GET /api/v1/variables → list.
	req, _ = http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/variables", nil)
	resp = mustDo(t, authed, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list variables: expected 200, got %d", resp.StatusCode)
	}
	drainClose(resp)

	// DELETE /api/v1/variables/{id} → 200.
	if varID != "" {
		req, _ = http.NewRequest(http.MethodDelete, tc.HTTPBaseURL+"/api/v1/variables/"+varID, nil)
		resp = mustDo(t, authed, req)
		drainClose(resp)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("delete variable: expected 200, got %d", resp.StatusCode)
		}
	}
}

// --------------------------------------------------------------------------
// TestWebhooksCRUD
// --------------------------------------------------------------------------

// TestWebhooksCRUD exercises the full CRUD lifecycle for webhooks.
func TestWebhooksCRUD(t *testing.T) {
	tc := testharness.StartController(t)

	setupAndLogin(t, tc)
	authed := adminLogin(t, tc)

	// POST /api/v1/webhooks → 201.
	body := jsonBody(t, map[string]any{
		"name":     "test-webhook",
		"provider": "discord",
		"url":      "https://discord.com/api/webhooks/test",
		"events":   []string{"job.completed"},
	})
	req, _ := http.NewRequest(http.MethodPost, tc.HTTPBaseURL+"/api/v1/webhooks", body)
	req.Header.Set("Content-Type", "application/json")
	resp := mustDo(t, authed, req)
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("create webhook: expected 201, got %d: %s", resp.StatusCode, bodyBytes)
	}
	var wh map[string]any
	decodeJSON(t, resp, &wh)
	whID, _ := wh["id"].(string)

	// GET /api/v1/webhooks → list.
	req, _ = http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/webhooks", nil)
	resp = mustDo(t, authed, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list webhooks: expected 200, got %d", resp.StatusCode)
	}
	drainClose(resp)

	// GET /api/v1/webhooks/{id} → 200.
	req, _ = http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/webhooks/"+whID, nil)
	resp = mustDo(t, authed, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get webhook: expected 200, got %d", resp.StatusCode)
	}
	drainClose(resp)

	// PUT /api/v1/webhooks/{id} → 200.
	updateBody := jsonBody(t, map[string]any{
		"name":    "updated-webhook",
		"url":     "https://discord.com/api/webhooks/updated",
		"events":  []string{"job.completed", "job.failed"},
		"enabled": true,
	})
	req, _ = http.NewRequest(http.MethodPut, tc.HTTPBaseURL+"/api/v1/webhooks/"+whID, updateBody)
	req.Header.Set("Content-Type", "application/json")
	resp = mustDo(t, authed, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update webhook: expected 200, got %d", resp.StatusCode)
	}
	drainClose(resp)

	// DELETE /api/v1/webhooks/{id} → 200.
	req, _ = http.NewRequest(http.MethodDelete, tc.HTTPBaseURL+"/api/v1/webhooks/"+whID, nil)
	resp = mustDo(t, authed, req)
	drainClose(resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("delete webhook: expected 200, got %d", resp.StatusCode)
	}
}

// --------------------------------------------------------------------------
// TestUserManagement
// --------------------------------------------------------------------------

// TestUserManagement exercises user create, role update, and delete.
func TestUserManagement(t *testing.T) {
	tc := testharness.StartController(t)

	setupAndLogin(t, tc)
	authed := adminLogin(t, tc)

	// POST /api/v1/users → 201.
	newUsername := fmt.Sprintf("operator-%d", time.Now().UnixNano())
	body := jsonBody(t, map[string]any{
		"username": newUsername,
		"email":    newUsername + "@test.local",
		"role":     "operator",
		"password": "operatorpass1",
	})
	req, _ := http.NewRequest(http.MethodPost, tc.HTTPBaseURL+"/api/v1/users", body)
	req.Header.Set("Content-Type", "application/json")
	resp := mustDo(t, authed, req)
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("create user: expected 201, got %d: %s", resp.StatusCode, bodyBytes)
	}
	var user map[string]any
	decodeJSON(t, resp, &user)
	userID, _ := user["id"].(string)

	// PUT /api/v1/users/{id}/role → 200.
	roleBody := jsonBody(t, map[string]any{"role": "viewer"})
	req, _ = http.NewRequest(http.MethodPut, tc.HTTPBaseURL+"/api/v1/users/"+userID+"/role", roleBody)
	req.Header.Set("Content-Type", "application/json")
	resp = mustDo(t, authed, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update role: expected 200, got %d", resp.StatusCode)
	}
	drainClose(resp)

	// DELETE /api/v1/users/{id} → 200.
	req, _ = http.NewRequest(http.MethodDelete, tc.HTTPBaseURL+"/api/v1/users/"+userID, nil)
	resp = mustDo(t, authed, req)
	drainClose(resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("delete user: expected 200, got %d", resp.StatusCode)
	}
}

// --------------------------------------------------------------------------
// TestPathMappingsCRUD
// --------------------------------------------------------------------------

// TestPathMappingsCRUD exercises the path-mappings CRUD endpoints.
func TestPathMappingsCRUD(t *testing.T) {
	tc := testharness.StartController(t)

	setupAndLogin(t, tc)
	authed := adminLogin(t, tc)

	// POST /api/v1/path-mappings → 201.
	body := jsonBody(t, map[string]any{
		"name":           "test-mapping",
		"windows_prefix": `\\NAS01\media`,
		"linux_prefix":   "/mnt/nas/media",
	})
	req, _ := http.NewRequest(http.MethodPost, tc.HTTPBaseURL+"/api/v1/path-mappings", body)
	req.Header.Set("Content-Type", "application/json")
	resp := mustDo(t, authed, req)
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("create path-mapping: expected 201, got %d: %s", resp.StatusCode, bodyBytes)
	}
	var pm map[string]any
	decodeJSON(t, resp, &pm)
	pmID, _ := pm["id"].(string)

	// GET /api/v1/path-mappings → list.
	req, _ = http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/path-mappings", nil)
	resp = mustDo(t, authed, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list path-mappings: expected 200, got %d", resp.StatusCode)
	}
	drainClose(resp)

	// GET /api/v1/path-mappings/{id} → 200.
	req, _ = http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/path-mappings/"+pmID, nil)
	resp = mustDo(t, authed, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get path-mapping: expected 200, got %d", resp.StatusCode)
	}
	drainClose(resp)

	// PUT /api/v1/path-mappings/{id} → 200.
	updateBody := jsonBody(t, map[string]any{
		"name":           "updated-mapping",
		"windows_prefix": `\\NAS01\media`,
		"linux_prefix":   "/mnt/nas/media",
		"enabled":        true,
	})
	req, _ = http.NewRequest(http.MethodPut, tc.HTTPBaseURL+"/api/v1/path-mappings/"+pmID, updateBody)
	req.Header.Set("Content-Type", "application/json")
	resp = mustDo(t, authed, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update path-mapping: expected 200, got %d", resp.StatusCode)
	}
	drainClose(resp)

	// DELETE /api/v1/path-mappings/{id} → 200.
	req, _ = http.NewRequest(http.MethodDelete, tc.HTTPBaseURL+"/api/v1/path-mappings/"+pmID, nil)
	resp = mustDo(t, authed, req)
	drainClose(resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("delete path-mapping: expected 200, got %d", resp.StatusCode)
	}
}

// --------------------------------------------------------------------------
// TestSchedulesCRUD
// --------------------------------------------------------------------------

// TestSchedulesCRUD exercises the schedules CRUD endpoints.
func TestSchedulesCRUD(t *testing.T) {
	tc := testharness.StartController(t)

	setupAndLogin(t, tc)
	authed := adminLogin(t, tc)

	// POST /api/v1/schedules → 201.
	body := jsonBody(t, map[string]any{
		"name":      "nightly-encode",
		"cron_expr": "0 2 * * *",
		"enabled":   true,
		"job_template": map[string]any{
			"source_id": "placeholder",
			"job_type":  "encode",
		},
	})
	req, _ := http.NewRequest(http.MethodPost, tc.HTTPBaseURL+"/api/v1/schedules", body)
	req.Header.Set("Content-Type", "application/json")
	resp := mustDo(t, authed, req)
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("create schedule: expected 201, got %d: %s", resp.StatusCode, bodyBytes)
	}
	var sched map[string]any
	decodeJSON(t, resp, &sched)
	schedID, _ := sched["id"].(string)

	// GET /api/v1/schedules → list.
	req, _ = http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/schedules", nil)
	resp = mustDo(t, authed, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list schedules: expected 200, got %d", resp.StatusCode)
	}
	drainClose(resp)

	// GET /api/v1/schedules/{id} → 200.
	req, _ = http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/schedules/"+schedID, nil)
	resp = mustDo(t, authed, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get schedule: expected 200, got %d", resp.StatusCode)
	}
	drainClose(resp)

	// PUT /api/v1/schedules/{id} → 200.
	updateBody := jsonBody(t, map[string]any{
		"name":      "nightly-encode-updated",
		"cron_expr": "0 3 * * *",
		"enabled":   false,
		"job_template": map[string]any{
			"source_id": "placeholder",
			"job_type":  "encode",
		},
	})
	req, _ = http.NewRequest(http.MethodPut, tc.HTTPBaseURL+"/api/v1/schedules/"+schedID, updateBody)
	req.Header.Set("Content-Type", "application/json")
	resp = mustDo(t, authed, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update schedule: expected 200, got %d", resp.StatusCode)
	}
	drainClose(resp)

	// DELETE /api/v1/schedules/{id} → 200.
	req, _ = http.NewRequest(http.MethodDelete, tc.HTTPBaseURL+"/api/v1/schedules/"+schedID, nil)
	resp = mustDo(t, authed, req)
	drainClose(resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("delete schedule: expected 200, got %d", resp.StatusCode)
	}
}

// --------------------------------------------------------------------------
// shared test helpers
// --------------------------------------------------------------------------

// setupAndLogin runs the setup wizard to create the admin user. It is safe to
// call multiple times within the same TestController (subsequent calls are
// no-ops because the wizard will return 4xx).
func setupAndLogin(t *testing.T, tc *testharness.TestController) {
	t.Helper()
	client := &http.Client{}
	body := strings.NewReader(`{"username":"admin","email":"admin@test.local","password":"testpassword1"}`)
	req, _ := http.NewRequest(http.MethodPost, tc.HTTPBaseURL+"/setup", body)
	req.Header.Set("Content-Type", "application/json")
	resp := mustDo(t, client, req)
	drainClose(resp)
	// 200 = newly set up, 4xx = already done — both are fine here.
}

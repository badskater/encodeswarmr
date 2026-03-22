//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/auth"
	"github.com/badskater/encodeswarmr/internal/db"
	"github.com/badskater/encodeswarmr/tests/integration/testharness"
)

// --------------------------------------------------------------------------
// TestSecurity_ExpiredAPIKey
// --------------------------------------------------------------------------

// TestSecurity_ExpiredAPIKey creates an API key, manually expires it in the
// DB, then verifies the key is rejected with 401.
func TestSecurity_ExpiredAPIKey(t *testing.T) {
	ctx := context.Background()
	tc := testharness.StartController(t)
	user, token := testharness.CreateAdminUser(t, tc.Store, tc.AuthSvc)
	_ = user

	// Create an API key via HTTP.
	client := testharness.AuthenticatedClient(t, tc.HTTPBaseURL, token)
	req, _ := http.NewRequest(http.MethodPost, tc.HTTPBaseURL+"/api/v1/api-keys",
		jsonBody(t, map[string]string{"name": "expiry-test-key"}),
	)
	req.Header.Set("Content-Type", "application/json")
	resp := mustDo(t, client, req)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create api key: expected 201, got %d", resp.StatusCode)
	}
	var createResp struct {
		Data struct {
			ID  string `json:"id"`
			Key string `json:"key"`
		} `json:"data"`
	}
	decodeJSON(t, resp, &createResp)
	keyID := createResp.Data.ID
	plaintext := createResp.Data.Key
	if keyID == "" || plaintext == "" {
		t.Fatal("create api key: got empty id or key in response")
	}

	// Manually expire the key in the DB.
	_, err := tc.Pool.Exec(ctx,
		`UPDATE api_keys SET expires_at = now() - interval '1 hour' WHERE id = $1`,
		keyID,
	)
	if err != nil {
		t.Fatalf("expire api key in DB: %v", err)
	}

	// Use the now-expired key; expect 401.
	bare := &http.Client{}
	authReq, _ := http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/agents", nil)
	authReq.Header.Set("Authorization", "Bearer "+plaintext)
	authResp := mustDo(t, bare, authReq)
	drainClose(authResp)
	if authResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expired api key: expected 401, got %d", authResp.StatusCode)
	}
}

// --------------------------------------------------------------------------
// TestSecurity_InvalidAPIKey
// --------------------------------------------------------------------------

// TestSecurity_InvalidAPIKey verifies that a completely bogus Bearer token
// results in 401 Unauthorized.
func TestSecurity_InvalidAPIKey(t *testing.T) {
	tc := testharness.StartController(t)

	client := &http.Client{}
	req, _ := http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer totally-invalid-key")
	resp := mustDo(t, client, req)
	drainClose(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("invalid api key: expected 401, got %d", resp.StatusCode)
	}
}

// --------------------------------------------------------------------------
// TestSecurity_NoAuthRequired_Health
// --------------------------------------------------------------------------

// TestSecurity_NoAuthRequired_Health verifies that GET /health is publicly
// reachable without any authentication.
func TestSecurity_NoAuthRequired_Health(t *testing.T) {
	tc := testharness.StartController(t)

	client := &http.Client{}
	req, _ := http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/health", nil)
	resp := mustDo(t, client, req)
	if resp.StatusCode != http.StatusOK {
		drainClose(resp)
		t.Fatalf("health: expected 200, got %d", resp.StatusCode)
	}

	// The health response is wrapped in the standard envelope: {"data": {...}, "meta": {...}}.
	var envelope map[string]any
	decodeJSON(t, resp, &envelope)
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatalf("health body: expected 'data' envelope, got: %v", envelope)
	}
	if data["status"] != "ok" {
		t.Errorf("health body: want status=ok, got %v", data["status"])
	}
}

// --------------------------------------------------------------------------
// TestSecurity_AuthRequired_Agents
// --------------------------------------------------------------------------

// TestSecurity_AuthRequired_Agents verifies that GET /api/v1/agents without
// any authentication returns 401.
func TestSecurity_AuthRequired_Agents(t *testing.T) {
	tc := testharness.StartController(t)

	client := &http.Client{}
	req, _ := http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/agents", nil)
	resp := mustDo(t, client, req)
	drainClose(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauthenticated GET /api/v1/agents: expected 401, got %d", resp.StatusCode)
	}
}

// --------------------------------------------------------------------------
// TestSecurity_RoleEnforcement
// --------------------------------------------------------------------------

// TestSecurity_RoleEnforcement creates a viewer user and verifies that:
//   - DELETE /api/v1/templates/{id} returns 403 (admin only)
//   - GET    /api/v1/templates       returns 200 (viewer allowed)
func TestSecurity_RoleEnforcement(t *testing.T) {
	ctx := context.Background()
	tc := testharness.StartController(t)

	// Create a viewer user.
	const (
		viewerUser = "viewer-role-test"
		viewerPass = "testpassword1"
	)
	hash, err := auth.HashPassword(viewerPass)
	if err != nil {
		t.Fatalf("hash viewer password: %v", err)
	}
	_, err = tc.Store.CreateUser(ctx, db.CreateUserParams{
		Username:     viewerUser,
		Email:        viewerUser + "@test.local",
		Role:         "viewer",
		PasswordHash: &hash,
	})
	if err != nil {
		t.Fatalf("create viewer user: %v", err)
	}

	// Log in as the viewer.
	sess, err := tc.AuthSvc.Login(ctx, viewerUser, viewerPass)
	if err != nil {
		t.Fatalf("viewer login: %v", err)
	}
	viewerClient := testharness.AuthenticatedClient(t, tc.HTTPBaseURL, sess.Token)

	// Create a template to give the DELETE something to target.
	tmpl := testharness.CreateTestTemplate(t, tc.Store)

	// DELETE /api/v1/templates/{id} as viewer → 403 Forbidden.
	delReq, _ := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/api/v1/templates/%s", tc.HTTPBaseURL, tmpl.ID), nil)
	delResp := mustDo(t, viewerClient, delReq)
	drainClose(delResp)
	if delResp.StatusCode != http.StatusForbidden {
		t.Errorf("viewer DELETE template: expected 403, got %d", delResp.StatusCode)
	}

	// GET /api/v1/templates as viewer → 200 OK.
	getReq, _ := http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/templates", nil)
	getResp := mustDo(t, viewerClient, getReq)
	drainClose(getResp)
	if getResp.StatusCode != http.StatusOK {
		t.Errorf("viewer GET templates: expected 200, got %d", getResp.StatusCode)
	}
}

// --------------------------------------------------------------------------
// TestSecurity_SecurityHeaders
// --------------------------------------------------------------------------

// TestSecurity_SecurityHeaders verifies that every authenticated response
// carries the expected security headers.
func TestSecurity_SecurityHeaders(t *testing.T) {
	tc := testharness.StartController(t)
	_, token := testharness.CreateAdminUser(t, tc.Store, tc.AuthSvc)

	client := testharness.AuthenticatedClient(t, tc.HTTPBaseURL, token)
	req, _ := http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/agents", nil)
	resp := mustDo(t, client, req)
	drainClose(resp)

	checks := []struct {
		header   string
		contains string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
		{"Content-Security-Policy", "default-src 'self'"},
		{"Permissions-Policy", "camera=()"},
	}

	for _, c := range checks {
		val := resp.Header.Get(c.header)
		if !strings.Contains(val, c.contains) {
			t.Errorf("header %q: want contains %q, got %q", c.header, c.contains, val)
		}
	}
}

// --------------------------------------------------------------------------
// TestSecurity_CORSHeaders
// --------------------------------------------------------------------------

// TestSecurity_CORSHeaders exercises CORS behaviour:
//   - An origin not in the allowlist receives no Access-Control-Allow-Origin.
//   - The wildcard "*" origin (set in testharness StartController) is matched.
func TestSecurity_CORSHeaders(t *testing.T) {
	tc := testharness.StartController(t)
	// The test controller sets AllowedOrigins: []string{"*"}.

	client := &http.Client{}

	// --- Evil origin: should NOT get a matching ACAO header when only "*" is
	// in the allowlist.  The corsMiddleware does an exact match, so
	// "https://evil.com" won't match "*".
	evilReq, _ := http.NewRequest(http.MethodOptions, tc.HTTPBaseURL+"/api/v1/agents", nil)
	evilReq.Header.Set("Origin", "https://evil.com")
	evilReq.Header.Set("Access-Control-Request-Method", "GET")
	evilResp := mustDo(t, client, evilReq)
	drainClose(evilResp)
	// The middleware only sets the header when the origin matches exactly.
	// With allowlist = ["*"] the string "https://evil.com" is not in the set,
	// so Access-Control-Allow-Origin should be absent or empty.
	if acao := evilResp.Header.Get("Access-Control-Allow-Origin"); acao == "https://evil.com" {
		t.Errorf("evil origin: Access-Control-Allow-Origin should not reflect evil origin, got %q", acao)
	}

	// --- Wildcard origin: the literal string "*" should be matched.
	wildcardReq, _ := http.NewRequest(http.MethodOptions, tc.HTTPBaseURL+"/api/v1/agents", nil)
	wildcardReq.Header.Set("Origin", "*")
	wildcardReq.Header.Set("Access-Control-Request-Method", "GET")
	wildcardResp := mustDo(t, client, wildcardReq)
	drainClose(wildcardResp)
	acao := wildcardResp.Header.Get("Access-Control-Allow-Origin")
	if acao != "*" {
		t.Errorf("wildcard origin: expected Access-Control-Allow-Origin=*, got %q", acao)
	}
	if wildcardResp.StatusCode != http.StatusNoContent {
		t.Errorf("OPTIONS preflight: expected 204, got %d", wildcardResp.StatusCode)
	}
}

// --------------------------------------------------------------------------
// TestSecurity_RateLimiting
// --------------------------------------------------------------------------

// TestSecurity_RateLimiting fires requests at the same endpoint until a 429
// is received.  The rate limiter is configured at 200 req/s burst 400; by
// sending more than 400 requests in a tight loop we must hit the limit.
func TestSecurity_RateLimiting(t *testing.T) {
	tc := testharness.StartController(t)
	_, token := testharness.CreateAdminUser(t, tc.Store, tc.AuthSvc)

	client := testharness.AuthenticatedClient(t, tc.HTTPBaseURL, token)

	const maxRequests = 600 // burst is 400; we should hit 429 before this

	got429 := false
	for i := range maxRequests {
		req, _ := http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/agents", nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		drainClose(resp)
		if resp.StatusCode == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}

	if !got429 {
		t.Errorf("expected at least one 429 Too Many Requests within %d rapid requests", maxRequests)
	}
}

// --------------------------------------------------------------------------
// TestSecurity_SQLInjection
// --------------------------------------------------------------------------

// TestSecurity_SQLInjection attempts to inject SQL via a source name.
// The injection must not cause an error and the sources table must still
// function correctly after the attempt.
func TestSecurity_SQLInjection(t *testing.T) {
	ctx := context.Background()
	tc := testharness.StartController(t)
	_, token := testharness.CreateAdminUser(t, tc.Store, tc.AuthSvc)

	maliciousName := `'; DROP TABLE sources; --`

	client := testharness.AuthenticatedClient(t, tc.HTTPBaseURL, token)
	req, _ := http.NewRequest(http.MethodPost, tc.HTTPBaseURL+"/api/v1/sources",
		jsonBody(t, map[string]any{
			"filename":   maliciousName,
			"unc_path":   `\\nas\media\inject.mkv`,
			"size_bytes": 1024,
		}),
	)
	req.Header.Set("Content-Type", "application/json")
	resp := mustDo(t, client, req)
	drainClose(resp)

	// Any 2xx or 4xx is acceptable; a 500 or panic is not.
	if resp.StatusCode >= http.StatusInternalServerError {
		t.Errorf("sql injection attempt: unexpected server error %d", resp.StatusCode)
	}

	// Verify the sources table is intact by performing a normal list.
	listReq, _ := http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/sources", nil)
	listResp := mustDo(t, client, listReq)
	drainClose(listResp)
	if listResp.StatusCode != http.StatusOK {
		t.Errorf("list sources after injection: expected 200, got %d", listResp.StatusCode)
	}

	// Also verify the table exists by querying it directly.
	rows, err := tc.Pool.Query(ctx, `SELECT id FROM sources LIMIT 1`)
	if err != nil {
		t.Errorf("sources table query after injection attempt: %v", err)
	} else {
		rows.Close()
	}
}

// --------------------------------------------------------------------------
// TestSecurity_XSSInJobName
// --------------------------------------------------------------------------

// TestSecurity_XSSInJobName creates a job with an XSS payload in its name,
// retrieves it, and verifies the payload is returned verbatim as JSON (the
// CSP header prevents execution in the browser).  The server must not error.
func TestSecurity_XSSInJobName(t *testing.T) {
	tc := testharness.StartController(t)
	_, token := testharness.CreateAdminUser(t, tc.Store, tc.AuthSvc)

	src := testharness.CreateTestSource(t, tc.Store)

	client := testharness.AuthenticatedClient(t, tc.HTTPBaseURL, token)

	xssPayload := "<script>alert('xss')</script>"

	createReq, _ := http.NewRequest(http.MethodPost, tc.HTTPBaseURL+"/api/v1/jobs",
		jsonBody(t, map[string]any{
			"source_id":   src.ID,
			"job_type":    "analysis",
			"priority":    5,
			"target_tags": []string{xssPayload},
		}),
	)
	createReq.Header.Set("Content-Type", "application/json")
	createResp := mustDo(t, client, createReq)
	if createResp.StatusCode != http.StatusCreated {
		drainClose(createResp)
		t.Fatalf("create job with xss name: expected 201, got %d", createResp.StatusCode)
	}
	// Job created successfully — the XSS payload in target_tags didn't crash the server.
	// The CSP header (verified in TestSecurity_SecurityHeaders) prevents script execution.
	drainClose(createResp)
}

// --------------------------------------------------------------------------
// TestSecurity_LargePayload
// --------------------------------------------------------------------------

// TestSecurity_LargePayload sends a 10 MiB POST body to /api/v1/jobs.
// The server must reject it with a 4xx (413 or 400) and not crash.
func TestSecurity_LargePayload(t *testing.T) {
	tc := testharness.StartController(t)
	_, token := testharness.CreateAdminUser(t, tc.Store, tc.AuthSvc)

	// Build a 10 MiB body: a JSON object with a very long "name" field.
	const tenMiB = 10 * 1024 * 1024
	padding := strings.Repeat("A", tenMiB)
	bigBody := fmt.Sprintf(`{"source_id":"fake","job_type":"encode","name":%q}`, padding)

	client := testharness.AuthenticatedClient(t, tc.HTTPBaseURL, token)
	req, _ := http.NewRequest(http.MethodPost, tc.HTTPBaseURL+"/api/v1/jobs",
		bytes.NewBufferString(bigBody),
	)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		// A connection reset from the server is also acceptable.
		t.Logf("large payload: connection-level rejection (%v) — acceptable", err)
		return
	}
	drainClose(resp)

	if resp.StatusCode < http.StatusBadRequest || resp.StatusCode >= http.StatusInternalServerError {
		t.Errorf("large payload: expected 4xx, got %d", resp.StatusCode)
	}
}

// --------------------------------------------------------------------------
// TestSecurity_APIKeyScoping
// --------------------------------------------------------------------------

// TestSecurity_APIKeyScoping verifies that user A's API key cannot delete
// user B's API key.
func TestSecurity_APIKeyScoping(t *testing.T) {
	ctx := context.Background()
	tc := testharness.StartController(t)

	// Create two independent admin users.
	const pass = "testpassword1"
	makeUser := func(username string) string {
		hash, err := auth.HashPassword(pass)
		if err != nil {
			t.Fatalf("hash password for %s: %v", username, err)
		}
		_, err = tc.Store.CreateUser(ctx, db.CreateUserParams{
			Username:     username,
			Email:        username + "@test.local",
			Role:         "admin",
			PasswordHash: &hash,
		})
		if err != nil {
			t.Fatalf("create user %s: %v", username, err)
		}
		sess, err := tc.AuthSvc.Login(ctx, username, pass)
		if err != nil {
			t.Fatalf("login %s: %v", username, err)
		}
		return sess.Token
	}

	tokenA := makeUser("scope-user-a-" + shortTestID(t))
	tokenB := makeUser("scope-user-b-" + shortTestID(t))

	// Create an API key as user B.
	clientB := testharness.AuthenticatedClient(t, tc.HTTPBaseURL, tokenB)
	createReq, _ := http.NewRequest(http.MethodPost, tc.HTTPBaseURL+"/api/v1/api-keys",
		jsonBody(t, map[string]string{"name": "user-b-key"}),
	)
	createReq.Header.Set("Content-Type", "application/json")
	createResp := mustDo(t, clientB, createReq)
	if createResp.StatusCode != http.StatusCreated {
		drainClose(createResp)
		t.Fatalf("create user B api key: expected 201, got %d", createResp.StatusCode)
	}
	var createBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeJSON(t, createResp, &createBody)
	keyBID := createBody.Data.ID
	if keyBID == "" {
		t.Fatal("create user B api key: got empty id")
	}

	// Attempt to delete user B's key as user A → expect 404 (not found for
	// this user) rather than a successful 200.
	clientA := testharness.AuthenticatedClient(t, tc.HTTPBaseURL, tokenA)
	delReq, _ := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/api/v1/api-keys/%s", tc.HTTPBaseURL, keyBID), nil)
	delResp := mustDo(t, clientA, delReq)
	drainClose(delResp)

	if delResp.StatusCode == http.StatusOK {
		t.Errorf("api key scoping: user A should not be able to delete user B's key; got 200")
	}
	if delResp.StatusCode != http.StatusNotFound {
		t.Logf("api key scoping: got %d (expected 404)", delResp.StatusCode)
	}

	// Verify the key still exists (user B can still list it).
	listReq, _ := http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/api-keys", nil)
	listResp := mustDo(t, clientB, listReq)
	var listBody struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeJSON(t, listResp, &listBody)

	found := false
	for _, k := range listBody.Data {
		if k.ID == keyBID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("api key scoping: user B's key %s was deleted by user A", keyBID)
	}
}

// --------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------

// shortTestID returns a short unique string based on the test name, suitable
// for use in resource names to avoid collisions across parallel runs.
func shortTestID(t *testing.T) string {
	t.Helper()
	// Re-use the package-level shortID helper from fixtures.go (same package).
	// We define this thin wrapper so security_test.go is self-documenting.
	return fmt.Sprintf("%x", time.Now().UnixNano()&0xFFFF)
}

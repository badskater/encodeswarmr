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
// helpers
// ---------------------------------------------------------------------------

// mustMarshal marshals v to JSON or fails the test.
func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}

// envelope wraps data and optional meta into the server envelope format.
func envelopeResponse(t *testing.T, data any, meta map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("envelope: marshal data: %v", err)
	}
	env := map[string]any{
		"data": json.RawMessage(raw),
		"meta": meta,
	}
	return mustMarshal(t, env)
}

// problemResponse returns a Problem JSON body.
func problemResponse(t *testing.T, status int, title, detail string) []byte {
	t.Helper()
	p := map[string]any{
		"type":   "https://encodeswarmr.dev/errors/test",
		"title":  title,
		"status": status,
		"detail": detail,
	}
	return mustMarshal(t, p)
}

// ---------------------------------------------------------------------------
// New
// ---------------------------------------------------------------------------

func TestNew_BaseURLTrimming(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"http://localhost:8080", "http://localhost:8080"},
		{"http://localhost:8080/", "http://localhost:8080"},
		{"http://localhost:8080///", "http://localhost:8080"},
		{"http://localhost:8080/path/", "http://localhost:8080/path"},
	}
	for _, tc := range cases {
		c := New(tc.input)
		if c.BaseURL() != tc.want {
			t.Errorf("New(%q).BaseURL() = %q, want %q", tc.input, c.BaseURL(), tc.want)
		}
	}
}

func TestNew_CookieJar(t *testing.T) {
	c := New("http://localhost:8080")
	if c.httpClient.Jar == nil {
		t.Error("expected cookie jar to be initialised, got nil")
	}
}

// ---------------------------------------------------------------------------
// SetAPIKey / BaseURL
// ---------------------------------------------------------------------------

func TestSetAPIKey(t *testing.T) {
	c := New("http://localhost:8080")
	if c.apiKey != "" {
		t.Fatal("expected empty apiKey on new client")
	}
	c.SetAPIKey("test-key-123")
	if c.apiKey != "test-key-123" {
		t.Errorf("apiKey = %q, want %q", c.apiKey, "test-key-123")
	}
}

func TestBaseURL(t *testing.T) {
	c := New("http://example.com/")
	if c.BaseURL() != "http://example.com" {
		t.Errorf("BaseURL() = %q, want %q", c.BaseURL(), "http://example.com")
	}
}

// ---------------------------------------------------------------------------
// Problem.Error
// ---------------------------------------------------------------------------

func TestProblemError_WithDetail(t *testing.T) {
	p := &Problem{Title: "Not Found", Detail: "job not found"}
	want := "Not Found: job not found"
	if p.Error() != want {
		t.Errorf("Error() = %q, want %q", p.Error(), want)
	}
}

func TestProblemError_WithoutDetail(t *testing.T) {
	p := &Problem{Title: "Unauthorized"}
	want := "Unauthorized"
	if p.Error() != want {
		t.Errorf("Error() = %q, want %q", p.Error(), want)
	}
}

func TestProblemError_EmptyDetail(t *testing.T) {
	p := &Problem{Title: "Bad Request", Detail: ""}
	want := "Bad Request"
	if p.Error() != want {
		t.Errorf("Error() = %q, want %q", p.Error(), want)
	}
}

// ---------------------------------------------------------------------------
// buildQuery
// ---------------------------------------------------------------------------

func TestBuildQuery_Empty(t *testing.T) {
	q := buildQuery(map[string]string{})
	if q != "" {
		t.Errorf("buildQuery(empty) = %q, want %q", q, "")
	}
}

func TestBuildQuery_NilMap(t *testing.T) {
	q := buildQuery(nil)
	if q != "" {
		t.Errorf("buildQuery(nil) = %q, want %q", q, "")
	}
}

func TestBuildQuery_SingleParam(t *testing.T) {
	q := buildQuery(map[string]string{"status": "running"})
	if q != "?status=running" {
		t.Errorf("buildQuery = %q, want %q", q, "?status=running")
	}
}

func TestBuildQuery_MultipleParams(t *testing.T) {
	q := buildQuery(map[string]string{"status": "running", "search": "movie"})
	// Both must be present; order is not guaranteed.
	if !strings.HasPrefix(q, "?") {
		t.Errorf("buildQuery missing leading '?': %q", q)
	}
	if !strings.Contains(q, "status=running") {
		t.Errorf("buildQuery missing status param: %q", q)
	}
	if !strings.Contains(q, "search=movie") {
		t.Errorf("buildQuery missing search param: %q", q)
	}
}

func TestBuildQuery_SkipsEmptyValues(t *testing.T) {
	q := buildQuery(map[string]string{"status": "running", "search": "", "cursor": ""})
	if !strings.HasPrefix(q, "?") {
		t.Errorf("buildQuery missing leading '?': %q", q)
	}
	if strings.Contains(q, "search") {
		t.Errorf("buildQuery should skip empty 'search' param, got: %q", q)
	}
	if strings.Contains(q, "cursor") {
		t.Errorf("buildQuery should skip empty 'cursor' param, got: %q", q)
	}
	if !strings.Contains(q, "status=running") {
		t.Errorf("buildQuery should retain non-empty 'status' param, got: %q", q)
	}
}

func TestBuildQuery_AllEmpty(t *testing.T) {
	q := buildQuery(map[string]string{"status": "", "search": ""})
	if q != "" {
		t.Errorf("buildQuery(all empty) = %q, want empty string", q)
	}
}

// ---------------------------------------------------------------------------
// request
// ---------------------------------------------------------------------------

func TestRequest_SuccessfulJSONEnvelope(t *testing.T) {
	user := User{
		ID:       "u1",
		Username: "alice",
		Email:    "alice@example.com",
		Role:     "admin",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/users/me" {
			t.Errorf("unexpected path: %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, user, map[string]any{"request_id": "req-1"}))
	}))
	defer srv.Close()

	c := New(srv.URL)
	var got User
	if err := c.request(context.Background(), "GET", "/users/me", nil, &got); err != nil {
		t.Fatalf("request() error = %v", err)
	}
	if got.ID != user.ID {
		t.Errorf("ID = %q, want %q", got.ID, user.ID)
	}
	if got.Username != user.Username {
		t.Errorf("Username = %q, want %q", got.Username, user.Username)
	}
}

func TestRequest_ErrorResponse_WithProblemJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(problemResponse(t, http.StatusNotFound, "Not Found", "job not found"))
	}))
	defer srv.Close()

	c := New(srv.URL)
	err := c.request(context.Background(), "GET", "/jobs/nonexistent", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	p, ok := err.(*Problem)
	if !ok {
		t.Fatalf("expected *Problem error, got %T: %v", err, err)
	}
	if p.Title != "Not Found" {
		t.Errorf("Title = %q, want %q", p.Title, "Not Found")
	}
	if p.Detail != "job not found" {
		t.Errorf("Detail = %q, want %q", p.Detail, "job not found")
	}
	if p.Status != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", p.Status, http.StatusNotFound)
	}
}

func TestRequest_ErrorResponse_WithoutProblemJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("plain text error"))
	}))
	defer srv.Close()

	c := New(srv.URL)
	err := c.request(context.Background(), "GET", "/jobs", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Should not be a *Problem since JSON cannot be decoded.
	if _, ok := err.(*Problem); ok {
		t.Error("expected non-Problem error for plain text body")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error = %q, want it to contain status code 400", err.Error())
	}
}

func TestRequest_APIKeyHeaderSent(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, map[string]string{}, map[string]any{}))
	}))
	defer srv.Close()

	c := New(srv.URL)
	c.SetAPIKey("my-secret-key")
	var result map[string]string
	_ = c.request(context.Background(), "GET", "/anything", nil, &result)

	if gotKey != "my-secret-key" {
		t.Errorf("X-API-Key = %q, want %q", gotKey, "my-secret-key")
	}
}

func TestRequest_NoAPIKeyHeader_WhenNotSet(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, map[string]string{}, map[string]any{}))
	}))
	defer srv.Close()

	c := New(srv.URL)
	var result map[string]string
	_ = c.request(context.Background(), "GET", "/anything", nil, &result)

	if gotKey != "" {
		t.Errorf("X-API-Key header should not be set, got %q", gotKey)
	}
}

func TestRequest_ContentTypeHeaderSent(t *testing.T) {
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, map[string]string{}, map[string]any{}))
	}))
	defer srv.Close()

	c := New(srv.URL)
	var result map[string]string
	_ = c.request(context.Background(), "GET", "/anything", nil, &result)

	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want %q", gotCT, "application/json")
	}
}

func TestRequest_BodyMarshaled(t *testing.T) {
	type reqBody struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	var gotBody reqBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, map[string]string{}, map[string]any{}))
	}))
	defer srv.Close()

	c := New(srv.URL)
	var result map[string]string
	payload := reqBody{Name: "alice", Age: 30}
	_ = c.request(context.Background(), "POST", "/anything", payload, &result)

	if gotBody.Name != "alice" {
		t.Errorf("body.name = %q, want %q", gotBody.Name, "alice")
	}
	if gotBody.Age != 30 {
		t.Errorf("body.age = %d, want 30", gotBody.Age)
	}
}

func TestRequest_NilResult_FireAndForget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL)
	// result=nil: should not attempt to decode body, no error expected.
	err := c.request(context.Background(), "POST", "/jobs/j1/cancel", nil, nil)
	if err != nil {
		t.Fatalf("request() with nil result error = %v", err)
	}
}

func TestRequest_PrependAPIV1Prefix(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, map[string]string{}, map[string]any{}))
	}))
	defer srv.Close()

	c := New(srv.URL)
	var result map[string]string
	_ = c.request(context.Background(), "GET", "/jobs", nil, &result)

	if gotPath != "/api/v1/jobs" {
		t.Errorf("path = %q, want %q", gotPath, "/api/v1/jobs")
	}
}

// ---------------------------------------------------------------------------
// requestCollection
// ---------------------------------------------------------------------------

func TestRequestCollection_WithItems(t *testing.T) {
	jobs := []Job{
		{ID: "j1", Status: JobQueued, CreatedAt: time.Now()},
		{ID: "j2", Status: JobRunning, CreatedAt: time.Now()},
	}
	meta := map[string]any{
		"request_id":  "req-col-1",
		"total_count": float64(42),
		"next_cursor": "cursor-xyz",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, jobs, meta))
	}))
	defer srv.Close()

	c := New(srv.URL)
	col, err := requestCollection[Job](c, context.Background(), "/jobs")
	if err != nil {
		t.Fatalf("requestCollection() error = %v", err)
	}
	if len(col.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2", len(col.Items))
	}
	if col.Items[0].ID != "j1" {
		t.Errorf("Items[0].ID = %q, want %q", col.Items[0].ID, "j1")
	}
	if col.TotalCount != 42 {
		t.Errorf("TotalCount = %d, want 42", col.TotalCount)
	}
	if col.NextCursor != "cursor-xyz" {
		t.Errorf("NextCursor = %q, want %q", col.NextCursor, "cursor-xyz")
	}
}

func TestRequestCollection_EmptyCollection(t *testing.T) {
	meta := map[string]any{
		"request_id":  "req-empty",
		"total_count": float64(0),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, []Job{}, meta))
	}))
	defer srv.Close()

	c := New(srv.URL)
	col, err := requestCollection[Job](c, context.Background(), "/jobs")
	if err != nil {
		t.Fatalf("requestCollection() error = %v", err)
	}
	if len(col.Items) != 0 {
		t.Errorf("Items should be empty, got %d items", len(col.Items))
	}
	if col.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", col.TotalCount)
	}
	if col.NextCursor != "" {
		t.Errorf("NextCursor = %q, want empty string", col.NextCursor)
	}
}

func TestRequestCollection_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write(problemResponse(t, http.StatusUnauthorized, "Unauthorized", ""))
	}))
	defer srv.Close()

	c := New(srv.URL)
	col, err := requestCollection[Job](c, context.Background(), "/jobs")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if col != nil {
		t.Errorf("expected nil collection on error, got %+v", col)
	}
	p, ok := err.(*Problem)
	if !ok {
		t.Fatalf("expected *Problem error, got %T: %v", err, err)
	}
	if p.Title != "Unauthorized" {
		t.Errorf("Title = %q, want %q", p.Title, "Unauthorized")
	}
}

func TestRequestCollection_APIKeyHeaderSent(t *testing.T) {
	var gotKey string
	meta := map[string]any{"request_id": "r1", "total_count": float64(0)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, []Job{}, meta))
	}))
	defer srv.Close()

	c := New(srv.URL)
	c.SetAPIKey("coll-key")
	_, _ = requestCollection[Job](c, context.Background(), "/jobs")

	if gotKey != "coll-key" {
		t.Errorf("X-API-Key = %q, want %q", gotKey, "coll-key")
	}
}

// ---------------------------------------------------------------------------
// Login
// ---------------------------------------------------------------------------

func TestLogin_Success(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/auth/login" {
			t.Errorf("path = %q, want /auth/login", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL)
	if err := c.Login(context.Background(), "alice", "s3cr3t"); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if gotBody["username"] != "alice" {
		t.Errorf("username = %q, want %q", gotBody["username"], "alice")
	}
	if gotBody["password"] != "s3cr3t" {
		t.Errorf("password = %q, want %q", gotBody["password"], "s3cr3t")
	}
}

func TestLogin_Failure_401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := New(srv.URL)
	err := c.Login(context.Background(), "alice", "wrongpass")
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %q, want it to contain 401", err.Error())
	}
}

func TestLogin_DoesNotPrependAPIV1(t *testing.T) {
	// Login uses requestRaw, which must NOT prepend /api/v1.
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_ = c.Login(context.Background(), "u", "p")
	if gotPath != "/auth/login" {
		t.Errorf("path = %q, want /auth/login (no /api/v1 prefix)", gotPath)
	}
}

// ---------------------------------------------------------------------------
// Logout
// ---------------------------------------------------------------------------

func TestLogout_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/auth/logout" {
			t.Errorf("path = %q, want /auth/logout", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL)
	if err := c.Logout(context.Background()); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
}

func TestLogout_ErrorResponse_StillReturnsNil(t *testing.T) {
	// Logout ignores the status code (no status check in the implementation).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(srv.URL)
	// The current implementation does not check the status on Logout.
	err := c.Logout(context.Background())
	if err != nil {
		t.Fatalf("Logout() unexpected error = %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetMe
// ---------------------------------------------------------------------------

func TestGetMe_ReturnsUser(t *testing.T) {
	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	user := User{
		ID:        "u-abc",
		Username:  "bob",
		Email:     "bob@example.com",
		Role:      "operator",
		CreatedAt: now,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/users/me" {
			t.Errorf("path = %q, want /api/v1/users/me", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, user, map[string]any{"request_id": "req-me"}))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetMe() returned nil user")
	}
	if got.ID != user.ID {
		t.Errorf("ID = %q, want %q", got.ID, user.ID)
	}
	if got.Username != user.Username {
		t.Errorf("Username = %q, want %q", got.Username, user.Username)
	}
	if got.Email != user.Email {
		t.Errorf("Email = %q, want %q", got.Email, user.Email)
	}
	if got.Role != user.Role {
		t.Errorf("Role = %q, want %q", got.Role, user.Role)
	}
}

func TestGetMe_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write(problemResponse(t, http.StatusUnauthorized, "Unauthorized", "session expired"))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.GetMe(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got != nil {
		t.Errorf("expected nil user on error, got %+v", got)
	}
	p, ok := err.(*Problem)
	if !ok {
		t.Fatalf("expected *Problem, got %T: %v", err, err)
	}
	if p.Title != "Unauthorized" {
		t.Errorf("Title = %q, want %q", p.Title, "Unauthorized")
	}
}

// ---------------------------------------------------------------------------
// Envelope / Collection types
// ---------------------------------------------------------------------------

func TestEnvelope_UnmarshalData(t *testing.T) {
	raw := `{"data":{"id":"j1","status":"running"},"meta":{"request_id":"req-x","total_count":1}}`
	var env Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("Unmarshal Envelope: %v", err)
	}
	var job Job
	if err := json.Unmarshal(env.Data, &job); err != nil {
		t.Fatalf("Unmarshal Data: %v", err)
	}
	if job.ID != "j1" {
		t.Errorf("job.ID = %q, want %q", job.ID, "j1")
	}
	if job.Status != "running" {
		t.Errorf("job.Status = %q, want %q", job.Status, "running")
	}
	if env.Meta["request_id"] != "req-x" {
		t.Errorf("meta.request_id = %v, want %q", env.Meta["request_id"], "req-x")
	}
}

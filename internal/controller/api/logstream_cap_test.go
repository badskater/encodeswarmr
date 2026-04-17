package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/auth"
	"github.com/badskater/encodeswarmr/internal/controller/config"
	"github.com/badskater/encodeswarmr/internal/controller/webhooks"
	"github.com/badskater/encodeswarmr/internal/db"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newCapHub(maxUser, maxIP int) *logStreamHub {
	return newLogStreamHub(config.LogStreamConfig{
		MaxPerUser: maxUser,
		MaxPerIP:   maxIP,
	})
}

// makeDummyConn creates a logStreamConn with no real websocket.  We never call
// websocket methods on it in these counter-only unit tests.
func makeDummyConn() *logStreamConn {
	return &logStreamConn{
		conn: nil,
		send: make(chan []byte, 1),
	}
}

// newCapTestServer builds a *Server wired with the given hub (and a real auth
// service so the middleware can inject claims from a cookie).
func newCapTestServer(hub *logStreamHub, authStore db.Store) *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wh := webhooks.New(&stubStore{}, webhooks.Config{}, logger)
	svc, _ := auth.NewService(context.Background(), authStore,
		&config.AuthConfig{SessionTTL: time.Hour}, logger)
	return &Server{
		store:    authStore,
		logger:   logger,
		webhooks: wh,
		auth:     svc,
		logHub:   hub,
	}
}

// ---------------------------------------------------------------------------
// TestLogStreamHub_Subscribe — unit tests for counter logic
// ---------------------------------------------------------------------------

func TestLogStreamHub_Subscribe_AllowsUpToLimit(t *testing.T) {
	h := newCapHub(2, 100)

	c1 := makeDummyConn()
	cleanup1, ok, reason := h.subscribe("task1", "userA", "1.2.3.4", c1)
	if !ok {
		t.Fatalf("first subscribe rejected: %s", reason)
	}
	defer cleanup1()

	c2 := makeDummyConn()
	cleanup2, ok, reason := h.subscribe("task1", "userA", "1.2.3.4", c2)
	if !ok {
		t.Fatalf("second subscribe rejected: %s", reason)
	}
	defer cleanup2()

	// Third connection must be rejected (user limit = 2).
	c3 := makeDummyConn()
	_, ok, reason = h.subscribe("task1", "userA", "1.2.3.4", c3)
	if ok {
		t.Fatal("expected third subscribe to be rejected, but it was accepted")
	}
	if reason != "user_limit" {
		t.Errorf("reason = %q, want user_limit", reason)
	}
}

func TestLogStreamHub_Subscribe_RejectsOverIPLimit(t *testing.T) {
	h := newCapHub(100, 1)

	c1 := makeDummyConn()
	cleanup1, ok, reason := h.subscribe("task1", "userA", "10.0.0.1", c1)
	if !ok {
		t.Fatalf("first subscribe rejected: %s", reason)
	}
	defer cleanup1()

	// Second connection from same IP, different user — must be rejected.
	c2 := makeDummyConn()
	_, ok, reason = h.subscribe("task1", "userB", "10.0.0.1", c2)
	if ok {
		t.Fatal("expected second subscribe (IP limit) to be rejected, but it was accepted")
	}
	if reason != "ip_limit" {
		t.Errorf("reason = %q, want ip_limit", reason)
	}
}

func TestLogStreamHub_Subscribe_DecrementsOnCleanup(t *testing.T) {
	h := newCapHub(1, 100)

	c1 := makeDummyConn()
	cleanup1, ok, _ := h.subscribe("task1", "userA", "1.2.3.4", c1)
	if !ok {
		t.Fatal("first subscribe rejected")
	}

	// While occupied the second must fail.
	c2 := makeDummyConn()
	_, ok, _ = h.subscribe("task1", "userA", "1.2.3.4", c2)
	if ok {
		t.Fatal("expected second subscribe to be rejected before cleanup")
	}

	// After cleanup the slot is free.
	cleanup1()

	c3 := makeDummyConn()
	cleanup3, ok, reason := h.subscribe("task1", "userA", "1.2.3.4", c3)
	if !ok {
		t.Fatalf("subscribe after cleanup rejected: %s", reason)
	}
	defer cleanup3()
}

func TestLogStreamHub_Subscribe_ConcurrentIncrements(t *testing.T) {
	const goroutines = 50
	h := newCapHub(goroutines, goroutines*2)

	var wg sync.WaitGroup
	accepted := make([]func(), 0, goroutines)
	var mu sync.Mutex

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := makeDummyConn()
			cleanup, ok, _ := h.subscribe("taskX", "userA", "5.5.5.5", c)
			if ok {
				mu.Lock()
				accepted = append(accepted, cleanup)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	// Exactly goroutines connections should have been accepted (cap == goroutines).
	if len(accepted) != goroutines {
		t.Errorf("accepted = %d, want %d", len(accepted), goroutines)
	}

	// Overflow: one more must be rejected.
	extra := makeDummyConn()
	_, ok, reason := h.subscribe("taskX", "userA", "5.5.5.5", extra)
	if ok {
		t.Fatal("expected overflow subscribe to be rejected")
	}
	if reason != "user_limit" {
		t.Errorf("overflow reason = %q, want user_limit", reason)
	}

	for _, fn := range accepted {
		fn()
	}
}

func TestLogStreamHub_ZeroCap_Unlimited(t *testing.T) {
	// MaxPerUser = 0 means disabled (unlimited).
	h := newCapHub(0, 0)

	cleanups := make([]func(), 0, 50)
	for i := 0; i < 50; i++ {
		c := makeDummyConn()
		cleanup, ok, reason := h.subscribe("taskY", "userA", "9.9.9.9", c)
		if !ok {
			t.Fatalf("subscribe %d rejected (%s) but cap should be disabled", i, reason)
		}
		cleanups = append(cleanups, cleanup)
	}
	for _, fn := range cleanups {
		fn()
	}
}

// ---------------------------------------------------------------------------
// TestClientIP — unit tests for IP extraction helper
// ---------------------------------------------------------------------------

func TestClientIP_XForwardedFor(t *testing.T) {
	tests := []struct {
		xff  string
		want string
	}{
		{"1.2.3.4", "1.2.3.4"},
		{"1.2.3.4, 10.0.0.1, 10.0.0.2", "1.2.3.4"},
		{"  203.0.113.5 , 10.1.1.1", "203.0.113.5"},
	}
	for _, tt := range tests {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-Forwarded-For", tt.xff)
		got := clientIP(r)
		if got != tt.want {
			t.Errorf("clientIP(xff=%q) = %q, want %q", tt.xff, got, tt.want)
		}
	}
}

func TestClientIP_RemoteAddr(t *testing.T) {
	tests := []struct {
		remoteAddr string
		want       string
	}{
		{"192.168.1.10:54321", "192.168.1.10"},
		{"[::1]:8080", "::1"},
		{"10.0.0.1:443", "10.0.0.1"},
	}
	for _, tt := range tests {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = tt.remoteAddr
		got := clientIP(r)
		if got != tt.want {
			t.Errorf("clientIP(remoteAddr=%q) = %q, want %q", tt.remoteAddr, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// TestHandleStreamTaskLogs_CapRejection — HTTP-level 429 tests.
//
// The handler reads user ID from auth.Claims in the context.  auth.withClaims
// is unexported, so we route through a real auth.Middleware backed by a stub
// session store — the same pattern used in me_test.go.
// ---------------------------------------------------------------------------

// capSessionStore satisfies the auth middleware session lookup.
type capSessionStore struct {
	*stubStore
	token string
	user  *db.User
}

func (s *capSessionStore) GetSessionByToken(_ context.Context, token string) (*db.Session, error) {
	if token == s.token {
		return &db.Session{Token: token, UserID: s.user.ID, ExpiresAt: time.Now().Add(time.Hour)}, nil
	}
	return nil, db.ErrNotFound
}

func (s *capSessionStore) GetUserByID(_ context.Context, _ string) (*db.User, error) {
	return s.user, nil
}

func TestHandleStreamTaskLogs_UserCapExceeded_Returns429(t *testing.T) {
	hub := newCapHub(1, 100)
	// Pre-fill the slot for "user-x" so the next connection is rejected.
	existing := makeDummyConn()
	cleanup, ok, _ := hub.subscribe("task-abc", "user-x", "127.0.0.1", existing)
	if !ok {
		t.Fatal("pre-filling hub failed")
	}
	defer cleanup()

	authStore := &capSessionStore{
		stubStore: &stubStore{},
		token:     "tok-userx",
		user:      &db.User{ID: "user-x", Username: "testuser", Role: "viewer"},
	}
	srv := newCapTestServer(hub, authStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/task-abc/logs/stream", nil)
	req.SetPathValue("id", "task-abc")
	req.RemoteAddr = "127.0.0.1:9999"
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok-userx"})

	rr := httptest.NewRecorder()
	// Route through real auth middleware so claims are set.
	srv.auth.Middleware(http.HandlerFunc(srv.handleStreamTaskLogs)).ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rr.Code)
	}
	if ra := rr.Header().Get("Retry-After"); ra != "60" {
		t.Errorf("Retry-After = %q, want 60", ra)
	}
	// Verify RFC 9457 problem body.
	var p problem
	if err := json.NewDecoder(rr.Body).Decode(&p); err != nil {
		t.Fatalf("decode problem body: %v", err)
	}
	if p.Status != http.StatusTooManyRequests {
		t.Errorf("problem.status = %d, want 429", p.Status)
	}
	if !strings.Contains(p.Detail, "user_limit") {
		t.Errorf("problem.detail = %q, should mention user_limit", p.Detail)
	}
}

func TestHandleStreamTaskLogs_IPCapExceeded_Returns429(t *testing.T) {
	hub := newCapHub(100, 1)
	// Pre-fill the IP slot for "203.0.113.9".
	existing := makeDummyConn()
	cleanup, ok, _ := hub.subscribe("task-xyz", "user-other", "203.0.113.9", existing)
	if !ok {
		t.Fatal("pre-filling hub failed")
	}
	defer cleanup()

	authStore := &capSessionStore{
		stubStore: &stubStore{},
		token:     "tok-new",
		user:      &db.User{ID: "user-new", Username: "newuser", Role: "viewer"},
	}
	srv := newCapTestServer(hub, authStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/task-xyz/logs/stream", nil)
	req.SetPathValue("id", "task-xyz")
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok-new"})

	rr := httptest.NewRecorder()
	srv.auth.Middleware(http.HandlerFunc(srv.handleStreamTaskLogs)).ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rr.Code)
	}
	var p problem
	if err := json.NewDecoder(rr.Body).Decode(&p); err != nil {
		t.Fatalf("decode problem body: %v", err)
	}
	if !strings.Contains(p.Detail, "ip_limit") {
		t.Errorf("problem.detail = %q, should mention ip_limit", p.Detail)
	}
}

func TestHandleStreamTaskLogs_MissingTaskID_Returns400(t *testing.T) {
	hub := newCapHub(10, 20)

	authStore := &capSessionStore{
		stubStore: &stubStore{},
		token:     "tok-z",
		user:      &db.User{ID: "user-z", Username: "userz", Role: "viewer"},
	}
	srv := newCapTestServer(hub, authStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks//logs/stream", nil)
	// PathValue("id") is empty — no SetPathValue call.
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok-z"})

	rr := httptest.NewRecorder()
	srv.auth.Middleware(http.HandlerFunc(srv.handleStreamTaskLogs)).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

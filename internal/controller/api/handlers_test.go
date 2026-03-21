package api

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/auth"
	"github.com/badskater/encodeswarmr/internal/controller/config"
	"github.com/badskater/encodeswarmr/internal/db"
)

// ---------------------------------------------------------------------------
// newMinimalAuthService creates a real *auth.Service with OIDC disabled.
// Used when tests need s.auth to be non-nil but OIDC disabled.
// ---------------------------------------------------------------------------

func newMinimalAuthService() *auth.Service {
	svc, _ := auth.NewService(
		context.Background(),
		&stubStore{},
		&config.AuthConfig{
			SessionTTL: time.Hour,
			OIDC:       config.OIDCConfig{Enabled: false},
		},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	return svc
}

// ---------------------------------------------------------------------------
// handleGetTask
// ---------------------------------------------------------------------------

func TestHandleGetTask(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := &getTaskStore{
			stubStore: &stubStore{},
			task:      &db.Task{ID: "t1", JobID: "j1", Status: "pending"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/t1", nil)
		req.SetPathValue("id", "t1")
		srv.handleGetTask(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body struct {
			Data db.Task `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data.ID != "t1" {
			t.Errorf("data.ID = %q, want t1", body.Data.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		store := &getTaskNotFoundStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/missing", nil)
		req.SetPathValue("id", "missing")
		srv.handleGetTask(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("missing path value returns 400", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/", nil)
		srv.handleGetTask(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})
}

type getTaskStore struct {
	*stubStore
	task *db.Task
}

func (s *getTaskStore) GetTaskByID(_ context.Context, _ string) (*db.Task, error) {
	return s.task, nil
}

type getTaskNotFoundStore struct {
	*stubStore
}

func (s *getTaskNotFoundStore) GetTaskByID(_ context.Context, _ string) (*db.Task, error) {
	return nil, db.ErrNotFound
}

// ---------------------------------------------------------------------------
// handleTailTaskLogs
// ---------------------------------------------------------------------------

func TestHandleTailTaskLogs(t *testing.T) {
	t.Run("missing id returns 400", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks//logs/tail", nil)
		srv.handleTailTaskLogs(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("non-flusher writer returns 500", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		w := &nonFlusherWriter{header: make(http.Header)}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/t1/logs/tail", nil)
		req.SetPathValue("id", "t1")
		srv.handleTailTaskLogs(w, req)

		if w.code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.code)
		}
	})
}

// nonFlusherWriter does NOT implement http.Flusher.
type nonFlusherWriter struct {
	header http.Header
	code   int
	body   []byte
}

func (w *nonFlusherWriter) Header() http.Header         { return w.header }
func (w *nonFlusherWriter) WriteHeader(code int)        { w.code = code }
func (w *nonFlusherWriter) Write(b []byte) (int, error) { w.body = append(w.body, b...); return len(b), nil }

// ---------------------------------------------------------------------------
// handleSetup
// ---------------------------------------------------------------------------

func TestHandleSetup_ShortPassword(t *testing.T) {
	setupDone.Store(false)
	store := &setupZeroAdminsStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"username":"admin","email":"admin@example.com","password":"short"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/setup", bytes.NewBufferString(body))
	srv.handleSetup(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}
}

func TestHandleSetup_AlreadyDone(t *testing.T) {
	store := &setupOneAdminStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/setup", bytes.NewBufferString(`{}`))
	srv.handleSetup(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
}

func TestHandleSetup_MissingFields(t *testing.T) {
	setupDone.Store(false)
	store := &setupZeroAdminsStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"username":"admin","email":"admin@example.com"}` // missing password
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/setup", bytes.NewBufferString(body))
	srv.handleSetup(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}
}

func TestHandleSetup_ValidRequest(t *testing.T) {
	setupDone.Store(false)
	store := &setupZeroAdminsStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"username":"admin","email":"admin@example.com","password":"my-secret-pass"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/setup", bytes.NewBufferString(body))
	srv.handleSetup(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rr.Code)
	}
}

type setupZeroAdminsStore struct {
	*stubStore
}

func (s *setupZeroAdminsStore) CountAdminUsers(_ context.Context) (int64, error) {
	return 0, nil
}

func (s *setupZeroAdminsStore) CreateUser(_ context.Context, p db.CreateUserParams) (*db.User, error) {
	return &db.User{ID: "admin-1", Username: p.Username, Email: p.Email, Role: p.Role}, nil
}

type setupOneAdminStore struct {
	*stubStore
}

func (s *setupOneAdminStore) CountAdminUsers(_ context.Context) (int64, error) {
	return 1, nil
}

// ---------------------------------------------------------------------------
// handleNoVNCViewer
// ---------------------------------------------------------------------------

func TestHandleNoVNCViewer(t *testing.T) {
	t.Run("agent not found returns 404", func(t *testing.T) {
		store := &vncAgentNotFoundStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/novnc/missing", nil)
		req.SetPathValue("id", "missing")
		srv.handleNoVNCViewer(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})

	t.Run("existing agent returns HTML page", func(t *testing.T) {
		store := &vncAgentFoundStore{
			stubStore: &stubStore{},
			agent:     &db.Agent{ID: "agent-1", Name: "worker-1", IPAddress: "10.0.0.1"},
		}
		srv := newTestServer(store)
		srv.cfg = &config.Config{}

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/novnc/agent-1", nil)
		req.SetPathValue("id", "agent-1")
		srv.handleNoVNCViewer(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		ct := rr.Header().Get("Content-Type")
		if ct != "text/html; charset=utf-8" {
			t.Errorf("Content-Type = %q, want text/html", ct)
		}
	})
}

type vncAgentNotFoundStore struct {
	*stubStore
}

func (s *vncAgentNotFoundStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return nil, db.ErrNotFound
}

type vncAgentFoundStore struct {
	*stubStore
	agent *db.Agent
}

func (s *vncAgentFoundStore) GetAgentByID(_ context.Context, _ string) (*db.Agent, error) {
	return s.agent, nil
}

// ---------------------------------------------------------------------------
// handleAgentVNCProxy — pre-upgrade error paths
// ---------------------------------------------------------------------------

func TestHandleAgentVNCProxy_NotFound(t *testing.T) {
	store := &vncAgentNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/missing/vnc", nil)
	req.SetPathValue("id", "missing")
	srv.handleAgentVNCProxy(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestHandleAgentVNCProxy_NoVNCPort(t *testing.T) {
	store := &vncAgentFoundStore{
		stubStore: &stubStore{},
		agent:     &db.Agent{ID: "agent-2", Name: "worker-2", IPAddress: "10.0.0.2", VNCPort: 0},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/agent-2/vnc", nil)
	req.SetPathValue("id", "agent-2")
	srv.handleAgentVNCProxy(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
}

func TestHandleAgentVNCProxy_UnreachableVNCPort(t *testing.T) {
	store := &vncAgentFoundStore{
		stubStore: &stubStore{},
		agent:     &db.Agent{ID: "agent-3", Name: "worker-3", IPAddress: "127.0.0.1", VNCPort: 1},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/agent-3/vnc", nil)
	req.SetPathValue("id", "agent-3")
	srv.handleAgentVNCProxy(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// clearOIDCStateCookie
// ---------------------------------------------------------------------------

func TestClearOIDCStateCookie(t *testing.T) {
	rr := httptest.NewRecorder()
	clearOIDCStateCookie(rr)

	cookies := rr.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == "oidc_state" {
			found = true
			if c.MaxAge != -1 {
				t.Errorf("MaxAge = %d, want -1 (cleared)", c.MaxAge)
			}
		}
	}
	if !found {
		t.Error("oidc_state cookie not found in response")
	}
}

// ---------------------------------------------------------------------------
// handleLogin / handleLogout (input validation paths only)
// ---------------------------------------------------------------------------

func TestHandleLogin_Validation(t *testing.T) {
	t.Run("missing credentials returns 422", func(t *testing.T) {
		srv := &Server{
			store:  &stubStore{},
			logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"username":""}`))
		srv.handleLogin(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422", rr.Code)
		}
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		srv := &Server{
			store:  &stubStore{},
			logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{invalid json`))
		srv.handleLogin(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})
}

func TestHandleLogout_NoSession(t *testing.T) {
	srv := &Server{
		store:  &stubStore{},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	srv.handleLogout(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// handleOIDCRedirect/Callback — OIDC disabled
// ---------------------------------------------------------------------------

func TestHandleOIDCRedirect_Disabled(t *testing.T) {
	srv := &Server{
		store:  &stubStore{},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		auth:   newMinimalAuthService(),
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/oidc", nil)
	srv.handleOIDCRedirect(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (OIDC disabled)", rr.Code)
	}
}

func TestHandleOIDCCallback_Disabled(t *testing.T) {
	srv := &Server{
		store:  &stubStore{},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		auth:   newMinimalAuthService(),
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/oidc/callback", nil)
	srv.handleOIDCCallback(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (OIDC disabled)", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Hub tests
// ---------------------------------------------------------------------------

func TestHub_NewHub(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHub(logger)

	if h == nil {
		t.Fatal("expected non-nil Hub")
	}
	if h.broadcast == nil {
		t.Error("expected broadcast channel to be initialized")
	}
	if h.clients == nil {
		t.Error("expected clients map to be initialized")
	}
}

func TestHub_Publish(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHub(logger)

	evt := HubEvent{Type: "test.event", Payload: map[string]any{"key": "value"}}
	h.Publish(evt)

	select {
	case got := <-h.broadcast:
		if got.Type != "test.event" {
			t.Errorf("event type = %q, want test.event", got.Type)
		}
	default:
		t.Error("expected event in broadcast channel")
	}
}

func TestHub_Publish_DropWhenFull(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHub(logger)

	// Fill the channel (capacity 256).
	for i := 0; i < 256; i++ {
		h.Publish(HubEvent{Type: "fill"})
	}
	// This should not block or panic — it drops silently.
	h.Publish(HubEvent{Type: "dropped"})
}

func TestHub_Run_CancelledContextExits(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHub(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		h.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Run exited when context cancelled
	case <-time.After(2 * time.Second):
		t.Fatal("Hub.Run did not exit after context cancellation")
	}
}

func TestHub_Run_DrainsBroadcastOnCancel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHub(logger)

	// Enqueue some events before cancelling.
	for i := 0; i < 5; i++ {
		h.Publish(HubEvent{Type: "fill"})
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		h.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// good
	case <-time.After(2 * time.Second):
		t.Fatal("Hub.Run did not exit after context cancellation")
	}
}

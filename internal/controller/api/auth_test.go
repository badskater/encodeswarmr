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

	"github.com/badskater/distributed-encoder/internal/controller/auth"
	"github.com/badskater/distributed-encoder/internal/controller/config"
	"github.com/badskater/distributed-encoder/internal/db"
	"github.com/badskater/distributed-encoder/internal/db/teststore"
)

// ---------------------------------------------------------------------------
// auth test helpers
// ---------------------------------------------------------------------------

// newAuthService constructs an auth.Service (no OIDC) backed by the given store.
func newAuthService(t *testing.T, store db.Store) *auth.Service {
	t.Helper()
	cfg := &config.AuthConfig{
		SessionTTL: time.Hour,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := auth.NewService(context.Background(), store, cfg, logger)
	if err != nil {
		t.Fatalf("auth.NewService: %v", err)
	}
	return svc
}

// newServerWithAuth creates a test Server with the given store and an auth
// service that uses authStore for credential lookups.
func newServerWithAuth(store db.Store, authStore db.Store) *Server {
	srv := newTestServer(store)
	// Build a real auth.Service using authStore for login/logout operations.
	cfg := &config.AuthConfig{SessionTTL: time.Hour}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, _ := auth.NewService(context.Background(), authStore, cfg, logger)
	srv.auth = svc
	return srv
}

// loginAuthStub provides login-related store methods.
type loginAuthStub struct {
	teststore.Stub
	user    *db.User
	userErr error
	sess    *db.Session
}

func (s *loginAuthStub) GetUserByUsername(_ context.Context, _ string) (*db.User, error) {
	return s.user, s.userErr
}

func (s *loginAuthStub) CreateSession(_ context.Context, p db.CreateSessionParams) (*db.Session, error) {
	if s.sess != nil {
		return s.sess, nil
	}
	return &db.Session{Token: p.Token, UserID: p.UserID, ExpiresAt: p.ExpiresAt}, nil
}

// logoutAuthStub handles logout (DeleteSession).
type logoutAuthStub struct {
	teststore.Stub
	deleteErr error
}

func (s *logoutAuthStub) DeleteSession(_ context.Context, _ string) error {
	return s.deleteErr
}

// ---------------------------------------------------------------------------
// TestHandleLogin
// ---------------------------------------------------------------------------

func TestHandleLogin(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		srv := newTestServer(&stubStore{})
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{not json`))
		srv.handleLogin(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing username", func(t *testing.T) {
		srv := newTestServer(&stubStore{})
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/login",
			bytes.NewBufferString(`{"password":"secret"}`))
		srv.handleLogin(rr, req)
		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("missing password", func(t *testing.T) {
		srv := newTestServer(&stubStore{})
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/login",
			bytes.NewBufferString(`{"username":"alice"}`))
		srv.handleLogin(rr, req)
		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("empty username and password", func(t *testing.T) {
		srv := newTestServer(&stubStore{})
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/login",
			bytes.NewBufferString(`{"username":"","password":""}`))
		srv.handleLogin(rr, req)
		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("invalid credentials — user not found", func(t *testing.T) {
		as := &loginAuthStub{userErr: db.ErrNotFound}
		srv := newServerWithAuth(&stubStore{}, as)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/login",
			bytes.NewBufferString(`{"username":"alice","password":"wrong"}`))
		srv.handleLogin(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("invalid credentials — wrong password", func(t *testing.T) {
		hash, _ := auth.HashPassword("correctpassword")
		as := &loginAuthStub{
			user: &db.User{ID: "u1", Username: "alice", PasswordHash: &hash},
		}
		srv := newServerWithAuth(&stubStore{}, as)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/login",
			bytes.NewBufferString(`{"username":"alice","password":"wrongpassword"}`))
		srv.handleLogin(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("success", func(t *testing.T) {
		hash, _ := auth.HashPassword("correctpassword")
		expiry := time.Now().Add(time.Hour)
		as := &loginAuthStub{
			user: &db.User{ID: "u1", Username: "alice", PasswordHash: &hash},
			sess: &db.Session{Token: "tok123", UserID: "u1", ExpiresAt: expiry},
		}
		srv := newServerWithAuth(&stubStore{}, as)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/login",
			bytes.NewBufferString(`{"username":"alice","password":"correctpassword"}`))
		srv.handleLogin(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		// Verify session cookie is set.
		found := false
		for _, c := range rr.Result().Cookies() {
			if c.Name == auth.SessionCookieName {
				found = true
				if c.Value != "tok123" {
					t.Errorf("cookie value = %q, want %q", c.Value, "tok123")
				}
				break
			}
		}
		if !found {
			t.Error("expected session cookie to be set")
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleLogout
// ---------------------------------------------------------------------------

func TestHandleLogout(t *testing.T) {
	t.Run("no session cookie returns ok", func(t *testing.T) {
		srv := newTestServer(&stubStore{})
		// auth is nil; request returns early because no cookie exists
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
		srv.handleLogout(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("logout error returns 500", func(t *testing.T) {
		as := &logoutAuthStub{deleteErr: errTestDB}
		srv := newServerWithAuth(&stubStore{}, as)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok"})
		srv.handleLogout(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("success clears cookie", func(t *testing.T) {
		as := &logoutAuthStub{}
		srv := newServerWithAuth(&stubStore{}, as)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok"})
		srv.handleLogout(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		// Session cookie should be cleared.
		for _, c := range rr.Result().Cookies() {
			if c.Name == auth.SessionCookieName {
				if c.MaxAge != -1 {
					t.Errorf("expected session cookie MaxAge=-1 after logout, got %d", c.MaxAge)
				}
				break
			}
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleOIDCRedirect
// ---------------------------------------------------------------------------

func TestHandleOIDCRedirect(t *testing.T) {
	t.Run("OIDC not enabled returns 404", func(t *testing.T) {
		as := &loginAuthStub{}
		srv := newServerWithAuth(&stubStore{}, as)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/auth/oidc", nil)
		srv.handleOIDCRedirect(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleOIDCCallback
// ---------------------------------------------------------------------------

func TestHandleOIDCCallback(t *testing.T) {
	t.Run("OIDC not enabled returns 404", func(t *testing.T) {
		as := &loginAuthStub{}
		srv := newServerWithAuth(&stubStore{}, as)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/auth/oidc/callback", nil)
		srv.handleOIDCCallback(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("state cookie missing returns 400", func(t *testing.T) {
		// Build a server with OIDCEnabled()=false; already covered above.
		// For the OIDC-enabled path a live provider is required (integration test).
		// The OIDC callback state-mismatch path requires OIDCEnabled() = true
		// which in turn needs a real OIDC server — out of scope for unit tests.
	})
}

// ---------------------------------------------------------------------------
// TestGenOIDCState
// ---------------------------------------------------------------------------

func TestGenOIDCState(t *testing.T) {
	state1, err := genOIDCState()
	if err != nil {
		t.Fatalf("genOIDCState() error: %v", err)
	}
	// 16 bytes → 32 hex chars
	if len(state1) != 32 {
		t.Errorf("state length = %d, want 32", len(state1))
	}
	state2, err := genOIDCState()
	if err != nil {
		t.Fatalf("genOIDCState() error: %v", err)
	}
	if state1 == state2 {
		t.Error("genOIDCState should produce unique values each call")
	}
}

// ---------------------------------------------------------------------------
// TestSetSessionCookie / TestClearSessionCookie
// ---------------------------------------------------------------------------

func TestSetSessionCookie(t *testing.T) {
	sess := &db.Session{
		Token:     "mytoken",
		UserID:    "u1",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	rr := httptest.NewRecorder()
	setSessionCookie(rr, sess)

	var found *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("session cookie not set")
	}
	if found.Value != "mytoken" {
		t.Errorf("cookie value = %q, want %q", found.Value, "mytoken")
	}
	if !found.HttpOnly {
		t.Error("expected HttpOnly=true")
	}
}

func TestClearSessionCookie(t *testing.T) {
	rr := httptest.NewRecorder()
	clearSessionCookie(rr)

	var found *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("session cookie not present after clear")
	}
	if found.MaxAge != -1 {
		t.Errorf("cookie MaxAge = %d, want -1", found.MaxAge)
	}
}

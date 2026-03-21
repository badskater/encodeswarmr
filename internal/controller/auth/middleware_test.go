package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/config"
	"github.com/badskater/encodeswarmr/internal/db"
	"github.com/badskater/encodeswarmr/internal/db/teststore"
)

// ---------------------------------------------------------------------------
// Minimal db.Store stub — only the methods auth.Service actually calls.
// ---------------------------------------------------------------------------

type authStubStore struct {
	teststore.Stub
	session *db.Session
	sessErr error
	user    *db.User
	userErr error
}

func (s *authStubStore) GetSessionByToken(_ context.Context, _ string) (*db.Session, error) {
	return s.session, s.sessErr
}
func (s *authStubStore) GetUserByID(_ context.Context, _ string) (*db.User, error) {
	return s.user, s.userErr
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestService(store *authStubStore) *Service {
	return &Service{
		store: store,
		cfg:   &config.AuthConfig{SessionTTL: time.Hour},
	}
}

func okHandler(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

// ---------------------------------------------------------------------------
// TestMiddleware
// ---------------------------------------------------------------------------

func TestMiddleware(t *testing.T) {
	t.Run("no token returns 401", func(t *testing.T) {
		svc := newTestService(&authStubStore{})
		h := svc.Middleware(http.HandlerFunc(okHandler))

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", rr.Code)
		}
	})

	t.Run("unknown session token returns 401", func(t *testing.T) {
		store := &authStubStore{sessErr: db.ErrNotFound}
		svc := newTestService(store)
		h := svc.Middleware(http.HandlerFunc(okHandler))

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "bad-token"})
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", rr.Code)
		}
	})

	t.Run("valid cookie populates context claims", func(t *testing.T) {
		store := &authStubStore{
			session: &db.Session{Token: "tok", UserID: "u1"},
			user:    &db.User{ID: "u1", Username: "alice", Role: "admin"},
		}
		svc := newTestService(store)

		var gotClaims *Claims
		h := svc.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotClaims, _ = FromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		}))

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "tok"})
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("got %d, want 200", rr.Code)
		}
		if gotClaims == nil {
			t.Fatal("claims not set in context")
		}
		if gotClaims.Username != "alice" {
			t.Errorf("username = %q, want alice", gotClaims.Username)
		}
		if gotClaims.Role != "admin" {
			t.Errorf("role = %q, want admin", gotClaims.Role)
		}
	})

	t.Run("valid Bearer header populates context claims", func(t *testing.T) {
		store := &authStubStore{
			session: &db.Session{Token: "bearer-tok", UserID: "u2"},
			user:    &db.User{ID: "u2", Username: "bob", Role: "operator"},
		}
		svc := newTestService(store)

		var gotClaims *Claims
		h := svc.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotClaims, _ = FromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		}))

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer bearer-tok")
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("got %d, want 200", rr.Code)
		}
		if gotClaims == nil || gotClaims.Username != "bob" {
			t.Errorf("unexpected claims: %+v", gotClaims)
		}
	})
}

// ---------------------------------------------------------------------------
// TestRequireRole
// ---------------------------------------------------------------------------

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name     string
		role     string
		minRole  string
		wantCode int
	}{
		{"admin satisfies admin", "admin", "admin", http.StatusOK},
		{"admin satisfies operator", "admin", "operator", http.StatusOK},
		{"admin satisfies viewer", "admin", "viewer", http.StatusOK},
		{"operator satisfies operator", "operator", "operator", http.StatusOK},
		{"operator satisfies viewer", "operator", "viewer", http.StatusOK},
		{"operator denied admin", "operator", "admin", http.StatusForbidden},
		{"viewer satisfies viewer", "viewer", "viewer", http.StatusOK},
		{"viewer denied operator", "viewer", "operator", http.StatusForbidden},
		{"viewer denied admin", "viewer", "admin", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := RequireRole(tt.minRole, http.HandlerFunc(okHandler))

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			// Inject claims directly into context.
			req = req.WithContext(withClaims(req.Context(), &Claims{
				UserID:   "u1",
				Username: "user",
				Role:     tt.role,
			}))
			h.ServeHTTP(rr, req)

			if rr.Code != tt.wantCode {
				t.Errorf("got %d, want %d", rr.Code, tt.wantCode)
			}
		})
	}

	t.Run("no claims in context returns 401", func(t *testing.T) {
		h := RequireRole("viewer", http.HandlerFunc(okHandler))

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", rr.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// TestHasRole
// ---------------------------------------------------------------------------

func TestHasRole(t *testing.T) {
	tests := []struct {
		role    string
		minRole string
		want    bool
	}{
		{"admin", "admin", true},
		{"admin", "operator", true},
		{"admin", "viewer", true},
		{"operator", "admin", false},
		{"operator", "operator", true},
		{"operator", "viewer", true},
		{"viewer", "admin", false},
		{"viewer", "operator", false},
		{"viewer", "viewer", true},
		{"unknown", "viewer", false},
	}
	for _, tt := range tests {
		if got := hasRole(tt.role, tt.minRole); got != tt.want {
			t.Errorf("hasRole(%q, %q) = %v, want %v", tt.role, tt.minRole, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// TestExtractToken
// ---------------------------------------------------------------------------

func TestExtractToken(t *testing.T) {
	svc := newTestService(&authStubStore{})

	t.Run("returns empty string when no auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if tok := svc.extractToken(req); tok != "" {
			t.Errorf("expected empty, got %q", tok)
		}
	})

	t.Run("reads session cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "cookie-token"})
		if tok := svc.extractToken(req); tok != "cookie-token" {
			t.Errorf("got %q, want cookie-token", tok)
		}
	})

	t.Run("reads Bearer header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer hdr-token")
		if tok := svc.extractToken(req); tok != "hdr-token" {
			t.Errorf("got %q, want hdr-token", tok)
		}
	})

	t.Run("cookie takes precedence over Bearer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "cookie-wins"})
		req.Header.Set("Authorization", "Bearer bearer-token")
		if tok := svc.extractToken(req); tok != "cookie-wins" {
			t.Errorf("got %q, want cookie-wins", tok)
		}
	})
}

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/badskater/distributed-encoder/internal/controller/auth"
	"github.com/badskater/distributed-encoder/internal/db"
)

// ---------------------------------------------------------------------------
// TestHandleGetMe
// ---------------------------------------------------------------------------

// TestHandleGetMe tests the /api/v1/users/me handler.
//
// Because auth.withClaims (which injects claims into the context) is
// unexported, the only way to produce an authenticated context is to route the
// request through a real auth.Middleware.  Each sub-test below wires a stub
// store that satisfies both the middleware session lookup and the handler's own
// store calls.
func TestHandleGetMe(t *testing.T) {
	t.Run("no claims in context returns 401", func(t *testing.T) {
		// Call the handler directly without going through auth middleware.
		// context has no claims → handler returns 401.
		srv := newTestServer(&stubStore{})
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
		srv.handleGetMe(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("claims present + user not found returns 404", func(t *testing.T) {
		store := &getMeNotFoundStore{
			stubStore: &stubStore{},
			sess:     &db.Session{Token: "tok", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour)},
			sessUser: &db.User{ID: "u1", Username: "alice", Role: "viewer"},
		}
		srv := newMeTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok"})
		srv.auth.Middleware(http.HandlerFunc(srv.handleGetMe)).ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("claims present + store error returns 500", func(t *testing.T) {
		store := &getMeErrStore{
			stubStore: &stubStore{},
			sess:     &db.Session{Token: "tok", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour)},
			sessUser: &db.User{ID: "u1", Username: "alice", Role: "viewer"},
		}
		srv := newMeTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok"})
		srv.auth.Middleware(http.HandlerFunc(srv.handleGetMe)).ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("success returns user profile", func(t *testing.T) {
		user := &db.User{
			ID:       "u1",
			Username: "alice",
			Email:    "alice@example.com",
			Role:     "admin",
		}
		store := &getMeSuccessStore{
			stubStore: &stubStore{},
			sess:     &db.Session{Token: "tok", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour)},
			sessUser: user,
			meUser:   user,
		}
		srv := newMeTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok"})
		srv.auth.Middleware(http.HandlerFunc(srv.handleGetMe)).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data["username"] != "alice" {
			t.Errorf("data.username = %v, want alice", body.Data["username"])
		}
		if body.Data["email"] != "alice@example.com" {
			t.Errorf("data.email = %v, want alice@example.com", body.Data["email"])
		}
	})
}

// newMeTestServer creates a Server where both srv.auth and srv.store use the
// same stub store — auth middleware uses it for session/user lookups, and the
// handler uses it for GetUserByID.
func newMeTestServer(store db.Store) *Server {
	return newServerWithAuth(store, store)
}

// ---------------------------------------------------------------------------
// Store stubs
// ---------------------------------------------------------------------------

// getMeNotFoundStore: middleware finds the session user, but the second
// GetUserByID call (from handleGetMe) returns db.ErrNotFound.
type getMeNotFoundStore struct {
	*stubStore
	sess      *db.Session
	sessUser  *db.User
	callCount int
}

func (s *getMeNotFoundStore) GetSessionByToken(_ context.Context, _ string) (*db.Session, error) {
	return s.sess, nil
}

func (s *getMeNotFoundStore) GetUserByID(_ context.Context, _ string) (*db.User, error) {
	s.callCount++
	if s.callCount == 1 {
		// First call: auth.Service.GetSession resolving the session owner.
		return s.sessUser, nil
	}
	// Second call: handleGetMe looking up the authenticated user.
	return nil, db.ErrNotFound
}

// getMeErrStore: second GetUserByID returns a generic error.
type getMeErrStore struct {
	*stubStore
	sess      *db.Session
	sessUser  *db.User
	callCount int
}

func (s *getMeErrStore) GetSessionByToken(_ context.Context, _ string) (*db.Session, error) {
	return s.sess, nil
}

func (s *getMeErrStore) GetUserByID(_ context.Context, _ string) (*db.User, error) {
	s.callCount++
	if s.callCount == 1 {
		return s.sessUser, nil
	}
	return nil, errTestDB
}

// getMeSuccessStore: both GetUserByID calls succeed.
type getMeSuccessStore struct {
	*stubStore
	sess      *db.Session
	sessUser  *db.User
	meUser    *db.User
	callCount int
}

func (s *getMeSuccessStore) GetSessionByToken(_ context.Context, _ string) (*db.Session, error) {
	return s.sess, nil
}

func (s *getMeSuccessStore) GetUserByID(_ context.Context, _ string) (*db.User, error) {
	s.callCount++
	if s.callCount == 1 {
		return s.sessUser, nil
	}
	return s.meUser, nil
}

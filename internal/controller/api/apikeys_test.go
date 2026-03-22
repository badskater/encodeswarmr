package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/auth"
	"github.com/badskater/encodeswarmr/internal/db"
)

// ---------------------------------------------------------------------------
// newAPIKeyTestServer creates a Server with an auth service backed by a store
// that resolves the session used for authenticated test requests.
// ---------------------------------------------------------------------------

func newAPIKeyTestServer(store db.Store) *Server {
	return newServerWithAuth(store, store)
}

// apiKeyAuthStore is a base store that satisfies the session lookup used by
// auth.Middleware in API key tests.  Concrete test stubs embed this.
type apiKeyAuthStore struct {
	*stubStore
	session *db.Session
	user    *db.User
}

func (s *apiKeyAuthStore) GetSessionByToken(_ context.Context, _ string) (*db.Session, error) {
	return s.session, nil
}

func (s *apiKeyAuthStore) GetUserByID(_ context.Context, _ string) (*db.User, error) {
	return s.user, nil
}

// ---------------------------------------------------------------------------
// TestHandleCreateAPIKey
// ---------------------------------------------------------------------------

func TestHandleCreateAPIKey(t *testing.T) {
	t.Run("no claims returns 401", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/apikeys",
			bytes.NewBufferString(`{"name":"ci-token"}`))
		srv.handleCreateAPIKey(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		now := time.Now()
		user := &db.User{ID: "u1", Username: "alice", Role: "admin"}
		store := &createAPIKeyStore{
			apiKeyAuthStore: &apiKeyAuthStore{
				stubStore: &stubStore{},
				session:   &db.Session{Token: "tok", UserID: "u1", ExpiresAt: now.Add(time.Hour)},
				user:      user,
			},
		}
		srv := newAPIKeyTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/apikeys",
			bytes.NewBufferString(`{not json`))
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok"})
		srv.auth.Middleware(http.HandlerFunc(srv.handleCreateAPIKey)).ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("missing name returns 422", func(t *testing.T) {
		now := time.Now()
		user := &db.User{ID: "u1", Username: "alice", Role: "admin"}
		store := &createAPIKeyStore{
			apiKeyAuthStore: &apiKeyAuthStore{
				stubStore: &stubStore{},
				session:   &db.Session{Token: "tok", UserID: "u1", ExpiresAt: now.Add(time.Hour)},
				user:      user,
			},
		}
		srv := newAPIKeyTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/apikeys",
			bytes.NewBufferString(`{"name":""}`))
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok"})
		srv.auth.Middleware(http.HandlerFunc(srv.handleCreateAPIKey)).ServeHTTP(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422", rr.Code)
		}
	})

	t.Run("success returns plaintext key in response", func(t *testing.T) {
		now := time.Now()
		user := &db.User{ID: "u1", Username: "alice", Role: "admin"}
		store := &createAPIKeyStore{
			apiKeyAuthStore: &apiKeyAuthStore{
				stubStore: &stubStore{},
				session:   &db.Session{Token: "tok", UserID: "u1", ExpiresAt: now.Add(time.Hour)},
				user:      user,
			},
			createdKey: &db.APIKey{ID: "key-1", UserID: "u1", Name: "ci-token", CreatedAt: now},
		}
		srv := newAPIKeyTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/apikeys",
			bytes.NewBufferString(`{"name":"ci-token"}`))
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok"})
		srv.auth.Middleware(http.HandlerFunc(srv.handleCreateAPIKey)).ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201", rr.Code)
		}
		var body struct {
			Data map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data["key"] == nil || body.Data["key"] == "" {
			t.Error("expected plaintext key in data.key")
		}
		if body.Data["id"] != "key-1" {
			t.Errorf("data.id = %v, want key-1", body.Data["id"])
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleListAPIKeys
// ---------------------------------------------------------------------------

func TestHandleListAPIKeys(t *testing.T) {
	t.Run("no claims returns 401", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/apikeys", nil)
		srv.handleListAPIKeys(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("success returns list without key_hash", func(t *testing.T) {
		now := time.Now()
		user := &db.User{ID: "u1", Username: "alice", Role: "admin"}
		store := &listAPIKeysStore{
			apiKeyAuthStore: &apiKeyAuthStore{
				stubStore: &stubStore{},
				session:   &db.Session{Token: "tok", UserID: "u1", ExpiresAt: now.Add(time.Hour)},
				user:      user,
			},
			keys: []*db.APIKey{
				{ID: "k1", UserID: "u1", Name: "ci", CreatedAt: now},
				{ID: "k2", UserID: "u1", Name: "local", CreatedAt: now},
			},
		}
		srv := newAPIKeyTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/apikeys", nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok"})
		srv.auth.Middleware(http.HandlerFunc(srv.handleListAPIKeys)).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		var body struct {
			Data []map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if len(body.Data) != 2 {
			t.Errorf("len(data) = %d, want 2", len(body.Data))
		}
		// plaintext key must not be present in any listed key.
		for i, k := range body.Data {
			if _, hasKey := k["key"]; hasKey {
				t.Errorf("data[%d] contains key field — plaintext must not be listed", i)
			}
		}
	})

	t.Run("nil keys returns empty array", func(t *testing.T) {
		now := time.Now()
		user := &db.User{ID: "u1", Username: "alice", Role: "admin"}
		store := &listAPIKeysStore{
			apiKeyAuthStore: &apiKeyAuthStore{
				stubStore: &stubStore{},
				session:   &db.Session{Token: "tok", UserID: "u1", ExpiresAt: now.Add(time.Hour)},
				user:      user,
			},
			keys: nil,
		}
		srv := newAPIKeyTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/apikeys", nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok"})
		srv.auth.Middleware(http.HandlerFunc(srv.handleListAPIKeys)).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		var body struct {
			Data []map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data == nil {
			t.Error("expected non-nil (empty) data array, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleDeleteAPIKey
// ---------------------------------------------------------------------------

func TestHandleDeleteAPIKey(t *testing.T) {
	t.Run("no claims returns 401", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/apikeys/k1", nil)
		req.SetPathValue("id", "k1")
		srv.handleDeleteAPIKey(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("missing id returns 400", func(t *testing.T) {
		now := time.Now()
		user := &db.User{ID: "u1", Username: "alice", Role: "admin"}
		store := &deleteAPIKeyStore{
			apiKeyAuthStore: &apiKeyAuthStore{
				stubStore: &stubStore{},
				session:   &db.Session{Token: "tok", UserID: "u1", ExpiresAt: now.Add(time.Hour)},
				user:      user,
			},
			keys: []*db.APIKey{{ID: "k1", UserID: "u1"}},
		}
		srv := newAPIKeyTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/apikeys/", nil)
		// no path value set
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok"})
		srv.auth.Middleware(http.HandlerFunc(srv.handleDeleteAPIKey)).ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("key not owned by user returns 404", func(t *testing.T) {
		now := time.Now()
		user := &db.User{ID: "u1", Username: "alice", Role: "admin"}
		store := &deleteAPIKeyStore{
			apiKeyAuthStore: &apiKeyAuthStore{
				stubStore: &stubStore{},
				session:   &db.Session{Token: "tok", UserID: "u1", ExpiresAt: now.Add(time.Hour)},
				user:      user,
			},
			// User u1 has no keys.
			keys: []*db.APIKey{},
		}
		srv := newAPIKeyTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/apikeys/k-other", nil)
		req.SetPathValue("id", "k-other")
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok"})
		srv.auth.Middleware(http.HandlerFunc(srv.handleDeleteAPIKey)).ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})

	t.Run("success deletes owned key", func(t *testing.T) {
		now := time.Now()
		user := &db.User{ID: "u1", Username: "alice", Role: "admin"}
		store := &deleteAPIKeyStore{
			apiKeyAuthStore: &apiKeyAuthStore{
				stubStore: &stubStore{},
				session:   &db.Session{Token: "tok", UserID: "u1", ExpiresAt: now.Add(time.Hour)},
				user:      user,
			},
			keys: []*db.APIKey{{ID: "k1", UserID: "u1", Name: "ci"}},
		}
		srv := newAPIKeyTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/apikeys/k1", nil)
		req.SetPathValue("id", "k1")
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok"})
		srv.auth.Middleware(http.HandlerFunc(srv.handleDeleteAPIKey)).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		var body struct {
			Data map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data["ok"] != true {
			t.Error("expected data.ok = true")
		}
	})
}

// ---------------------------------------------------------------------------
// Store stubs
// ---------------------------------------------------------------------------

type createAPIKeyStore struct {
	*apiKeyAuthStore
	createdKey *db.APIKey
}

func (s *createAPIKeyStore) GetSessionByToken(ctx context.Context, tok string) (*db.Session, error) {
	return s.apiKeyAuthStore.GetSessionByToken(ctx, tok)
}

func (s *createAPIKeyStore) GetUserByID(ctx context.Context, id string) (*db.User, error) {
	return s.apiKeyAuthStore.GetUserByID(ctx, id)
}

func (s *createAPIKeyStore) CreateAPIKey(_ context.Context, _ db.CreateAPIKeyParams) (*db.APIKey, error) {
	if s.createdKey != nil {
		return s.createdKey, nil
	}
	return &db.APIKey{ID: "gen-id", UserID: "u1", Name: "gen"}, nil
}

type listAPIKeysStore struct {
	*apiKeyAuthStore
	keys []*db.APIKey
}

func (s *listAPIKeysStore) GetSessionByToken(ctx context.Context, tok string) (*db.Session, error) {
	return s.apiKeyAuthStore.GetSessionByToken(ctx, tok)
}

func (s *listAPIKeysStore) GetUserByID(ctx context.Context, id string) (*db.User, error) {
	return s.apiKeyAuthStore.GetUserByID(ctx, id)
}

func (s *listAPIKeysStore) ListAPIKeysByUser(_ context.Context, _ string) ([]*db.APIKey, error) {
	return s.keys, nil
}

type deleteAPIKeyStore struct {
	*apiKeyAuthStore
	keys    []*db.APIKey
	deleted string
}

func (s *deleteAPIKeyStore) GetSessionByToken(ctx context.Context, tok string) (*db.Session, error) {
	return s.apiKeyAuthStore.GetSessionByToken(ctx, tok)
}

func (s *deleteAPIKeyStore) GetUserByID(ctx context.Context, id string) (*db.User, error) {
	return s.apiKeyAuthStore.GetUserByID(ctx, id)
}

func (s *deleteAPIKeyStore) ListAPIKeysByUser(_ context.Context, _ string) ([]*db.APIKey, error) {
	return s.keys, nil
}

func (s *deleteAPIKeyStore) DeleteAPIKey(_ context.Context, id string) error {
	s.deleted = id
	return nil
}

package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/badskater/distributed-encoder/internal/controller/auth"
	"github.com/badskater/distributed-encoder/internal/db"
)

// ---------------------------------------------------------------------------
// newNotificationsTestServer creates a Server with an auth service backed by
// the provided store, so the session middleware and handler share the same store.
// ---------------------------------------------------------------------------

func newNotificationsTestServer(store db.Store) *Server {
	return newServerWithAuth(store, store)
}

// notifAuthStore is a base session/user store shared by notification test stubs.
type notifAuthStore struct {
	*stubStore
	session *db.Session
	user    *db.User
}

func (s *notifAuthStore) GetSessionByToken(_ context.Context, _ string) (*db.Session, error) {
	return s.session, nil
}

func (s *notifAuthStore) GetUserByID(_ context.Context, _ string) (*db.User, error) {
	return s.user, nil
}

// ---------------------------------------------------------------------------
// TestHandleGetNotificationPrefs
// ---------------------------------------------------------------------------

func TestHandleGetNotificationPrefs(t *testing.T) {
	t.Run("no claims returns 401", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me/notifications", nil)
		srv.handleGetNotificationPrefs(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("no row returns defaults", func(t *testing.T) {
		now := time.Now()
		user := &db.User{ID: "u1", Username: "alice", Role: "admin"}
		store := &getNotifPrefsNoRowStore{
			notifAuthStore: &notifAuthStore{
				stubStore: &stubStore{},
				session:   &db.Session{Token: "tok", UserID: "u1", ExpiresAt: now.Add(time.Hour)},
				user:      user,
			},
		}
		srv := newNotificationsTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me/notifications", nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok"})
		srv.auth.Middleware(http.HandlerFunc(srv.handleGetNotificationPrefs)).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		var body struct {
			Data db.NotificationPrefs `json:"data"`
		}
		decodeJSON(t, rr, &body)
		// Defaults: job_complete=true, job_failed=true, agent_stale=false
		if !body.Data.NotifyOnJobComplete {
			t.Error("default NotifyOnJobComplete = false, want true")
		}
		if !body.Data.NotifyOnJobFailed {
			t.Error("default NotifyOnJobFailed = false, want true")
		}
		if body.Data.NotifyOnAgentStale {
			t.Error("default NotifyOnAgentStale = true, want false")
		}
	})

	t.Run("existing prefs are returned", func(t *testing.T) {
		now := time.Now()
		user := &db.User{ID: "u1", Username: "alice", Role: "admin"}
		prefs := &db.NotificationPrefs{
			ID:                  "np1",
			UserID:              "u1",
			NotifyOnJobComplete: false,
			NotifyOnJobFailed:   true,
			NotifyOnAgentStale:  true,
		}
		store := &getNotifPrefsExistingStore{
			notifAuthStore: &notifAuthStore{
				stubStore: &stubStore{},
				session:   &db.Session{Token: "tok", UserID: "u1", ExpiresAt: now.Add(time.Hour)},
				user:      user,
			},
			prefs: prefs,
		}
		srv := newNotificationsTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me/notifications", nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok"})
		srv.auth.Middleware(http.HandlerFunc(srv.handleGetNotificationPrefs)).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		var body struct {
			Data db.NotificationPrefs `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data.NotifyOnAgentStale != true {
			t.Errorf("NotifyOnAgentStale = %v, want true", body.Data.NotifyOnAgentStale)
		}
		if body.Data.NotifyOnJobComplete != false {
			t.Errorf("NotifyOnJobComplete = %v, want false", body.Data.NotifyOnJobComplete)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		now := time.Now()
		user := &db.User{ID: "u1", Username: "alice", Role: "admin"}
		store := &getNotifPrefsErrStore{
			notifAuthStore: &notifAuthStore{
				stubStore: &stubStore{},
				session:   &db.Session{Token: "tok", UserID: "u1", ExpiresAt: now.Add(time.Hour)},
				user:      user,
			},
		}
		srv := newNotificationsTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me/notifications", nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok"})
		srv.auth.Middleware(http.HandlerFunc(srv.handleGetNotificationPrefs)).ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rr.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleUpdateNotificationPrefs
// ---------------------------------------------------------------------------

func TestHandleUpdateNotificationPrefs(t *testing.T) {
	t.Run("no claims returns 401", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/me/notifications",
			bytes.NewBufferString(`{"notify_on_job_complete":true}`))
		srv.handleUpdateNotificationPrefs(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		now := time.Now()
		user := &db.User{ID: "u1", Username: "alice", Role: "admin"}
		store := &updateNotifPrefsStore{
			notifAuthStore: &notifAuthStore{
				stubStore: &stubStore{},
				session:   &db.Session{Token: "tok", UserID: "u1", ExpiresAt: now.Add(time.Hour)},
				user:      user,
			},
		}
		srv := newNotificationsTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/me/notifications",
			bytes.NewBufferString(`{not json`))
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok"})
		srv.auth.Middleware(http.HandlerFunc(srv.handleUpdateNotificationPrefs)).ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("success updates and returns saved prefs", func(t *testing.T) {
		now := time.Now()
		user := &db.User{ID: "u1", Username: "alice", Role: "admin"}
		savedPrefs := &db.NotificationPrefs{
			ID:                  "np1",
			UserID:              "u1",
			NotifyOnJobComplete: true,
			NotifyOnJobFailed:   false,
			NotifyOnAgentStale:  true,
		}
		store := &updateNotifPrefsStore{
			notifAuthStore: &notifAuthStore{
				stubStore: &stubStore{},
				session:   &db.Session{Token: "tok", UserID: "u1", ExpiresAt: now.Add(time.Hour)},
				user:      user,
			},
			savedPrefs: savedPrefs,
		}
		srv := newNotificationsTestServer(store)

		body := `{"notify_on_job_complete":true,"notify_on_job_failed":false,"notify_on_agent_stale":true}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/me/notifications",
			bytes.NewBufferString(body))
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok"})
		srv.auth.Middleware(http.HandlerFunc(srv.handleUpdateNotificationPrefs)).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		var resp struct {
			Data db.NotificationPrefs `json:"data"`
		}
		decodeJSON(t, rr, &resp)
		if !resp.Data.NotifyOnJobComplete {
			t.Error("NotifyOnJobComplete = false, want true")
		}
		if resp.Data.NotifyOnJobFailed {
			t.Error("NotifyOnJobFailed = true, want false")
		}
	})

	t.Run("upsert store error returns 500", func(t *testing.T) {
		now := time.Now()
		user := &db.User{ID: "u1", Username: "alice", Role: "admin"}
		store := &updateNotifPrefsErrStore{
			notifAuthStore: &notifAuthStore{
				stubStore: &stubStore{},
				session:   &db.Session{Token: "tok", UserID: "u1", ExpiresAt: now.Add(time.Hour)},
				user:      user,
			},
		}
		srv := newNotificationsTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/me/notifications",
			bytes.NewBufferString(`{"notify_on_job_complete":true}`))
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tok"})
		srv.auth.Middleware(http.HandlerFunc(srv.handleUpdateNotificationPrefs)).ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rr.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// Store stubs
// ---------------------------------------------------------------------------

type getNotifPrefsNoRowStore struct {
	*notifAuthStore
}

func (s *getNotifPrefsNoRowStore) GetSessionByToken(ctx context.Context, tok string) (*db.Session, error) {
	return s.notifAuthStore.GetSessionByToken(ctx, tok)
}

func (s *getNotifPrefsNoRowStore) GetUserByID(ctx context.Context, id string) (*db.User, error) {
	return s.notifAuthStore.GetUserByID(ctx, id)
}

func (s *getNotifPrefsNoRowStore) GetNotificationPrefs(_ context.Context, _ string) (*db.NotificationPrefs, error) {
	return nil, db.ErrNotFound
}

type getNotifPrefsExistingStore struct {
	*notifAuthStore
	prefs *db.NotificationPrefs
}

func (s *getNotifPrefsExistingStore) GetSessionByToken(ctx context.Context, tok string) (*db.Session, error) {
	return s.notifAuthStore.GetSessionByToken(ctx, tok)
}

func (s *getNotifPrefsExistingStore) GetUserByID(ctx context.Context, id string) (*db.User, error) {
	return s.notifAuthStore.GetUserByID(ctx, id)
}

func (s *getNotifPrefsExistingStore) GetNotificationPrefs(_ context.Context, _ string) (*db.NotificationPrefs, error) {
	return s.prefs, nil
}

type getNotifPrefsErrStore struct {
	*notifAuthStore
}

func (s *getNotifPrefsErrStore) GetSessionByToken(ctx context.Context, tok string) (*db.Session, error) {
	return s.notifAuthStore.GetSessionByToken(ctx, tok)
}

func (s *getNotifPrefsErrStore) GetUserByID(ctx context.Context, id string) (*db.User, error) {
	return s.notifAuthStore.GetUserByID(ctx, id)
}

func (s *getNotifPrefsErrStore) GetNotificationPrefs(_ context.Context, _ string) (*db.NotificationPrefs, error) {
	return nil, errTestDB
}

type updateNotifPrefsStore struct {
	*notifAuthStore
	savedPrefs *db.NotificationPrefs
}

func (s *updateNotifPrefsStore) GetSessionByToken(ctx context.Context, tok string) (*db.Session, error) {
	return s.notifAuthStore.GetSessionByToken(ctx, tok)
}

func (s *updateNotifPrefsStore) GetUserByID(ctx context.Context, id string) (*db.User, error) {
	return s.notifAuthStore.GetUserByID(ctx, id)
}

func (s *updateNotifPrefsStore) UpsertNotificationPrefs(_ context.Context, _ db.UpsertNotificationPrefsParams) error {
	return nil
}

func (s *updateNotifPrefsStore) GetNotificationPrefs(_ context.Context, _ string) (*db.NotificationPrefs, error) {
	if s.savedPrefs != nil {
		return s.savedPrefs, nil
	}
	return &db.NotificationPrefs{UserID: "u1"}, nil
}

type updateNotifPrefsErrStore struct {
	*notifAuthStore
}

func (s *updateNotifPrefsErrStore) GetSessionByToken(ctx context.Context, tok string) (*db.Session, error) {
	return s.notifAuthStore.GetSessionByToken(ctx, tok)
}

func (s *updateNotifPrefsErrStore) GetUserByID(ctx context.Context, id string) (*db.User, error) {
	return s.notifAuthStore.GetUserByID(ctx, id)
}

func (s *updateNotifPrefsErrStore) UpsertNotificationPrefs(_ context.Context, _ db.UpsertNotificationPrefsParams) error {
	return errTestDB
}

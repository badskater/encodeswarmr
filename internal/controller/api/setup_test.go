package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/db"
)

// ---------------------------------------------------------------------------
// TestHandleSetupStatus
// ---------------------------------------------------------------------------

func TestHandleSetupStatus(t *testing.T) {
	// Reset the in-process atomic flag before each run so tests don't affect
	// each other.  setupDone is a package-level atomic.Bool in setup.go.

	t.Run("required when no admin users exist", func(t *testing.T) {
		setupDone.Store(false)
		store := &setupStatusStore{stubStore: &stubStore{}, adminCount: 0}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/setup/status", nil)
		srv.handleSetupStatus(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data["required"] != true {
			t.Errorf("data.required = %v, want true", body.Data["required"])
		}
	})

	t.Run("not required when admin users exist", func(t *testing.T) {
		setupDone.Store(false)
		store := &setupStatusStore{stubStore: &stubStore{}, adminCount: 1}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/setup/status", nil)
		srv.handleSetupStatus(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data["required"] != false {
			t.Errorf("data.required = %v, want false", body.Data["required"])
		}
	})

	t.Run("returns false immediately when setupDone is true", func(t *testing.T) {
		setupDone.Store(true)
		// Store error would be reached if the DB is queried, but it should not be.
		store := &setupStatusErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/setup/status", nil)
		srv.handleSetupStatus(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data["required"] != false {
			t.Errorf("data.required = %v, want false when setupDone=true", body.Data["required"])
		}
		// Reset for subsequent tests.
		setupDone.Store(false)
	})

	t.Run("store error returns 500", func(t *testing.T) {
		setupDone.Store(false)
		store := &setupStatusErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/setup/status", nil)
		srv.handleSetupStatus(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleSetup
// ---------------------------------------------------------------------------

func TestHandleSetup(t *testing.T) {
	t.Run("store error on count returns 500", func(t *testing.T) {
		setupDone.Store(false)
		store := &setupStatusErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/setup",
			bytes.NewBufferString(`{"username":"admin","email":"admin@example.com","password":"secret12"}`))
		srv.handleSetup(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("already completed returns 409", func(t *testing.T) {
		setupDone.Store(false)
		store := &setupStatusStore{stubStore: &stubStore{}, adminCount: 1}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/setup",
			bytes.NewBufferString(`{"username":"admin","email":"admin@example.com","password":"secret12"}`))
		srv.handleSetup(rr, req)

		if rr.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusConflict)
		}
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		setupDone.Store(false)
		store := &setupStatusStore{stubStore: &stubStore{}, adminCount: 0}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/setup",
			bytes.NewBufferString(`{not json`))
		srv.handleSetup(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing username returns 422", func(t *testing.T) {
		setupDone.Store(false)
		store := &setupStatusStore{stubStore: &stubStore{}, adminCount: 0}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/setup",
			bytes.NewBufferString(`{"email":"a@b.com","password":"secret12"}`))
		srv.handleSetup(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("missing email returns 422", func(t *testing.T) {
		setupDone.Store(false)
		store := &setupStatusStore{stubStore: &stubStore{}, adminCount: 0}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/setup",
			bytes.NewBufferString(`{"username":"admin","password":"secret12"}`))
		srv.handleSetup(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("missing password returns 422", func(t *testing.T) {
		setupDone.Store(false)
		store := &setupStatusStore{stubStore: &stubStore{}, adminCount: 0}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/setup",
			bytes.NewBufferString(`{"username":"admin","email":"a@b.com"}`))
		srv.handleSetup(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("password too short returns 422", func(t *testing.T) {
		setupDone.Store(false)
		store := &setupStatusStore{stubStore: &stubStore{}, adminCount: 0}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/setup",
			bytes.NewBufferString(`{"username":"admin","email":"a@b.com","password":"short"}`))
		srv.handleSetup(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("store error on create user returns 500", func(t *testing.T) {
		setupDone.Store(false)
		store := &setupCreateErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/setup",
			bytes.NewBufferString(`{"username":"admin","email":"admin@example.com","password":"secret12"}`))
		srv.handleSetup(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
		setupDone.Store(false)
	})

	t.Run("success creates admin user", func(t *testing.T) {
		setupDone.Store(false)
		now := time.Now()
		store := &setupSuccessStore{
			stubStore: &stubStore{},
			user: &db.User{
				ID:        "u1",
				Username:  "admin",
				Email:     "admin@example.com",
				Role:      "admin",
				CreatedAt: now,
				UpdatedAt: now,
			},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/setup",
			bytes.NewBufferString(`{"username":"admin","email":"admin@example.com","password":"secret12"}`))
		srv.handleSetup(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
		}
		var body struct {
			Data map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data["username"] != "admin" {
			t.Errorf("data.username = %v, want admin", body.Data["username"])
		}
		if body.Data["role"] != "admin" {
			t.Errorf("data.role = %v, want admin", body.Data["role"])
		}
		// Verify setupDone flag was set.
		if !setupDone.Load() {
			t.Error("expected setupDone to be true after successful setup")
		}
		// Reset for subsequent tests.
		setupDone.Store(false)
	})
}

// ---------------------------------------------------------------------------
// store stubs
// ---------------------------------------------------------------------------

type setupStatusStore struct {
	*stubStore
	adminCount int64
}

func (s *setupStatusStore) CountAdminUsers(_ context.Context) (int64, error) {
	return s.adminCount, nil
}

type setupStatusErrStore struct{ *stubStore }

func (s *setupStatusErrStore) CountAdminUsers(_ context.Context) (int64, error) {
	return 0, errTestDB
}

type setupCreateErrStore struct{ *stubStore }

func (s *setupCreateErrStore) CountAdminUsers(_ context.Context) (int64, error) {
	return 0, nil
}

func (s *setupCreateErrStore) CreateUser(_ context.Context, _ db.CreateUserParams) (*db.User, error) {
	return nil, errTestDB
}

type setupSuccessStore struct {
	*stubStore
	user *db.User
}

func (s *setupSuccessStore) CountAdminUsers(_ context.Context) (int64, error) {
	return 0, nil
}

func (s *setupSuccessStore) CreateUser(_ context.Context, _ db.CreateUserParams) (*db.User, error) {
	return s.user, nil
}

package api

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/db"
)

// ---------------------------------------------------------------------------
// handleListUsers
// ---------------------------------------------------------------------------

func TestHandleListUsers_Success(t *testing.T) {
	store := &listUsersStore{
		stubStore: &stubStore{},
		users: []*db.User{
			{ID: "u1", Username: "alice", Email: "alice@example.com", Role: "admin", CreatedAt: time.Now(), UpdatedAt: time.Now()},
			{ID: "u2", Username: "bob", Email: "bob@example.com", Role: "viewer", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	srv.handleListUsers(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleListUsers_StoreError(t *testing.T) {
	store := &listUsersErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	srv.handleListUsers(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestHandleListUsers_Empty(t *testing.T) {
	store := &listUsersStore{
		stubStore: &stubStore{},
		users:     []*db.User{},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	srv.handleListUsers(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

// ---------------------------------------------------------------------------
// handleCreateUser
// ---------------------------------------------------------------------------

func TestHandleCreateUser_Success(t *testing.T) {
	store := &createUserStore{
		stubStore: &stubStore{},
		user:      &db.User{ID: "u-new", Username: "carol", Email: "carol@example.com", Role: "operator", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	srv := newTestServer(store)

	body := `{"username":"carol","email":"carol@example.com","role":"operator","password":"secret123"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(body))
	srv.handleCreateUser(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
}

func TestHandleCreateUser_NoPassword(t *testing.T) {
	// Creating a user without a password is valid (e.g. OIDC users).
	store := &createUserStore{
		stubStore: &stubStore{},
		user:      &db.User{ID: "u-oidc", Username: "dave", Email: "dave@example.com", Role: "viewer", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	srv := newTestServer(store)

	body := `{"username":"dave","email":"dave@example.com","role":"viewer"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(body))
	srv.handleCreateUser(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
}

func TestHandleCreateUser_InvalidJSON(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString("bad"))
	srv.handleCreateUser(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateUser_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"missing username", `{"email":"a@b.com","role":"viewer"}`},
		{"missing email", `{"username":"a","role":"viewer"}`},
		{"missing role", `{"username":"a","email":"a@b.com"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServer(&stubStore{})
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(tc.body))
			srv.handleCreateUser(rr, req)
			if rr.Code != http.StatusUnprocessableEntity {
				t.Fatalf("%s: status = %d, want %d", tc.name, rr.Code, http.StatusUnprocessableEntity)
			}
		})
	}
}

func TestHandleCreateUser_InvalidRole(t *testing.T) {
	srv := newTestServer(&stubStore{})

	body := `{"username":"a","email":"a@b.com","role":"superuser"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(body))
	srv.handleCreateUser(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}
}

func TestHandleCreateUser_ValidRoles(t *testing.T) {
	for _, role := range []string{"viewer", "operator", "admin"} {
		t.Run("role="+role, func(t *testing.T) {
			store := &createUserStore{
				stubStore: &stubStore{},
				user:      &db.User{ID: "u-" + role, Username: "user-" + role, Email: "u@b.com", Role: role, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			}
			srv := newTestServer(store)
			body := `{"username":"user-` + role + `","email":"u@b.com","role":"` + role + `"}`
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(body))
			srv.handleCreateUser(rr, req)
			if rr.Code != http.StatusCreated {
				t.Errorf("role=%q: status = %d, want %d", role, rr.Code, http.StatusCreated)
			}
		})
	}
}

func TestHandleCreateUser_StoreError(t *testing.T) {
	store := &createUserErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"username":"a","email":"a@b.com","role":"viewer"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(body))
	srv.handleCreateUser(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleDeleteUser
// ---------------------------------------------------------------------------

func TestHandleDeleteUser_Success(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/u1", nil)
	req.SetPathValue("id", "u1")
	srv.handleDeleteUser(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestHandleDeleteUser_NotFound(t *testing.T) {
	store := &deleteUserNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/missing", nil)
	req.SetPathValue("id", "missing")
	srv.handleDeleteUser(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleDeleteUser_StoreError(t *testing.T) {
	store := &deleteUserErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/u1", nil)
	req.SetPathValue("id", "u1")
	srv.handleDeleteUser(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleUpdateUserRole
// ---------------------------------------------------------------------------

func TestHandleUpdateUserRole_Success(t *testing.T) {
	srv := newTestServer(&stubStore{})

	body := `{"role":"admin"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/u1/role", bytes.NewBufferString(body))
	req.SetPathValue("id", "u1")
	srv.handleUpdateUserRole(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleUpdateUserRole_InvalidJSON(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/u1/role", bytes.NewBufferString("bad"))
	req.SetPathValue("id", "u1")
	srv.handleUpdateUserRole(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateUserRole_InvalidRole(t *testing.T) {
	srv := newTestServer(&stubStore{})

	body := `{"role":"superadmin"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/u1/role", bytes.NewBufferString(body))
	req.SetPathValue("id", "u1")
	srv.handleUpdateUserRole(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}
}

func TestHandleUpdateUserRole_NotFound(t *testing.T) {
	store := &updateRoleNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"role":"viewer"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/missing/role", bytes.NewBufferString(body))
	req.SetPathValue("id", "missing")
	srv.handleUpdateUserRole(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleUpdateUserRole_StoreError(t *testing.T) {
	store := &updateRoleErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"role":"operator"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/u1/role", bytes.NewBufferString(body))
	req.SetPathValue("id", "u1")
	srv.handleUpdateUserRole(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// store stubs for users tests
// ---------------------------------------------------------------------------

type listUsersStore struct {
	*stubStore
	users []*db.User
}

func (s *listUsersStore) ListUsers(_ context.Context) ([]*db.User, error) {
	return s.users, nil
}

type listUsersErrStore struct{ *stubStore }

func (s *listUsersErrStore) ListUsers(_ context.Context) ([]*db.User, error) {
	return nil, errors.New("db failure")
}

type createUserStore struct {
	*stubStore
	user *db.User
}

func (s *createUserStore) CreateUser(_ context.Context, _ db.CreateUserParams) (*db.User, error) {
	return s.user, nil
}

type createUserErrStore struct{ *stubStore }

func (s *createUserErrStore) CreateUser(_ context.Context, _ db.CreateUserParams) (*db.User, error) {
	return nil, errors.New("db failure")
}

type deleteUserNotFoundStore struct{ *stubStore }

func (s *deleteUserNotFoundStore) DeleteUser(_ context.Context, _ string) error {
	return db.ErrNotFound
}

type deleteUserErrStore struct{ *stubStore }

func (s *deleteUserErrStore) DeleteUser(_ context.Context, _ string) error {
	return errors.New("db failure")
}

type updateRoleNotFoundStore struct{ *stubStore }

func (s *updateRoleNotFoundStore) UpdateUserRole(_ context.Context, _, _ string) error {
	return db.ErrNotFound
}

type updateRoleErrStore struct{ *stubStore }

func (s *updateRoleErrStore) UpdateUserRole(_ context.Context, _, _ string) error {
	return errors.New("db failure")
}

package api

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/badskater/distributed-encoder/internal/db"
)

// ---------------------------------------------------------------------------
// handleListPathMappings
// ---------------------------------------------------------------------------

func TestHandleListPathMappings_Success(t *testing.T) {
	store := &listPathMappingsStore{
		stubStore: &stubStore{},
		mappings: []*db.PathMapping{
			{ID: "pm1", Name: "NAS", WindowsPrefix: `\\NAS01\media`, LinuxPrefix: "/mnt/nas/media", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/path-mappings", nil)
	srv.handleListPathMappings(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleListPathMappings_StoreError(t *testing.T) {
	store := &listPathMappingsErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/path-mappings", nil)
	srv.handleListPathMappings(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleCreatePathMapping
// ---------------------------------------------------------------------------

func TestHandleCreatePathMapping_Success(t *testing.T) {
	store := &createPathMappingStore{
		stubStore: &stubStore{},
		mapping: &db.PathMapping{
			ID: "pm-new", Name: "NAS2", WindowsPrefix: `\\NAS02\video`, LinuxPrefix: "/mnt/nas2/video",
			Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
	}
	srv := newTestServer(store)

	body := `{"name":"NAS2","windows_prefix":"\\\\NAS02\\video","linux_prefix":"/mnt/nas2/video"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/path-mappings", bytes.NewBufferString(body))
	srv.handleCreatePathMapping(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
}

func TestHandleCreatePathMapping_InvalidJSON(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/path-mappings", bytes.NewBufferString("bad"))
	srv.handleCreatePathMapping(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleCreatePathMapping_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"missing name", `{"windows_prefix":"\\\\NAS\\x","linux_prefix":"/mnt/x"}`},
		{"missing windows_prefix", `{"name":"N","linux_prefix":"/mnt/x"}`},
		{"missing linux_prefix", `{"name":"N","windows_prefix":"\\\\NAS\\x"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServer(&stubStore{})
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/path-mappings", bytes.NewBufferString(tc.body))
			srv.handleCreatePathMapping(rr, req)
			if rr.Code != http.StatusUnprocessableEntity {
				t.Fatalf("%s: status = %d, want %d", tc.name, rr.Code, http.StatusUnprocessableEntity)
			}
		})
	}
}

func TestHandleCreatePathMapping_StoreError(t *testing.T) {
	store := &createPathMappingErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"name":"N","windows_prefix":"\\\\NAS\\x","linux_prefix":"/mnt/x"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/path-mappings", bytes.NewBufferString(body))
	srv.handleCreatePathMapping(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleGetPathMapping
// ---------------------------------------------------------------------------

func TestHandleGetPathMapping_Success(t *testing.T) {
	store := &getPathMappingStore{
		stubStore: &stubStore{},
		mapping: &db.PathMapping{
			ID: "pm1", Name: "NAS", WindowsPrefix: `\\NAS01\media`, LinuxPrefix: "/mnt/nas/media",
			Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/path-mappings/pm1", nil)
	req.SetPathValue("id", "pm1")
	srv.handleGetPathMapping(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleGetPathMapping_NotFound(t *testing.T) {
	store := &getPathMappingNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/path-mappings/missing", nil)
	req.SetPathValue("id", "missing")
	srv.handleGetPathMapping(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleGetPathMapping_StoreError(t *testing.T) {
	store := &getPathMappingErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/path-mappings/pm1", nil)
	req.SetPathValue("id", "pm1")
	srv.handleGetPathMapping(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleUpdatePathMapping
// ---------------------------------------------------------------------------

func TestHandleUpdatePathMapping_Success(t *testing.T) {
	store := &updatePathMappingStore{
		stubStore: &stubStore{},
		mapping: &db.PathMapping{
			ID: "pm1", Name: "Updated", WindowsPrefix: `\\NAS01\new`, LinuxPrefix: "/mnt/new",
			Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
	}
	srv := newTestServer(store)

	body := `{"name":"Updated","windows_prefix":"\\\\NAS01\\new","linux_prefix":"/mnt/new","enabled":true}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/path-mappings/pm1", bytes.NewBufferString(body))
	req.SetPathValue("id", "pm1")
	srv.handleUpdatePathMapping(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleUpdatePathMapping_InvalidJSON(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/path-mappings/pm1", bytes.NewBufferString("bad"))
	req.SetPathValue("id", "pm1")
	srv.handleUpdatePathMapping(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdatePathMapping_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"missing name", `{"windows_prefix":"\\\\NAS\\x","linux_prefix":"/mnt/x","enabled":true}`},
		{"missing windows_prefix", `{"name":"N","linux_prefix":"/mnt/x","enabled":true}`},
		{"missing linux_prefix", `{"name":"N","windows_prefix":"\\\\NAS\\x","enabled":true}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServer(&stubStore{})
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/api/v1/path-mappings/pm1", bytes.NewBufferString(tc.body))
			req.SetPathValue("id", "pm1")
			srv.handleUpdatePathMapping(rr, req)
			if rr.Code != http.StatusUnprocessableEntity {
				t.Fatalf("%s: status = %d, want %d", tc.name, rr.Code, http.StatusUnprocessableEntity)
			}
		})
	}
}

func TestHandleUpdatePathMapping_NotFound(t *testing.T) {
	store := &updatePathMappingNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"name":"N","windows_prefix":"\\\\NAS\\x","linux_prefix":"/mnt/x","enabled":true}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/path-mappings/missing", bytes.NewBufferString(body))
	req.SetPathValue("id", "missing")
	srv.handleUpdatePathMapping(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleUpdatePathMapping_StoreError(t *testing.T) {
	store := &updatePathMappingErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"name":"N","windows_prefix":"\\\\NAS\\x","linux_prefix":"/mnt/x","enabled":true}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/path-mappings/pm1", bytes.NewBufferString(body))
	req.SetPathValue("id", "pm1")
	srv.handleUpdatePathMapping(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleDeletePathMapping
// ---------------------------------------------------------------------------

func TestHandleDeletePathMapping_Success(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/path-mappings/pm1", nil)
	req.SetPathValue("id", "pm1")
	srv.handleDeletePathMapping(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestHandleDeletePathMapping_NotFound(t *testing.T) {
	store := &deletePathMappingNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/path-mappings/missing", nil)
	req.SetPathValue("id", "missing")
	srv.handleDeletePathMapping(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleDeletePathMapping_StoreError(t *testing.T) {
	store := &deletePathMappingErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/path-mappings/pm1", nil)
	req.SetPathValue("id", "pm1")
	srv.handleDeletePathMapping(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// store stubs for path mapping tests
// ---------------------------------------------------------------------------

type listPathMappingsStore struct {
	*stubStore
	mappings []*db.PathMapping
}

func (s *listPathMappingsStore) ListPathMappings(_ context.Context) ([]*db.PathMapping, error) {
	return s.mappings, nil
}

type listPathMappingsErrStore struct{ *stubStore }

func (s *listPathMappingsErrStore) ListPathMappings(_ context.Context) ([]*db.PathMapping, error) {
	return nil, errors.New("db failure")
}

type createPathMappingStore struct {
	*stubStore
	mapping *db.PathMapping
}

func (s *createPathMappingStore) CreatePathMapping(_ context.Context, _ db.CreatePathMappingParams) (*db.PathMapping, error) {
	return s.mapping, nil
}

type createPathMappingErrStore struct{ *stubStore }

func (s *createPathMappingErrStore) CreatePathMapping(_ context.Context, _ db.CreatePathMappingParams) (*db.PathMapping, error) {
	return nil, errors.New("db failure")
}

type getPathMappingStore struct {
	*stubStore
	mapping *db.PathMapping
}

func (s *getPathMappingStore) GetPathMappingByID(_ context.Context, _ string) (*db.PathMapping, error) {
	return s.mapping, nil
}

type getPathMappingNotFoundStore struct{ *stubStore }

func (s *getPathMappingNotFoundStore) GetPathMappingByID(_ context.Context, _ string) (*db.PathMapping, error) {
	return nil, db.ErrNotFound
}

type getPathMappingErrStore struct{ *stubStore }

func (s *getPathMappingErrStore) GetPathMappingByID(_ context.Context, _ string) (*db.PathMapping, error) {
	return nil, errors.New("db failure")
}

type updatePathMappingStore struct {
	*stubStore
	mapping *db.PathMapping
}

func (s *updatePathMappingStore) UpdatePathMapping(_ context.Context, _ db.UpdatePathMappingParams) (*db.PathMapping, error) {
	return s.mapping, nil
}

type updatePathMappingNotFoundStore struct{ *stubStore }

func (s *updatePathMappingNotFoundStore) UpdatePathMapping(_ context.Context, _ db.UpdatePathMappingParams) (*db.PathMapping, error) {
	return nil, db.ErrNotFound
}

type updatePathMappingErrStore struct{ *stubStore }

func (s *updatePathMappingErrStore) UpdatePathMapping(_ context.Context, _ db.UpdatePathMappingParams) (*db.PathMapping, error) {
	return nil, errors.New("db failure")
}

type deletePathMappingNotFoundStore struct{ *stubStore }

func (s *deletePathMappingNotFoundStore) DeletePathMapping(_ context.Context, _ string) error {
	return db.ErrNotFound
}

type deletePathMappingErrStore struct{ *stubStore }

func (s *deletePathMappingErrStore) DeletePathMapping(_ context.Context, _ string) error {
	return errors.New("db failure")
}

package api

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/badskater/distributed-encoder/internal/db"
)

// ---------------------------------------------------------------------------
// handleListVariables
// ---------------------------------------------------------------------------

func TestHandleListVariables_Success(t *testing.T) {
	store := &listVariablesStore{
		stubStore: &stubStore{},
		vars: []*db.Variable{
			{ID: "v1", Name: "FFMPEG_PRESET", Value: "slow"},
			{ID: "v2", Name: "OUTPUT_EXT", Value: "mkv"},
		},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/variables", nil)
	srv.handleListVariables(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleListVariables_CategoryFilter(t *testing.T) {
	store := &listVariablesCatStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/variables?category=encoding", nil)
	srv.handleListVariables(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if store.gotCategory != "encoding" {
		t.Errorf("ListVariables called with category = %q, want %q", store.gotCategory, "encoding")
	}
}

func TestHandleListVariables_StoreError(t *testing.T) {
	store := &listVariablesErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/variables", nil)
	srv.handleListVariables(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleGetVariable
// ---------------------------------------------------------------------------

func TestHandleGetVariable_Success(t *testing.T) {
	store := &getVariableStore{
		stubStore: &stubStore{},
		v:         &db.Variable{ID: "v1", Name: "FFMPEG_PRESET", Value: "slow"},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/variables/FFMPEG_PRESET", nil)
	req.SetPathValue("name", "FFMPEG_PRESET")
	srv.handleGetVariable(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleGetVariable_NotFound(t *testing.T) {
	store := &getVariableNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/variables/MISSING", nil)
	req.SetPathValue("name", "MISSING")
	srv.handleGetVariable(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleGetVariable_StoreError(t *testing.T) {
	store := &getVariableErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/variables/V1", nil)
	req.SetPathValue("name", "V1")
	srv.handleGetVariable(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleUpsertVariable
// ---------------------------------------------------------------------------

func TestHandleUpsertVariable_Success(t *testing.T) {
	store := &upsertVariableStore{
		stubStore: &stubStore{},
		v:         &db.Variable{ID: "v1", Name: "FFMPEG_PRESET", Value: "veryslow"},
	}
	srv := newTestServer(store)

	body := `{"value":"veryslow","description":"preset","category":"encoding"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/variables/FFMPEG_PRESET", bytes.NewBufferString(body))
	req.SetPathValue("name", "FFMPEG_PRESET")
	srv.handleUpsertVariable(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleUpsertVariable_InvalidJSON(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/variables/V1", bytes.NewBufferString("bad"))
	req.SetPathValue("name", "V1")
	srv.handleUpsertVariable(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleUpsertVariable_StoreError(t *testing.T) {
	store := &upsertVariableErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"value":"x"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/variables/V1", bytes.NewBufferString(body))
	req.SetPathValue("name", "V1")
	srv.handleUpsertVariable(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleDeleteVariable
// ---------------------------------------------------------------------------

func TestHandleDeleteVariable_Success(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/variables/v1", nil)
	req.SetPathValue("id", "v1")
	srv.handleDeleteVariable(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestHandleDeleteVariable_NotFound(t *testing.T) {
	store := &deleteVariableNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/variables/missing", nil)
	req.SetPathValue("id", "missing")
	srv.handleDeleteVariable(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleDeleteVariable_StoreError(t *testing.T) {
	store := &deleteVariableErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/variables/v1", nil)
	req.SetPathValue("id", "v1")
	srv.handleDeleteVariable(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// store stubs for variables tests
// ---------------------------------------------------------------------------

type listVariablesStore struct {
	*stubStore
	vars []*db.Variable
}

func (s *listVariablesStore) ListVariables(_ context.Context, _ string) ([]*db.Variable, error) {
	return s.vars, nil
}

type listVariablesCatStore struct {
	*stubStore
	gotCategory string
}

func (s *listVariablesCatStore) ListVariables(_ context.Context, category string) ([]*db.Variable, error) {
	s.gotCategory = category
	return nil, nil
}

type listVariablesErrStore struct{ *stubStore }

func (s *listVariablesErrStore) ListVariables(_ context.Context, _ string) ([]*db.Variable, error) {
	return nil, errors.New("db failure")
}

type getVariableStore struct {
	*stubStore
	v *db.Variable
}

func (s *getVariableStore) GetVariableByName(_ context.Context, _ string) (*db.Variable, error) {
	return s.v, nil
}

type getVariableNotFoundStore struct{ *stubStore }

func (s *getVariableNotFoundStore) GetVariableByName(_ context.Context, _ string) (*db.Variable, error) {
	return nil, db.ErrNotFound
}

type getVariableErrStore struct{ *stubStore }

func (s *getVariableErrStore) GetVariableByName(_ context.Context, _ string) (*db.Variable, error) {
	return nil, errors.New("db failure")
}

type upsertVariableStore struct {
	*stubStore
	v *db.Variable
}

func (s *upsertVariableStore) UpsertVariable(_ context.Context, _ db.UpsertVariableParams) (*db.Variable, error) {
	return s.v, nil
}

type upsertVariableErrStore struct{ *stubStore }

func (s *upsertVariableErrStore) UpsertVariable(_ context.Context, _ db.UpsertVariableParams) (*db.Variable, error) {
	return nil, errors.New("db failure")
}

type deleteVariableNotFoundStore struct{ *stubStore }

func (s *deleteVariableNotFoundStore) DeleteVariable(_ context.Context, _ string) error {
	return db.ErrNotFound
}

type deleteVariableErrStore struct{ *stubStore }

func (s *deleteVariableErrStore) DeleteVariable(_ context.Context, _ string) error {
	return errors.New("db failure")
}

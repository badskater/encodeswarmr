package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/badskater/encodeswarmr/internal/db"
)

// ---------------------------------------------------------------------------
// handleListTemplates
// ---------------------------------------------------------------------------

func TestHandleListTemplates_Success(t *testing.T) {
	store := &listTemplatesStore{
		stubStore: &stubStore{},
		templates: []*db.Template{
			{ID: "t1", Name: "AviSynth run", Type: "avs", Extension: "avs", Content: "..."},
			{ID: "t2", Name: "VapourSynth run", Type: "vpy", Extension: "vpy", Content: "..."},
		},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates", nil)
	srv.handleListTemplates(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body struct {
		Data []json.RawMessage `json:"data"`
	}
	decodeJSON(t, rr, &body)
	if len(body.Data) != 2 {
		t.Errorf("len(data) = %d, want 2", len(body.Data))
	}
}

func TestHandleListTemplates_StoreError(t *testing.T) {
	store := &listTemplatesErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates", nil)
	srv.handleListTemplates(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestHandleListTemplates_TypeFilter(t *testing.T) {
	store := &listTemplatesFilterStore{
		stubStore: &stubStore{},
		wantType:  "avs",
		templates: []*db.Template{
			{ID: "t1", Name: "AviSynth", Type: "avs", Extension: "avs", Content: "..."},
		},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates?type=avs", nil)
	srv.handleListTemplates(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if store.gotType != "avs" {
		t.Errorf("ListTemplates called with type = %q, want %q", store.gotType, "avs")
	}
}

// ---------------------------------------------------------------------------
// handleGetTemplate
// ---------------------------------------------------------------------------

func TestHandleGetTemplate_Success(t *testing.T) {
	store := &getTemplateStore{
		stubStore: &stubStore{},
		tmpl:      &db.Template{ID: "t1", Name: "Test", Type: "avs", Extension: "avs", Content: "# avs"},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates/t1", nil)
	req.SetPathValue("id", "t1")
	srv.handleGetTemplate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body struct {
		Data db.Template `json:"data"`
	}
	decodeJSON(t, rr, &body)
	if body.Data.ID != "t1" {
		t.Errorf("data.id = %q, want %q", body.Data.ID, "t1")
	}
}

func TestHandleGetTemplate_NotFound(t *testing.T) {
	store := &getTemplateNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates/missing", nil)
	req.SetPathValue("id", "missing")
	srv.handleGetTemplate(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleGetTemplate_StoreError(t *testing.T) {
	store := &getTemplateErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates/t1", nil)
	req.SetPathValue("id", "t1")
	srv.handleGetTemplate(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleCreateTemplate
// ---------------------------------------------------------------------------

func TestHandleCreateTemplate_Success(t *testing.T) {
	store := &createTemplateStore{
		stubStore: &stubStore{},
		created:   &db.Template{ID: "t-new", Name: "My Tmpl", Type: "avs", Extension: "avs", Content: "# avs"},
	}
	srv := newTestServer(store)

	body := `{"name":"My Tmpl","type":"avs","extension":"avs","content":"# avs"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/templates", bytes.NewBufferString(body))
	srv.handleCreateTemplate(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var resp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeJSON(t, rr, &resp)
	if resp.Data.ID != "t-new" {
		t.Errorf("data.id = %q, want %q", resp.Data.ID, "t-new")
	}
}

func TestHandleCreateTemplate_InvalidJSON(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/templates", bytes.NewBufferString("not-json"))
	srv.handleCreateTemplate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateTemplate_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"missing name", `{"type":"avs","extension":"avs","content":"# avs"}`},
		{"missing type", `{"name":"T","extension":"avs","content":"# avs"}`},
		{"missing extension", `{"name":"T","type":"avs","content":"# avs"}`},
		{"missing content", `{"name":"T","type":"avs","extension":"avs"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServer(&stubStore{})
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/templates", bytes.NewBufferString(tc.body))
			srv.handleCreateTemplate(rr, req)
			if rr.Code != http.StatusUnprocessableEntity {
				t.Fatalf("%s: status = %d, want %d", tc.name, rr.Code, http.StatusUnprocessableEntity)
			}
		})
	}
}

func TestHandleCreateTemplate_StoreError(t *testing.T) {
	store := &createTemplateErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"name":"T","type":"avs","extension":"avs","content":"x"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/templates", bytes.NewBufferString(body))
	srv.handleCreateTemplate(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleUpdateTemplate
// ---------------------------------------------------------------------------

func TestHandleUpdateTemplate_Success(t *testing.T) {
	store := &updateTemplateStore{
		stubStore: &stubStore{},
		tmpl:      &db.Template{ID: "t1", Name: "Updated", Type: "avs", Extension: "avs", Content: "# updated"},
	}
	srv := newTestServer(store)

	body := `{"name":"Updated","content":"# updated"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/templates/t1", bytes.NewBufferString(body))
	req.SetPathValue("id", "t1")
	srv.handleUpdateTemplate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleUpdateTemplate_InvalidJSON(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/templates/t1", bytes.NewBufferString("bad"))
	req.SetPathValue("id", "t1")
	srv.handleUpdateTemplate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateTemplate_NotFound(t *testing.T) {
	store := &updateTemplateNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"name":"X","content":"x"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/templates/missing", bytes.NewBufferString(body))
	req.SetPathValue("id", "missing")
	srv.handleUpdateTemplate(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleUpdateTemplate_StoreError(t *testing.T) {
	store := &updateTemplateErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"name":"X","content":"x"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/templates/t1", bytes.NewBufferString(body))
	req.SetPathValue("id", "t1")
	srv.handleUpdateTemplate(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleDeleteTemplate
// ---------------------------------------------------------------------------

func TestHandleDeleteTemplate_Success(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/templates/t1", nil)
	req.SetPathValue("id", "t1")
	srv.handleDeleteTemplate(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestHandleDeleteTemplate_NotFound(t *testing.T) {
	store := &deleteTemplateNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/templates/missing", nil)
	req.SetPathValue("id", "missing")
	srv.handleDeleteTemplate(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleDeleteTemplate_StoreError(t *testing.T) {
	store := &deleteTemplateErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/templates/t1", nil)
	req.SetPathValue("id", "t1")
	srv.handleDeleteTemplate(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// store stubs for templates tests
// ---------------------------------------------------------------------------

type listTemplatesStore struct {
	*stubStore
	templates []*db.Template
}

func (s *listTemplatesStore) ListTemplates(_ context.Context, _ string) ([]*db.Template, error) {
	return s.templates, nil
}

type listTemplatesErrStore struct{ *stubStore }

func (s *listTemplatesErrStore) ListTemplates(_ context.Context, _ string) ([]*db.Template, error) {
	return nil, errors.New("db failure")
}

type listTemplatesFilterStore struct {
	*stubStore
	wantType  string
	gotType   string
	templates []*db.Template
}

func (s *listTemplatesFilterStore) ListTemplates(_ context.Context, t string) ([]*db.Template, error) {
	s.gotType = t
	return s.templates, nil
}

type getTemplateStore struct {
	*stubStore
	tmpl *db.Template
}

func (s *getTemplateStore) GetTemplateByID(_ context.Context, _ string) (*db.Template, error) {
	return s.tmpl, nil
}

type getTemplateNotFoundStore struct{ *stubStore }

func (s *getTemplateNotFoundStore) GetTemplateByID(_ context.Context, _ string) (*db.Template, error) {
	return nil, db.ErrNotFound
}

type getTemplateErrStore struct{ *stubStore }

func (s *getTemplateErrStore) GetTemplateByID(_ context.Context, _ string) (*db.Template, error) {
	return nil, errors.New("db failure")
}

type createTemplateStore struct {
	*stubStore
	created *db.Template
}

func (s *createTemplateStore) CreateTemplate(_ context.Context, _ db.CreateTemplateParams) (*db.Template, error) {
	return s.created, nil
}

type createTemplateErrStore struct{ *stubStore }

func (s *createTemplateErrStore) CreateTemplate(_ context.Context, _ db.CreateTemplateParams) (*db.Template, error) {
	return nil, errors.New("db failure")
}

type updateTemplateStore struct {
	*stubStore
	tmpl *db.Template
}

func (s *updateTemplateStore) UpdateTemplate(_ context.Context, _ db.UpdateTemplateParams) error {
	return nil
}

func (s *updateTemplateStore) GetTemplateByID(_ context.Context, _ string) (*db.Template, error) {
	return s.tmpl, nil
}

type updateTemplateNotFoundStore struct{ *stubStore }

func (s *updateTemplateNotFoundStore) GetTemplateByID(_ context.Context, _ string) (*db.Template, error) {
	return nil, db.ErrNotFound
}

func (s *updateTemplateNotFoundStore) UpdateTemplate(_ context.Context, _ db.UpdateTemplateParams) error {
	return db.ErrNotFound
}

type updateTemplateErrStore struct{ *stubStore }

func (s *updateTemplateErrStore) GetTemplateByID(_ context.Context, _ string) (*db.Template, error) {
	return &db.Template{ID: "t1", Name: "X", Content: "old"}, nil
}

func (s *updateTemplateErrStore) UpdateTemplate(_ context.Context, _ db.UpdateTemplateParams) error {
	return errors.New("db failure")
}

type deleteTemplateNotFoundStore struct{ *stubStore }

func (s *deleteTemplateNotFoundStore) DeleteTemplate(_ context.Context, _ string) error {
	return db.ErrNotFound
}

type deleteTemplateErrStore struct{ *stubStore }

func (s *deleteTemplateErrStore) DeleteTemplate(_ context.Context, _ string) error {
	return errors.New("db failure")
}

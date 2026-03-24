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
// TestHandleListEncodingProfiles
// ---------------------------------------------------------------------------

func TestHandleListEncodingProfiles(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := &listProfilesStore{
			stubStore: &stubStore{},
			profiles: []*db.EncodingProfile{
				{ID: "p1", Name: "4K HDR", Container: "mkv"},
				{ID: "p2", Name: "1080p", Container: "mp4"},
			},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/encoding-profiles", nil)
		srv.handleListEncodingProfiles(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data []map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if len(body.Data) != 2 {
			t.Errorf("len = %d, want 2", len(body.Data))
		}
	})

	t.Run("nil profiles returns empty array", func(t *testing.T) {
		store := &listProfilesStore{
			stubStore: &stubStore{},
			profiles:  nil,
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/encoding-profiles", nil)
		srv.handleListEncodingProfiles(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data []map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data == nil {
			t.Error("expected non-nil empty array")
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &listProfilesErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/encoding-profiles", nil)
		srv.handleListEncodingProfiles(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

type listProfilesStore struct {
	*stubStore
	profiles []*db.EncodingProfile
}

func (s *listProfilesStore) ListEncodingProfiles(_ context.Context) ([]*db.EncodingProfile, error) {
	return s.profiles, nil
}

type listProfilesErrStore struct{ *stubStore }

func (s *listProfilesErrStore) ListEncodingProfiles(_ context.Context) ([]*db.EncodingProfile, error) {
	return nil, errTestDB
}

// ---------------------------------------------------------------------------
// TestHandleGetEncodingProfile
// ---------------------------------------------------------------------------

func TestHandleGetEncodingProfile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := &getProfileStore{
			stubStore: &stubStore{},
			profile:   &db.EncodingProfile{ID: "p1", Name: "4K HDR", Container: "mkv"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/encoding-profiles/p1", nil)
		req.SetPathValue("id", "p1")
		srv.handleGetEncodingProfile(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data db.EncodingProfile `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data.ID != "p1" {
			t.Errorf("id = %q, want p1", body.Data.ID)
		}
	})

	t.Run("not found returns 404", func(t *testing.T) {
		store := &getProfileNotFoundStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/encoding-profiles/missing", nil)
		req.SetPathValue("id", "missing")
		srv.handleGetEncodingProfile(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &getProfileErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/encoding-profiles/p1", nil)
		req.SetPathValue("id", "p1")
		srv.handleGetEncodingProfile(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

type getProfileStore struct {
	*stubStore
	profile *db.EncodingProfile
}

func (s *getProfileStore) GetEncodingProfileByID(_ context.Context, _ string) (*db.EncodingProfile, error) {
	return s.profile, nil
}

type getProfileNotFoundStore struct{ *stubStore }

func (s *getProfileNotFoundStore) GetEncodingProfileByID(_ context.Context, _ string) (*db.EncodingProfile, error) {
	return nil, db.ErrNotFound
}

type getProfileErrStore struct{ *stubStore }

func (s *getProfileErrStore) GetEncodingProfileByID(_ context.Context, _ string) (*db.EncodingProfile, error) {
	return nil, errTestDB
}

// ---------------------------------------------------------------------------
// TestHandleCreateEncodingProfile
// ---------------------------------------------------------------------------

func TestHandleCreateEncodingProfile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		now := time.Now()
		store := &createProfileStore{
			stubStore: &stubStore{},
			profile:   &db.EncodingProfile{ID: "p1", Name: "Test", Container: "mkv", CreatedAt: now},
		}
		srv := newTestServer(store)

		body := `{"name":"Test","container":"mkv","settings":{"codec":"hevc"}}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/encoding-profiles", bytes.NewBufferString(body))
		srv.handleCreateEncodingProfile(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
		}
		var resp struct {
			Data db.EncodingProfile `json:"data"`
		}
		decodeJSON(t, rr, &resp)
		if resp.Data.ID != "p1" {
			t.Errorf("id = %q, want p1", resp.Data.ID)
		}
	})

	t.Run("missing name returns 422", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		body := `{"description":"no name"}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/encoding-profiles", bytes.NewBufferString(body))
		srv.handleCreateEncodingProfile(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/encoding-profiles", bytes.NewBufferString(`{bad`))
		srv.handleCreateEncodingProfile(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("defaults container to mkv", func(t *testing.T) {
		store := &createProfileStore{
			stubStore: &stubStore{},
			profile:   &db.EncodingProfile{ID: "p2", Name: "NoContainer", Container: "mkv"},
		}
		srv := newTestServer(store)

		body := `{"name":"NoContainer"}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/encoding-profiles", bytes.NewBufferString(body))
		srv.handleCreateEncodingProfile(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
		}
		if store.calledParams.Container != "mkv" {
			t.Errorf("container = %q, want mkv", store.calledParams.Container)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &createProfileErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		body := `{"name":"Test"}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/encoding-profiles", bytes.NewBufferString(body))
		srv.handleCreateEncodingProfile(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

type createProfileStore struct {
	*stubStore
	profile      *db.EncodingProfile
	calledParams db.CreateEncodingProfileParams
}

func (s *createProfileStore) CreateEncodingProfile(_ context.Context, p db.CreateEncodingProfileParams) (*db.EncodingProfile, error) {
	s.calledParams = p
	return s.profile, nil
}

type createProfileErrStore struct{ *stubStore }

func (s *createProfileErrStore) CreateEncodingProfile(_ context.Context, _ db.CreateEncodingProfileParams) (*db.EncodingProfile, error) {
	return nil, errTestDB
}

// ---------------------------------------------------------------------------
// TestHandleUpdateEncodingProfile
// ---------------------------------------------------------------------------

func TestHandleUpdateEncodingProfile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := &updateProfileStore{
			stubStore: &stubStore{},
			existing:  &db.EncodingProfile{ID: "p1", Name: "Old"},
			updated:   &db.EncodingProfile{ID: "p1", Name: "New", Container: "mp4"},
		}
		srv := newTestServer(store)

		body := `{"name":"New","container":"mp4"}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/encoding-profiles/p1", bytes.NewBufferString(body))
		req.SetPathValue("id", "p1")
		srv.handleUpdateEncodingProfile(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("not found returns 404", func(t *testing.T) {
		store := &getProfileNotFoundStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		body := `{"name":"New"}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/encoding-profiles/missing", bytes.NewBufferString(body))
		req.SetPathValue("id", "missing")
		srv.handleUpdateEncodingProfile(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("get error returns 500", func(t *testing.T) {
		store := &getProfileErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		body := `{"name":"New"}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/encoding-profiles/p1", bytes.NewBufferString(body))
		req.SetPathValue("id", "p1")
		srv.handleUpdateEncodingProfile(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		store := &updateProfileStore{
			stubStore: &stubStore{},
			existing:  &db.EncodingProfile{ID: "p1"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/encoding-profiles/p1", bytes.NewBufferString(`{bad`))
		req.SetPathValue("id", "p1")
		srv.handleUpdateEncodingProfile(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing name returns 422", func(t *testing.T) {
		store := &updateProfileStore{
			stubStore: &stubStore{},
			existing:  &db.EncodingProfile{ID: "p1"},
		}
		srv := newTestServer(store)

		body := `{"description":"no name"}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/encoding-profiles/p1", bytes.NewBufferString(body))
		req.SetPathValue("id", "p1")
		srv.handleUpdateEncodingProfile(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("update store error returns 500", func(t *testing.T) {
		store := &updateProfileErrStore{
			stubStore: &stubStore{},
			existing:  &db.EncodingProfile{ID: "p1"},
		}
		srv := newTestServer(store)

		body := `{"name":"New"}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/encoding-profiles/p1", bytes.NewBufferString(body))
		req.SetPathValue("id", "p1")
		srv.handleUpdateEncodingProfile(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

type updateProfileStore struct {
	*stubStore
	existing *db.EncodingProfile
	updated  *db.EncodingProfile
}

func (s *updateProfileStore) GetEncodingProfileByID(_ context.Context, _ string) (*db.EncodingProfile, error) {
	return s.existing, nil
}

func (s *updateProfileStore) UpdateEncodingProfile(_ context.Context, _ db.UpdateEncodingProfileParams) (*db.EncodingProfile, error) {
	return s.updated, nil
}

type updateProfileErrStore struct {
	*stubStore
	existing *db.EncodingProfile
}

func (s *updateProfileErrStore) GetEncodingProfileByID(_ context.Context, _ string) (*db.EncodingProfile, error) {
	return s.existing, nil
}

func (s *updateProfileErrStore) UpdateEncodingProfile(_ context.Context, _ db.UpdateEncodingProfileParams) (*db.EncodingProfile, error) {
	return nil, errTestDB
}

// ---------------------------------------------------------------------------
// TestHandleDeleteEncodingProfile
// ---------------------------------------------------------------------------

func TestHandleDeleteEncodingProfile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := &deleteProfileStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/encoding-profiles/p1", nil)
		req.SetPathValue("id", "p1")
		srv.handleDeleteEncodingProfile(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data["ok"] != true {
			t.Error("expected ok to be true")
		}
	})

	t.Run("not found returns 404", func(t *testing.T) {
		store := &deleteProfileNotFoundStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/encoding-profiles/missing", nil)
		req.SetPathValue("id", "missing")
		srv.handleDeleteEncodingProfile(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &deleteProfileErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/encoding-profiles/p1", nil)
		req.SetPathValue("id", "p1")
		srv.handleDeleteEncodingProfile(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

type deleteProfileStore struct{ *stubStore }

func (s *deleteProfileStore) DeleteEncodingProfile(_ context.Context, _ string) error {
	return nil
}

type deleteProfileNotFoundStore struct{ *stubStore }

func (s *deleteProfileNotFoundStore) DeleteEncodingProfile(_ context.Context, _ string) error {
	return db.ErrNotFound
}

type deleteProfileErrStore struct{ *stubStore }

func (s *deleteProfileErrStore) DeleteEncodingProfile(_ context.Context, _ string) error {
	return errTestDB
}

package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/badskater/distributed-encoder/internal/db"
)

// ---------------------------------------------------------------------------
// handleListSources
// ---------------------------------------------------------------------------

func TestHandleListSources_Success(t *testing.T) {
	store := &listSourcesStore{
		stubStore: &stubStore{},
		sources: []*db.Source{
			{ID: "s1", Filename: "movie.mkv", UNCPath: `\\nas\share\movie.mkv`},
			{ID: "s2", Filename: "series.mkv", UNCPath: `\\nas\share\series.mkv`},
		},
		total: 2,
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	srv.handleListSources(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body struct {
		Data []json.RawMessage `json:"data"`
		Meta map[string]any    `json:"meta"`
	}
	decodeJSON(t, rr, &body)
	if len(body.Data) != 2 {
		t.Errorf("len(data) = %d, want 2", len(body.Data))
	}
	if body.Meta["total_count"].(float64) != 2 {
		t.Errorf("meta.total_count = %v, want 2", body.Meta["total_count"])
	}
}

func TestHandleListSources_BadPageSize(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources?page_size=abc", nil)
	srv.handleListSources(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleListSources_ZeroPageSize(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources?page_size=0", nil)
	srv.handleListSources(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleListSources_InvalidCursor(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources?cursor=!!!notbase64!!!", nil)
	srv.handleListSources(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleListSources_ValidCursor(t *testing.T) {
	store := &listSourcesStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	cursor := base64.StdEncoding.EncodeToString([]byte("s2"))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources?cursor="+cursor, nil)
	srv.handleListSources(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleListSources_NextCursorSet(t *testing.T) {
	// Return exactly pageSize (50) sources so handler emits next_cursor.
	sources := make([]*db.Source, 50)
	for i := range sources {
		sources[i] = &db.Source{ID: "s-" + string(rune('a'+i%26)), Filename: "f.mkv"}
	}
	store := &listSourcesStore{stubStore: &stubStore{}, sources: sources, total: 100}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	srv.handleListSources(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body struct {
		Meta map[string]any `json:"meta"`
	}
	decodeJSON(t, rr, &body)
	if _, ok := body.Meta["next_cursor"]; !ok {
		t.Error("expected meta.next_cursor to be set when sources == page_size")
	}
}

func TestHandleListSources_StoreError(t *testing.T) {
	store := &listSourcesErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	srv.handleListSources(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestHandleListSources_MaxPageSizeClamped(t *testing.T) {
	store := &listSourcesParamsStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	// Request page_size=999 which exceeds max of 200.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources?page_size=999", nil)
	srv.handleListSources(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if store.gotFilter.PageSize != 200 {
		t.Errorf("page_size = %d, want clamped to 200", store.gotFilter.PageSize)
	}
}

// ---------------------------------------------------------------------------
// handleGetSource
// ---------------------------------------------------------------------------

func TestHandleGetSource_Success(t *testing.T) {
	store := &getSourceStore{
		stubStore: &stubStore{},
		source:    &db.Source{ID: "s1", Filename: "movie.mkv", UNCPath: `\\nas\share\movie.mkv`},
		results:   []*db.AnalysisResult{},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources/s1", nil)
	req.SetPathValue("id", "s1")
	srv.handleGetSource(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body struct {
		Data struct {
			Source json.RawMessage   `json:"source"`
			AR     []json.RawMessage `json:"analysis_results"`
		} `json:"data"`
	}
	decodeJSON(t, rr, &body)
	if body.Data.Source == nil {
		t.Error("data.source is nil")
	}
}

func TestHandleGetSource_NotFound(t *testing.T) {
	store := &getSourceNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources/missing", nil)
	req.SetPathValue("id", "missing")
	srv.handleGetSource(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleGetSource_StoreError(t *testing.T) {
	store := &getSourceErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources/s1", nil)
	req.SetPathValue("id", "s1")
	srv.handleGetSource(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestHandleGetSource_AnalysisResultsError(t *testing.T) {
	store := &getSourceAnalysisErrStore{
		stubStore: &stubStore{},
		source:    &db.Source{ID: "s1", Filename: "movie.mkv"},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources/s1", nil)
	req.SetPathValue("id", "s1")
	srv.handleGetSource(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleEncodeSource
// ---------------------------------------------------------------------------

func TestHandleEncodeSource_Success(t *testing.T) {
	store := &encodeSourceStore{
		stubStore: &stubStore{},
		source:    &db.Source{ID: "s1", Filename: "movie.mkv"},
		job:       &db.Job{ID: "j-enc", SourceID: "s1", JobType: "encode"},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/s1/encode", nil)
	req.SetPathValue("id", "s1")
	srv.handleEncodeSource(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
}

func TestHandleEncodeSource_WithBody(t *testing.T) {
	store := &encodeSourceStore{
		stubStore: &stubStore{},
		source:    &db.Source{ID: "s1", Filename: "movie.mkv"},
		job:       &db.Job{ID: "j-enc", SourceID: "s1", JobType: "encode"},
	}
	srv := newTestServer(store)

	body := `{"priority":10,"target_tags":["gpu"]}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/s1/encode", bytes.NewBufferString(body))
	req.SetPathValue("id", "s1")
	srv.handleEncodeSource(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
}

func TestHandleEncodeSource_InvalidJSON(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/s1/encode", bytes.NewBufferString("bad"))
	req.Header.Set("Content-Length", "3")
	req.ContentLength = 3
	req.SetPathValue("id", "s1")
	srv.handleEncodeSource(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleEncodeSource_NotFound(t *testing.T) {
	store := &encodeSourceNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/missing/encode", nil)
	req.SetPathValue("id", "missing")
	srv.handleEncodeSource(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleEncodeSource_StoreError(t *testing.T) {
	store := &encodeSourceErrStore{
		stubStore: &stubStore{},
		source:    &db.Source{ID: "s1"},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/s1/encode", nil)
	req.SetPathValue("id", "s1")
	srv.handleEncodeSource(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleAnalyzeSource
// ---------------------------------------------------------------------------

func TestHandleAnalyzeSource_Success(t *testing.T) {
	store := &analyzeSourceStore{
		stubStore: &stubStore{},
		source:    &db.Source{ID: "s1"},
		job:       &db.Job{ID: "j-analysis", SourceID: "s1", JobType: "analysis"},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/s1/analyze", nil)
	req.SetPathValue("id", "s1")
	srv.handleAnalyzeSource(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
}

func TestHandleAnalyzeSource_NotFound(t *testing.T) {
	store := &analyzeSourceNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/missing/analyze", nil)
	req.SetPathValue("id", "missing")
	srv.handleAnalyzeSource(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleAnalyzeSource_GetSourceError(t *testing.T) {
	store := &analyzeSourceGetErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/s1/analyze", nil)
	req.SetPathValue("id", "s1")
	srv.handleAnalyzeSource(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestHandleAnalyzeSource_CreateJobError(t *testing.T) {
	store := &analyzeSourceJobErrStore{
		stubStore: &stubStore{},
		source:    &db.Source{ID: "s1"},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/s1/analyze", nil)
	req.SetPathValue("id", "s1")
	srv.handleAnalyzeSource(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleDeleteSource
// ---------------------------------------------------------------------------

func TestHandleDeleteSource_Success(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sources/s1", nil)
	req.SetPathValue("id", "s1")
	srv.handleDeleteSource(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestHandleDeleteSource_NotFound(t *testing.T) {
	store := &deleteSourceNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sources/missing", nil)
	req.SetPathValue("id", "missing")
	srv.handleDeleteSource(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleDeleteSource_StoreError(t *testing.T) {
	store := &deleteSourceErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sources/s1", nil)
	req.SetPathValue("id", "s1")
	srv.handleDeleteSource(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleGetSourceScenes
// ---------------------------------------------------------------------------

func TestHandleGetSourceScenes_SourceNotFound(t *testing.T) {
	store := &getScenesSourceNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources/missing/scenes", nil)
	req.SetPathValue("id", "missing")
	srv.handleGetSourceScenes(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleGetSourceScenes_NoAnalysis(t *testing.T) {
	store := &getScenesNoAnalysisStore{
		stubStore: &stubStore{},
		source:    &db.Source{ID: "s1"},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources/s1/scenes", nil)
	req.SetPathValue("id", "s1")
	srv.handleGetSourceScenes(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleGetSourceScenes_Success(t *testing.T) {
	frameData := json.RawMessage(`[{"frame":0,"pts":0.0},{"frame":24,"pts":1.0}]`)
	summary := json.RawMessage(`{"frame_count":100,"duration_sec":4.0}`)
	store := &getScenesSuccessStore{
		stubStore: &stubStore{},
		source:    &db.Source{ID: "s1"},
		result: &db.AnalysisResult{
			ID:        "ar1",
			SourceID:  "s1",
			Type:      "scene",
			FrameData: frameData,
			Summary:   summary,
		},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources/s1/scenes", nil)
	req.SetPathValue("id", "s1")
	srv.handleGetSourceScenes(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body struct {
		Data struct {
			SourceID   string          `json:"source_id"`
			Scenes     json.RawMessage `json:"scenes"`
			TotalFrames int            `json:"total_frames"`
		} `json:"data"`
	}
	decodeJSON(t, rr, &body)
	if body.Data.SourceID != "s1" {
		t.Errorf("source_id = %q, want %q", body.Data.SourceID, "s1")
	}
}

// ---------------------------------------------------------------------------
// handleCreateSource: cloud_uri path
// ---------------------------------------------------------------------------

func TestHandleCreateSource_CloudURI_Success(t *testing.T) {
	store := &createSourceCloudStore{
		stubStore: &stubStore{},
		created:   &db.Source{ID: "src-cloud", Filename: "video.mp4"},
	}
	srv := newTestServer(store)

	cloudURI := "s3://my-bucket/video.mp4"
	body := `{"cloud_uri":"` + cloudURI + `"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", bytes.NewBufferString(body))
	srv.handleCreateSource(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
}

func TestHandleCreateSource_CloudURI_InvalidScheme(t *testing.T) {
	srv := newTestServer(&stubStore{})

	body := `{"cloud_uri":"ftp://my-bucket/video.mp4"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", bytes.NewBufferString(body))
	srv.handleCreateSource(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateSource_BothPathAndCloudURI(t *testing.T) {
	srv := newTestServer(&stubStore{})

	body := `{"path":"\\\\nas\\share\\x.mkv","cloud_uri":"s3://bucket/x.mkv"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", bytes.NewBufferString(body))
	srv.handleCreateSource(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateSource_NeitherPathNorCloudURI(t *testing.T) {
	srv := newTestServer(&stubStore{})

	body := `{"name":"no-path"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", bytes.NewBufferString(body))
	srv.handleCreateSource(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateSource_GetByUNCPathError(t *testing.T) {
	store := &createSourceUNCErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"path":"\\\\nas\\share\\film.mkv"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", bytes.NewBufferString(body))
	srv.handleCreateSource(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestHandleCreateSource_CreateSourceStoreError(t *testing.T) {
	store := &createSourceStoreErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"path":"\\\\nas\\share\\film.mkv"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", bytes.NewBufferString(body))
	srv.handleCreateSource(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// ptsToTimecode helper
// ---------------------------------------------------------------------------

func TestPtsToTimecode(t *testing.T) {
	cases := []struct {
		pts  float64
		fps  float64
		want string
	}{
		{0, 24, "00:00:00.00"},
		{3661, 24, "01:01:01.00"},
		{90, 25, "00:01:30.00"},
	}
	for _, tc := range cases {
		got := ptsToTimecode(tc.pts, tc.fps)
		if got != tc.want {
			t.Errorf("ptsToTimecode(%v, %v) = %q, want %q", tc.pts, tc.fps, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// store stubs for sources_crud tests
// ---------------------------------------------------------------------------

type listSourcesStore struct {
	*stubStore
	sources []*db.Source
	total   int64
}

func (s *listSourcesStore) ListSources(_ context.Context, _ db.ListSourcesFilter) ([]*db.Source, int64, error) {
	return s.sources, s.total, nil
}

type listSourcesErrStore struct{ *stubStore }

func (s *listSourcesErrStore) ListSources(_ context.Context, _ db.ListSourcesFilter) ([]*db.Source, int64, error) {
	return nil, 0, errors.New("db failure")
}

type listSourcesParamsStore struct {
	*stubStore
	gotFilter db.ListSourcesFilter
}

func (s *listSourcesParamsStore) ListSources(_ context.Context, f db.ListSourcesFilter) ([]*db.Source, int64, error) {
	s.gotFilter = f
	return []*db.Source{}, 0, nil
}

type getSourceStore struct {
	*stubStore
	source  *db.Source
	results []*db.AnalysisResult
}

func (s *getSourceStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return s.source, nil
}

func (s *getSourceStore) ListAnalysisResults(_ context.Context, _ string) ([]*db.AnalysisResult, error) {
	return s.results, nil
}

type getSourceNotFoundStore struct{ *stubStore }

func (s *getSourceNotFoundStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return nil, db.ErrNotFound
}

type getSourceErrStore struct{ *stubStore }

func (s *getSourceErrStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return nil, errors.New("db failure")
}

type getSourceAnalysisErrStore struct {
	*stubStore
	source *db.Source
}

func (s *getSourceAnalysisErrStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return s.source, nil
}

func (s *getSourceAnalysisErrStore) ListAnalysisResults(_ context.Context, _ string) ([]*db.AnalysisResult, error) {
	return nil, errors.New("analysis db failure")
}

type encodeSourceStore struct {
	*stubStore
	source *db.Source
	job    *db.Job
}

func (s *encodeSourceStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return s.source, nil
}

func (s *encodeSourceStore) CreateJob(_ context.Context, _ db.CreateJobParams) (*db.Job, error) {
	return s.job, nil
}

type encodeSourceNotFoundStore struct{ *stubStore }

func (s *encodeSourceNotFoundStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return nil, db.ErrNotFound
}

type encodeSourceErrStore struct {
	*stubStore
	source *db.Source
}

func (s *encodeSourceErrStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return s.source, nil
}

func (s *encodeSourceErrStore) CreateJob(_ context.Context, _ db.CreateJobParams) (*db.Job, error) {
	return nil, errors.New("db failure")
}

type analyzeSourceStore struct {
	*stubStore
	source *db.Source
	job    *db.Job
}

func (s *analyzeSourceStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return s.source, nil
}

func (s *analyzeSourceStore) CreateJob(_ context.Context, _ db.CreateJobParams) (*db.Job, error) {
	return s.job, nil
}

type analyzeSourceNotFoundStore struct{ *stubStore }

func (s *analyzeSourceNotFoundStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return nil, db.ErrNotFound
}

type analyzeSourceGetErrStore struct{ *stubStore }

func (s *analyzeSourceGetErrStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return nil, errors.New("db failure")
}

type analyzeSourceJobErrStore struct {
	*stubStore
	source *db.Source
}

func (s *analyzeSourceJobErrStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return s.source, nil
}

func (s *analyzeSourceJobErrStore) CreateJob(_ context.Context, _ db.CreateJobParams) (*db.Job, error) {
	return nil, errors.New("db failure")
}

type deleteSourceNotFoundStore struct{ *stubStore }

func (s *deleteSourceNotFoundStore) DeleteSource(_ context.Context, _ string) error {
	return db.ErrNotFound
}

type deleteSourceErrStore struct{ *stubStore }

func (s *deleteSourceErrStore) DeleteSource(_ context.Context, _ string) error {
	return errors.New("db failure")
}

type getScenesSourceNotFoundStore struct{ *stubStore }

func (s *getScenesSourceNotFoundStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return nil, db.ErrNotFound
}

type getScenesNoAnalysisStore struct {
	*stubStore
	source *db.Source
}

func (s *getScenesNoAnalysisStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return s.source, nil
}

func (s *getScenesNoAnalysisStore) GetAnalysisResult(_ context.Context, _, _ string) (*db.AnalysisResult, error) {
	return nil, db.ErrNotFound
}

type getScenesSuccessStore struct {
	*stubStore
	source *db.Source
	result *db.AnalysisResult
}

func (s *getScenesSuccessStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return s.source, nil
}

func (s *getScenesSuccessStore) GetAnalysisResult(_ context.Context, _, _ string) (*db.AnalysisResult, error) {
	return s.result, nil
}

type createSourceCloudStore struct {
	*stubStore
	created *db.Source
}

func (s *createSourceCloudStore) GetSourceByUNCPath(_ context.Context, _ string) (*db.Source, error) {
	return nil, db.ErrNotFound
}

func (s *createSourceCloudStore) CreateSource(_ context.Context, _ db.CreateSourceParams) (*db.Source, error) {
	return s.created, nil
}

func (s *createSourceCloudStore) CreateJob(_ context.Context, _ db.CreateJobParams) (*db.Job, error) {
	return &db.Job{ID: "auto"}, nil
}

type createSourceUNCErrStore struct{ *stubStore }

func (s *createSourceUNCErrStore) GetSourceByUNCPath(_ context.Context, _ string) (*db.Source, error) {
	return nil, errors.New("db failure")
}

type createSourceStoreErrStore struct{ *stubStore }

func (s *createSourceStoreErrStore) GetSourceByUNCPath(_ context.Context, _ string) (*db.Source, error) {
	return nil, db.ErrNotFound
}

func (s *createSourceStoreErrStore) CreateSource(_ context.Context, _ db.CreateSourceParams) (*db.Source, error) {
	return nil, errors.New("db failure")
}

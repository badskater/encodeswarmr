package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/badskater/distributed-encoder/internal/db"
)

// ---------------------------------------------------------------------------
// TestHandleScanAnalysis
// ---------------------------------------------------------------------------

func TestHandleScanAnalysis(t *testing.T) {
	t.Run("invalid JSON returns 400", func(t *testing.T) {
		srv := newTestServer(&stubStore{})
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/analysis/scan",
			bytes.NewBufferString(`{not json`))
		srv.handleScanAnalysis(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing source_id returns 422", func(t *testing.T) {
		srv := newTestServer(&stubStore{})
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/analysis/scan",
			bytes.NewBufferString(`{"analysis_type":"vmaf"}`))
		srv.handleScanAnalysis(rr, req)
		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &scanAnalysisErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/analysis/scan",
			bytes.NewBufferString(`{"source_id":"s1"}`))
		srv.handleScanAnalysis(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("success with default analysis_type", func(t *testing.T) {
		store := &scanAnalysisSuccessStore{
			stubStore: &stubStore{},
			job:       &db.Job{ID: "j1", SourceID: "s1", Status: "queued", JobType: "analysis"},
		}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/analysis/scan",
			bytes.NewBufferString(`{"source_id":"s1"}`))
		srv.handleScanAnalysis(rr, req)
		if rr.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusAccepted)
		}
		var body struct {
			Data map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data["job_id"] != "j1" {
			t.Errorf("data.job_id = %v, want j1", body.Data["job_id"])
		}
		if body.Data["source_id"] != "s1" {
			t.Errorf("data.source_id = %v, want s1", body.Data["source_id"])
		}
		if body.Data["status"] != "queued" {
			t.Errorf("data.status = %v, want queued", body.Data["status"])
		}
	})

	t.Run("success with explicit analysis_type", func(t *testing.T) {
		store := &scanAnalysisSuccessStore{
			stubStore: &stubStore{},
			job:       &db.Job{ID: "j2", SourceID: "s1", Status: "queued", JobType: "analysis"},
		}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/analysis/scan",
			bytes.NewBufferString(`{"source_id":"s1","analysis_type":"histogram"}`))
		srv.handleScanAnalysis(rr, req)
		if rr.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusAccepted)
		}
	})
}

type scanAnalysisErrStore struct{ *stubStore }

func (s *scanAnalysisErrStore) CreateJob(_ context.Context, _ db.CreateJobParams) (*db.Job, error) {
	return nil, errTestDB
}

type scanAnalysisSuccessStore struct {
	*stubStore
	job *db.Job
}

func (s *scanAnalysisSuccessStore) CreateJob(_ context.Context, _ db.CreateJobParams) (*db.Job, error) {
	return s.job, nil
}

// ---------------------------------------------------------------------------
// TestHandleGetAnalysisResult
// ---------------------------------------------------------------------------

func TestHandleGetAnalysisResult(t *testing.T) {
	t.Run("not found returns 404", func(t *testing.T) {
		store := &getAnalysisResultNotFoundStore{stubStore: &stubStore{}}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/analysis/s1", nil)
		req.SetPathValue("source_id", "s1")
		srv.handleGetAnalysisResult(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &getAnalysisResultErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/analysis/s1", nil)
		req.SetPathValue("source_id", "s1")
		srv.handleGetAnalysisResult(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("success with default type", func(t *testing.T) {
		now := time.Now()
		store := &getAnalysisResultSuccessStore{
			stubStore: &stubStore{},
			result: &db.AnalysisResult{
				ID:       "ar1",
				SourceID: "s1",
				Type:     "vmaf",
				Summary:  []byte(`{"score":98.5}`),
				CreatedAt: now,
			},
		}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/analysis/s1", nil)
		req.SetPathValue("source_id", "s1")
		srv.handleGetAnalysisResult(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data["id"] != "ar1" {
			t.Errorf("data.id = %v, want ar1", body.Data["id"])
		}
	})

	t.Run("success with explicit type param", func(t *testing.T) {
		now := time.Now()
		store := &getAnalysisResultSuccessStore{
			stubStore: &stubStore{},
			result: &db.AnalysisResult{
				ID:        "ar2",
				SourceID:  "s1",
				Type:      "histogram",
				CreatedAt: now,
			},
		}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/analysis/s1?type=histogram", nil)
		req.SetPathValue("source_id", "s1")
		srv.handleGetAnalysisResult(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("success with nil frame_data and summary", func(t *testing.T) {
		now := time.Now()
		store := &getAnalysisResultSuccessStore{
			stubStore: &stubStore{},
			result: &db.AnalysisResult{
				ID:        "ar3",
				SourceID:  "s1",
				Type:      "vmaf",
				FrameData: nil,
				Summary:   nil,
				CreatedAt: now,
			},
		}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/analysis/s1", nil)
		req.SetPathValue("source_id", "s1")
		srv.handleGetAnalysisResult(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})
}

type getAnalysisResultNotFoundStore struct{ *stubStore }

func (s *getAnalysisResultNotFoundStore) GetAnalysisResult(_ context.Context, _, _ string) (*db.AnalysisResult, error) {
	return nil, db.ErrNotFound
}

type getAnalysisResultErrStore struct{ *stubStore }

func (s *getAnalysisResultErrStore) GetAnalysisResult(_ context.Context, _, _ string) (*db.AnalysisResult, error) {
	return nil, errTestDB
}

type getAnalysisResultSuccessStore struct {
	*stubStore
	result *db.AnalysisResult
}

func (s *getAnalysisResultSuccessStore) GetAnalysisResult(_ context.Context, _, _ string) (*db.AnalysisResult, error) {
	return s.result, nil
}

// ---------------------------------------------------------------------------
// TestHandleListAnalysisResults
// ---------------------------------------------------------------------------

func TestHandleListAnalysisResults(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		now := time.Now()
		store := &listAnalysisResultsStore{
			stubStore: &stubStore{},
			results: []*db.AnalysisResult{
				{ID: "ar1", SourceID: "s1", Type: "vmaf", CreatedAt: now},
				{ID: "ar2", SourceID: "s1", Type: "histogram", CreatedAt: now},
			},
		}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/analysis/s1/all", nil)
		req.SetPathValue("source_id", "s1")
		srv.handleListAnalysisResults(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data []map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if len(body.Data) != 2 {
			t.Errorf("len(data) = %d, want 2", len(body.Data))
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &listAnalysisResultsErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/analysis/s1/all", nil)
		req.SetPathValue("source_id", "s1")
		srv.handleListAnalysisResults(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("empty results", func(t *testing.T) {
		store := &listAnalysisResultsStore{
			stubStore: &stubStore{},
			results:   []*db.AnalysisResult{},
		}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/analysis/s1/all", nil)
		req.SetPathValue("source_id", "s1")
		srv.handleListAnalysisResults(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})
}

type listAnalysisResultsStore struct {
	*stubStore
	results []*db.AnalysisResult
}

func (s *listAnalysisResultsStore) ListAnalysisResults(_ context.Context, _ string) ([]*db.AnalysisResult, error) {
	return s.results, nil
}

type listAnalysisResultsErrStore struct{ *stubStore }

func (s *listAnalysisResultsErrStore) ListAnalysisResults(_ context.Context, _ string) ([]*db.AnalysisResult, error) {
	return nil, errTestDB
}

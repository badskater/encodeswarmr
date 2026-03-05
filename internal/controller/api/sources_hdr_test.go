package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/badskater/distributed-encoder/internal/db"
)

// ---------------------------------------------------------------------------
// handleHDRDetectSource
// ---------------------------------------------------------------------------

func TestHandleHDRDetectSource_NotFound(t *testing.T) {
	store := &hdrDetectNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/missing/hdr-detect", nil)
	req.SetPathValue("id", "missing")
	srv.handleHDRDetectSource(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleHDRDetectSource_Success(t *testing.T) {
	store := &hdrDetectSuccessStore{
		stubStore: &stubStore{},
		source:    &db.Source{ID: "src-1", Filename: "movie.mkv"},
		job:       &db.Job{ID: "job-hdr-1", SourceID: "src-1", JobType: "hdr_detect"},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/src-1/hdr-detect", nil)
	req.SetPathValue("id", "src-1")
	srv.handleHDRDetectSource(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var body struct {
		Data struct {
			JobID    string `json:"job_id"`
			SourceID string `json:"source_id"`
			Status   string `json:"status"`
		} `json:"data"`
	}
	decodeJSON(t, rr, &body)

	if body.Data.JobID != "job-hdr-1" {
		t.Errorf("data.job_id = %q, want %q", body.Data.JobID, "job-hdr-1")
	}
	if body.Data.SourceID != "src-1" {
		t.Errorf("data.source_id = %q, want %q", body.Data.SourceID, "src-1")
	}
	if body.Data.Status != "queued" {
		t.Errorf("data.status = %q, want %q", body.Data.Status, "queued")
	}
	if store.createdJobType != "hdr_detect" {
		t.Errorf("created job_type = %q, want %q", store.createdJobType, "hdr_detect")
	}
}

type hdrDetectNotFoundStore struct{ *stubStore }

func (s *hdrDetectNotFoundStore) GetSourceByID(context.Context, string) (*db.Source, error) {
	return nil, db.ErrNotFound
}

type hdrDetectSuccessStore struct {
	*stubStore
	source         *db.Source
	job            *db.Job
	createdJobType string
}

func (s *hdrDetectSuccessStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return s.source, nil
}

func (s *hdrDetectSuccessStore) CreateJob(_ context.Context, p db.CreateJobParams) (*db.Job, error) {
	s.createdJobType = p.JobType
	return s.job, nil
}

// ---------------------------------------------------------------------------
// handleUpdateSourceHDR
// ---------------------------------------------------------------------------

func TestHandleUpdateSourceHDR_InvalidJSON(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/src-1/hdr", bytes.NewBufferString("not-json"))
	req.SetPathValue("id", "src-1")
	srv.handleUpdateSourceHDR(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateSourceHDR_InvalidHDRType(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/src-1/hdr",
		bytes.NewBufferString(`{"hdr_type":"invalid_type","dv_profile":0}`))
	req.SetPathValue("id", "src-1")
	srv.handleUpdateSourceHDR(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateSourceHDR_NegativeDVProfile(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/src-1/hdr",
		bytes.NewBufferString(`{"hdr_type":"hdr10","dv_profile":-1}`))
	req.SetPathValue("id", "src-1")
	srv.handleUpdateSourceHDR(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateSourceHDR_NotFound(t *testing.T) {
	store := &updateHDRNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/missing/hdr",
		bytes.NewBufferString(`{"hdr_type":"hdr10","dv_profile":0}`))
	req.SetPathValue("id", "missing")
	srv.handleUpdateSourceHDR(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleUpdateSourceHDR_Success(t *testing.T) {
	store := &updateHDRSuccessStore{
		stubStore: &stubStore{},
		source:    &db.Source{ID: "src-2", Filename: "dolby.mkv", HDRType: "dolby_vision", DVProfile: 8},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/src-2/hdr",
		bytes.NewBufferString(`{"hdr_type":"dolby_vision","dv_profile":8}`))
	req.SetPathValue("id", "src-2")
	srv.handleUpdateSourceHDR(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if store.calledHDRType != "dolby_vision" {
		t.Errorf("UpdateSourceHDR hdr_type = %q, want %q", store.calledHDRType, "dolby_vision")
	}
	if store.calledDVProfile != 8 {
		t.Errorf("UpdateSourceHDR dv_profile = %d, want 8", store.calledDVProfile)
	}
}

func TestHandleUpdateSourceHDR_ValidTypesAccepted(t *testing.T) {
	for _, hdrType := range []string{"", "hdr10", "hdr10+", "dolby_vision", "hlg"} {
		t.Run("type="+hdrType, func(t *testing.T) {
			store := &updateHDRSuccessStore{
				stubStore: &stubStore{},
				source:    &db.Source{ID: "src-3"},
			}
			srv := newTestServer(store)

			body := `{"hdr_type":"` + hdrType + `","dv_profile":0}`
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/src-3/hdr", bytes.NewBufferString(body))
			req.SetPathValue("id", "src-3")
			srv.handleUpdateSourceHDR(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("hdr_type=%q: status = %d, want %d", hdrType, rr.Code, http.StatusOK)
			}
		})
	}
}

type updateHDRNotFoundStore struct{ *stubStore }

func (s *updateHDRNotFoundStore) UpdateSourceHDR(context.Context, db.UpdateSourceHDRParams) error {
	return db.ErrNotFound
}

type updateHDRSuccessStore struct {
	*stubStore
	source          *db.Source
	calledHDRType   string
	calledDVProfile int
}

func (s *updateHDRSuccessStore) UpdateSourceHDR(_ context.Context, p db.UpdateSourceHDRParams) error {
	s.calledHDRType = p.HDRType
	s.calledDVProfile = p.DVProfile
	return nil
}

func (s *updateHDRSuccessStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return s.source, nil
}

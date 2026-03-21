package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/badskater/distributed-encoder/internal/db"
)

// ---------------------------------------------------------------------------
// TestHandleEstimate
// ---------------------------------------------------------------------------

func TestHandleEstimate(t *testing.T) {
	t.Run("missing source_id returns 400", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/estimate",
			bytes.NewBufferString(`{"codec":"x265","chunk_count":4}`))
		srv.handleEstimate(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/estimate",
			bytes.NewBufferString(`{not json`))
		srv.handleEstimate(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("source not found returns 404", func(t *testing.T) {
		store := &estimateSourceNotFoundStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/estimate",
			bytes.NewBufferString(`{"source_id":"s-missing","codec":"x265","chunk_count":4}`))
		srv.handleEstimate(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})

	t.Run("source DB error returns 500", func(t *testing.T) {
		store := &estimateSourceErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/estimate",
			bytes.NewBufferString(`{"source_id":"s1","codec":"x265","chunk_count":4}`))
		srv.handleEstimate(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rr.Code)
		}
	})

	t.Run("no historical data returns confidence none", func(t *testing.T) {
		store := &estimateNoHistoryStore{
			stubStore: &stubStore{},
			source:    &db.Source{ID: "s1"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/estimate",
			bytes.NewBufferString(`{"source_id":"s1","codec":"x265","chunk_count":4}`))
		srv.handleEstimate(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}

		var body struct {
			Data estimateResponse `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data.Confidence != "none" {
			t.Errorf("confidence = %q, want none", body.Data.Confidence)
		}
		if body.Data.BasedOnSamples != 0 {
			t.Errorf("based_on_samples = %d, want 0", body.Data.BasedOnSamples)
		}
	})

	t.Run("historical data with high sample count returns confidence high", func(t *testing.T) {
		store := &estimateWithHistoryStore{
			stubStore:   &stubStore{},
			source:      &db.Source{ID: "s1"},
			avgFPS:      25.0,
			sampleCount: 35,
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/estimate",
			bytes.NewBufferString(`{"source_id":"s1","codec":"x265","chunk_count":4}`))
		srv.handleEstimate(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}

		var body struct {
			Data estimateResponse `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data.Confidence != "high" {
			t.Errorf("confidence = %q, want high", body.Data.Confidence)
		}
		if body.Data.BasedOnSamples != 35 {
			t.Errorf("based_on_samples = %d, want 35", body.Data.BasedOnSamples)
		}
	})

	t.Run("historical data with medium sample count returns confidence medium", func(t *testing.T) {
		store := &estimateWithHistoryStore{
			stubStore:   &stubStore{},
			source:      &db.Source{ID: "s1"},
			avgFPS:      20.0,
			sampleCount: 10,
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/estimate",
			bytes.NewBufferString(`{"source_id":"s1","codec":"x265","chunk_count":4}`))
		srv.handleEstimate(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}

		var body struct {
			Data estimateResponse `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data.Confidence != "medium" {
			t.Errorf("confidence = %q, want medium", body.Data.Confidence)
		}
	})

	t.Run("historical data with low sample count returns confidence low", func(t *testing.T) {
		store := &estimateWithHistoryStore{
			stubStore:   &stubStore{},
			source:      &db.Source{ID: "s1"},
			avgFPS:      15.0,
			sampleCount: 3,
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/estimate",
			bytes.NewBufferString(`{"source_id":"s1","codec":"x265","chunk_count":4}`))
		srv.handleEstimate(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}

		var body struct {
			Data estimateResponse `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data.Confidence != "low" {
			t.Errorf("confidence = %q, want low", body.Data.Confidence)
		}
	})

	t.Run("valid preset_name resolves codec", func(t *testing.T) {
		store := &estimateNoHistoryStore{
			stubStore: &stubStore{},
			source:    &db.Source{ID: "s1"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/estimate",
			bytes.NewBufferString(`{"source_id":"s1","preset_name":"4K HDR10 x265 Quality","chunk_count":2}`))
		srv.handleEstimate(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}

		var body struct {
			Data estimateResponse `json:"data"`
		}
		decodeJSON(t, rr, &body)
		// Confidence none because no history store data.
		if body.Data.Confidence != "none" {
			t.Errorf("confidence = %q, want none", body.Data.Confidence)
		}
	})

	t.Run("invalid preset_name returns 400", func(t *testing.T) {
		store := &estimateNoHistoryStore{
			stubStore: &stubStore{},
			source:    &db.Source{ID: "s1"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/estimate",
			bytes.NewBufferString(`{"source_id":"s1","preset_name":"NonExistentPreset","chunk_count":2}`))
		srv.handleEstimate(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("missing both codec and preset_name returns 400", func(t *testing.T) {
		store := &estimateNoHistoryStore{
			stubStore: &stubStore{},
			source:    &db.Source{ID: "s1"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/estimate",
			bytes.NewBufferString(`{"source_id":"s1","chunk_count":2}`))
		srv.handleEstimate(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("analysis results with total_frames produces estimated duration", func(t *testing.T) {
		summaryJSON, _ := json.Marshal(map[string]any{
			"total_frames": float64(18000), // 10 minutes at 30 fps
		})
		store := &estimateWithFramesStore{
			stubStore: &stubStore{},
			source:    &db.Source{ID: "s1"},
			analysisResults: []*db.AnalysisResult{
				{ID: "ar1", SourceID: "s1", Summary: summaryJSON},
			},
			avgFPS:      30.0,
			sampleCount: 30,
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/estimate",
			bytes.NewBufferString(`{"source_id":"s1","codec":"x265","chunk_count":4}`))
		srv.handleEstimate(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}

		var body struct {
			Data estimateResponse `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data.EstimatedDurationSeconds <= 0 {
			t.Errorf("estimated_duration_seconds = %d, want > 0", body.Data.EstimatedDurationSeconds)
		}
		if body.Data.EstimatedDurationHuman == "unknown" {
			t.Error("estimated_duration_human = unknown, want a real estimate")
		}
	})
}

// ---------------------------------------------------------------------------
// TestFormatDuration
// ---------------------------------------------------------------------------

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		secs int64
		want string
	}{
		{0, "unknown"},
		{-1, "unknown"},
		{30, "0m 30s"},
		{90, "1m 30s"},
		{3600, "1h 0m"},
		{3661, "1h 1m"},
		{7200, "2h 0m"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.secs)
		if got != tt.want {
			t.Errorf("formatDuration(%d) = %q, want %q", tt.secs, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Store stubs
// ---------------------------------------------------------------------------

type estimateSourceNotFoundStore struct{ *stubStore }

func (s *estimateSourceNotFoundStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return nil, db.ErrNotFound
}

type estimateSourceErrStore struct{ *stubStore }

func (s *estimateSourceErrStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return nil, errTestDB
}

type estimateNoHistoryStore struct {
	*stubStore
	source *db.Source
}

func (s *estimateNoHistoryStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return s.source, nil
}

// GetAvgFPSStats returns zero values — no historical data.
func (s *estimateNoHistoryStore) GetAvgFPSStats(_ context.Context, _ string) (float64, int64, error) {
	return 0, 0, nil
}

type estimateWithHistoryStore struct {
	*stubStore
	source      *db.Source
	avgFPS      float64
	sampleCount int64
}

func (s *estimateWithHistoryStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return s.source, nil
}

func (s *estimateWithHistoryStore) GetAvgFPSStats(_ context.Context, _ string) (float64, int64, error) {
	return s.avgFPS, s.sampleCount, nil
}

type estimateWithFramesStore struct {
	*stubStore
	source          *db.Source
	analysisResults []*db.AnalysisResult
	avgFPS          float64
	sampleCount     int64
}

func (s *estimateWithFramesStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return s.source, nil
}

func (s *estimateWithFramesStore) ListAnalysisResults(_ context.Context, _ string) ([]*db.AnalysisResult, error) {
	return s.analysisResults, nil
}

func (s *estimateWithFramesStore) GetAvgFPSStats(_ context.Context, _ string) (float64, int64, error) {
	return s.avgFPS, s.sampleCount, nil
}

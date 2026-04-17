package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// CreateSource
// ---------------------------------------------------------------------------

func TestCreateSource_PathOnly(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	returned := Source{
		ID:        "src-new",
		Path:      "/mnt/media/movie.mkv",
		Filename:  "movie.mkv",
		SizeBytes: 1073741824,
		State:     "pending",
		CreatedAt: now,
	}
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/sources" {
			t.Errorf("path = %q, want /api/v1/sources", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(envelopeResponse(t, returned, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.CreateSource(context.Background(), "/mnt/media/movie.mkv", "", "")
	if err != nil {
		t.Fatalf("CreateSource() error = %v", err)
	}
	if got.ID != "src-new" {
		t.Errorf("ID = %q, want src-new", got.ID)
	}
	if gotBody["path"] != "/mnt/media/movie.mkv" {
		t.Errorf("body path = %q, want /mnt/media/movie.mkv", gotBody["path"])
	}
	// name and cloud_uri should not be in the body when empty.
	if _, ok := gotBody["name"]; ok {
		t.Error("body should not include empty name key")
	}
	if _, ok := gotBody["cloud_uri"]; ok {
		t.Error("body should not include empty cloud_uri key")
	}
}

func TestCreateSource_WithCloudURI(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	returned := Source{ID: "src-cloud", State: "pending", CreatedAt: now}
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(envelopeResponse(t, returned, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.CreateSource(context.Background(), "", "Cloud Movie", "s3://bucket/movie.mkv")
	if err != nil {
		t.Fatalf("CreateSource() error = %v", err)
	}
	if gotBody["name"] != "Cloud Movie" {
		t.Errorf("body name = %q, want Cloud Movie", gotBody["name"])
	}
	if gotBody["cloud_uri"] != "s3://bucket/movie.mkv" {
		t.Errorf("body cloud_uri = %q, want s3://bucket/movie.mkv", gotBody["cloud_uri"])
	}
	if _, ok := gotBody["path"]; ok {
		t.Error("body should not include empty path key")
	}
}

func TestCreateSource_ErrorResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write(problemResponse(t, http.StatusUnprocessableEntity, "Validation Error", "path is required"))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.CreateSource(context.Background(), "", "", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got != nil {
		t.Errorf("expected nil source on error, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// GetSource
// ---------------------------------------------------------------------------

func TestGetSource_ReturnsSource(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	dur := 7200.0
	src := Source{
		ID:          "src-get",
		Path:        "/mnt/media/film.mkv",
		Filename:    "film.mkv",
		SizeBytes:   5368709120,
		DurationSec: &dur,
		State:       "ready",
		CreatedAt:   now,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sources/src-get" {
			t.Errorf("path = %q, want /api/v1/sources/src-get", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, src, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.GetSource(context.Background(), "src-get")
	if err != nil {
		t.Fatalf("GetSource() error = %v", err)
	}
	if got.ID != "src-get" {
		t.Errorf("ID = %q, want src-get", got.ID)
	}
	if got.DurationSec == nil || *got.DurationSec != dur {
		t.Errorf("DurationSec = %v, want %v", got.DurationSec, dur)
	}
}

// ---------------------------------------------------------------------------
// DeleteSource
// ---------------------------------------------------------------------------

func TestDeleteSource_SendsDeleteAndReturnsNilOnSuccess(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL)
	if err := c.DeleteSource(context.Background(), "src-del"); err != nil {
		t.Fatalf("DeleteSource() error = %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/api/v1/sources/src-del" {
		t.Errorf("path = %q, want /api/v1/sources/src-del", gotPath)
	}
}

// ---------------------------------------------------------------------------
// AnalyzeSource
// ---------------------------------------------------------------------------

func TestAnalyzeSource_ReturnsJob(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	job := Job{ID: "j-analyze", JobType: "analyze", Status: JobQueued, CreatedAt: now, UpdatedAt: now}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/sources/src-1/analyze" {
			t.Errorf("path = %q, want /api/v1/sources/src-1/analyze", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, job, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.AnalyzeSource(context.Background(), "src-1")
	if err != nil {
		t.Fatalf("AnalyzeSource() error = %v", err)
	}
	if got.ID != "j-analyze" {
		t.Errorf("job.ID = %q, want j-analyze", got.ID)
	}
}

// ---------------------------------------------------------------------------
// GetSourceScenes
// ---------------------------------------------------------------------------

func TestGetSourceScenes_ReturnsSceneData(t *testing.T) {
	t.Parallel()
	sceneData := SceneData{
		SourceID:    "src-s",
		FPS:         23.976,
		TotalFrames: 170000,
		DurationSec: 7089.3,
		Scenes: []SceneBoundary{
			{Frame: 0, PTS: 0.0, Timecode: "00:00:00.000"},
			{Frame: 500, PTS: 20.85, Timecode: "00:00:20.850"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sources/src-s/scenes" {
			t.Errorf("path = %q, want /api/v1/sources/src-s/scenes", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, sceneData, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.GetSourceScenes(context.Background(), "src-s")
	if err != nil {
		t.Fatalf("GetSourceScenes() error = %v", err)
	}
	if got.SourceID != "src-s" {
		t.Errorf("SourceID = %q, want src-s", got.SourceID)
	}
	if got.FPS != 23.976 {
		t.Errorf("FPS = %v, want 23.976", got.FPS)
	}
	if len(got.Scenes) != 2 {
		t.Fatalf("len(Scenes) = %d, want 2", len(got.Scenes))
	}
	if got.Scenes[1].Frame != 500 {
		t.Errorf("Scenes[1].Frame = %d, want 500", got.Scenes[1].Frame)
	}
}

// ---------------------------------------------------------------------------
// ListSourcesPaged
// ---------------------------------------------------------------------------

func TestListSourcesPaged_PassesParams(t *testing.T) {
	t.Parallel()
	var gotQuery string
	meta := map[string]any{"total_count": float64(100), "request_id": "r1", "next_cursor": "tok"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, []Source{}, meta))
	}))
	defer srv.Close()

	c := New(srv.URL)
	col, err := c.ListSourcesPaged(context.Background(), "ready", "my-cursor", 10)
	if err != nil {
		t.Fatalf("ListSourcesPaged() error = %v", err)
	}
	for _, want := range []string{"state=ready", "cursor=my-cursor", "page_size=10"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
	if col.TotalCount != 100 {
		t.Errorf("TotalCount = %d, want 100", col.TotalCount)
	}
	if col.NextCursor != "tok" {
		t.Errorf("NextCursor = %q, want tok", col.NextCursor)
	}
}

// ---------------------------------------------------------------------------
// GetSourceSubtitles
// ---------------------------------------------------------------------------

func TestGetSourceSubtitles_ReturnsTracks(t *testing.T) {
	t.Parallel()
	resp := SubtitlesResponse{
		SourceID: "src-sub",
		Tracks: []SubtitleTrack{
			{Index: 0, Language: "eng", Codec: "subrip", Title: "English"},
			{Index: 1, Language: "fra", Codec: "ass", Title: "French"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sources/src-sub/subtitles" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, resp, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.GetSourceSubtitles(context.Background(), "src-sub")
	if err != nil {
		t.Fatalf("GetSourceSubtitles() error = %v", err)
	}
	if got.SourceID != "src-sub" {
		t.Errorf("SourceID = %q, want src-sub", got.SourceID)
	}
	if len(got.Tracks) != 2 {
		t.Fatalf("len(Tracks) = %d, want 2", len(got.Tracks))
	}
	if got.Tracks[0].Language != "eng" {
		t.Errorf("Tracks[0].Language = %q, want eng", got.Tracks[0].Language)
	}
}

// ---------------------------------------------------------------------------
// UpdateSourceHDR
// ---------------------------------------------------------------------------

func TestUpdateSourceHDR_SendsCorrectBody(t *testing.T) {
	t.Parallel()
	var gotBody map[string]any
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	src := Source{ID: "src-hdr", HDRType: "HDR10", DVProfile: 0, CreatedAt: now}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %q, want PATCH", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, src, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.UpdateSourceHDR(context.Background(), "src-hdr", "HDR10", 0)
	if err != nil {
		t.Fatalf("UpdateSourceHDR() error = %v", err)
	}
	if gotBody["hdr_type"] != "HDR10" {
		t.Errorf("body hdr_type = %v, want HDR10", gotBody["hdr_type"])
	}
	if gotBody["dv_profile"] != float64(0) {
		t.Errorf("body dv_profile = %v, want 0", gotBody["dv_profile"])
	}
}

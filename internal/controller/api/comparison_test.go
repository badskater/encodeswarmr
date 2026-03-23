package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/badskater/encodeswarmr/internal/db"
)

// decodeComparison decodes the JSON envelope and returns the comparison payload.
func decodeComparison(t *testing.T, rr *httptest.ResponseRecorder) ComparisonResponse {
	t.Helper()
	var body struct {
		Data ComparisonResponse `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode comparison response: %v", err)
	}
	return body.Data
}

// ---------------------------------------------------------------------------
// handleGetJobComparison
// ---------------------------------------------------------------------------

func TestHandleGetJobComparison_MissingID_Returns400(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs//comparison", nil)
	// PathValue("id") is empty string when not set.
	srv.handleGetJobComparison(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestHandleGetJobComparison_JobNotFound_Returns404(t *testing.T) {
	store := &comparisonJobNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/missing/comparison", nil)
	req.SetPathValue("id", "missing")
	srv.handleGetJobComparison(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestHandleGetJobComparison_StoreError_Returns500(t *testing.T) {
	store := &comparisonJobStoreErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/j1/comparison", nil)
	req.SetPathValue("id", "j1")
	srv.handleGetJobComparison(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

func TestHandleGetJobComparison_NoTasks_EmptyMetrics(t *testing.T) {
	store := &comparisonSuccessStore{
		stubStore: &stubStore{},
		job:       &db.Job{ID: "j1", SourceID: "s1", Status: "completed"},
		tasks:     nil,
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/j1/comparison", nil)
	req.SetPathValue("id", "j1")
	srv.handleGetJobComparison(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	resp := decodeComparison(t, rr)

	if resp.VMafScore != nil {
		t.Errorf("VMafScore = %v, want nil (no tasks)", resp.VMafScore)
	}
	if resp.PSNR != nil {
		t.Errorf("PSNR = %v, want nil (no tasks)", resp.PSNR)
	}
	if resp.SSIM != nil {
		t.Errorf("SSIM = %v, want nil (no tasks)", resp.SSIM)
	}
	if resp.Output.FileSizeMB != 0 {
		t.Errorf("Output.FileSizeMB = %v, want 0", resp.Output.FileSizeMB)
	}
}

func TestHandleGetJobComparison_WithCompletedEncodeTasks(t *testing.T) {
	outputSize := int64(50_000_000) // 50 MB
	durationSec := int64(120)       // 2 minutes
	vmaf := 94.5
	psnr := 42.1
	ssim := 0.98

	tasks := []*db.Task{
		{
			ID:          "t1",
			JobID:       "j1",
			Status:      "completed",
			TaskType:    db.TaskTypeEncode,
			OutputSize:  &outputSize,
			DurationSec: &durationSec,
			VMafScore:   &vmaf,
			PSNR:        &psnr,
			SSIM:        &ssim,
		},
	}

	store := &comparisonSuccessStore{
		stubStore: &stubStore{},
		job:       &db.Job{ID: "j1", SourceID: "s1", Status: "completed"},
		tasks:     tasks,
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/j1/comparison", nil)
	req.SetPathValue("id", "j1")
	srv.handleGetJobComparison(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	resp := decodeComparison(t, rr)

	if resp.VMafScore == nil {
		t.Fatal("VMafScore is nil, expected value")
	}
	if *resp.VMafScore != vmaf {
		t.Errorf("VMafScore = %v, want %v", *resp.VMafScore, vmaf)
	}
	if resp.PSNR == nil {
		t.Fatal("PSNR is nil, expected value")
	}
	if resp.SSIM == nil {
		t.Fatal("SSIM is nil, expected value")
	}
	// Output file size should be non-zero.
	if resp.Output.FileSizeMB <= 0 {
		t.Errorf("Output.FileSizeMB = %v, want > 0", resp.Output.FileSizeMB)
	}
}

func TestHandleGetJobComparison_ConcatTasksExcluded(t *testing.T) {
	outputSize := int64(100_000_000)
	vmaf := 95.0

	tasks := []*db.Task{
		{
			ID:         "t1",
			JobID:      "j1",
			Status:     "completed",
			TaskType:   db.TaskTypeConcat, // concat — must be excluded
			OutputSize: &outputSize,
			VMafScore:  &vmaf,
		},
	}

	store := &comparisonSuccessStore{
		stubStore: &stubStore{},
		job:       &db.Job{ID: "j1", SourceID: "s1", Status: "completed"},
		tasks:     tasks,
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/j1/comparison", nil)
	req.SetPathValue("id", "j1")
	srv.handleGetJobComparison(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	resp := decodeComparison(t, rr)

	// Concat tasks should be excluded — no VMAF, no output size.
	if resp.VMafScore != nil {
		t.Errorf("VMafScore = %v, want nil (concat tasks excluded)", resp.VMafScore)
	}
	if resp.Output.FileSizeMB != 0 {
		t.Errorf("Output.FileSizeMB = %v, want 0 (concat tasks excluded)", resp.Output.FileSizeMB)
	}
}

func TestHandleGetJobComparison_NonCompletedTasksExcluded(t *testing.T) {
	outputSize := int64(50_000_000)
	vmaf := 90.0

	tasks := []*db.Task{
		{
			ID:         "t1",
			JobID:      "j1",
			Status:     "running", // not completed
			TaskType:   db.TaskTypeEncode,
			OutputSize: &outputSize,
			VMafScore:  &vmaf,
		},
	}

	store := &comparisonSuccessStore{
		stubStore: &stubStore{},
		job:       &db.Job{ID: "j1", SourceID: "s1", Status: "running"},
		tasks:     tasks,
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/j1/comparison", nil)
	req.SetPathValue("id", "j1")
	srv.handleGetJobComparison(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	resp := decodeComparison(t, rr)

	if resp.VMafScore != nil {
		t.Errorf("VMafScore = %v, want nil (non-completed task excluded)", resp.VMafScore)
	}
}

func TestHandleGetJobComparison_ListTasksError_Returns500(t *testing.T) {
	store := &comparisonListTasksErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/j1/comparison", nil)
	req.SetPathValue("id", "j1")
	srv.handleGetJobComparison(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// formatResolution helper
// ---------------------------------------------------------------------------

func TestFormatResolution(t *testing.T) {
	tests := []struct {
		w, h int
		want string
	}{
		{1920, 1080, "1920x1080"},
		{3840, 2160, "3840x2160"},
		{0, 1080, ""},
		{1920, 0, ""},
		{0, 0, ""},
	}
	for _, tt := range tests {
		got := formatResolution(tt.w, tt.h)
		if got != tt.want {
			t.Errorf("formatResolution(%d, %d) = %q, want %q", tt.w, tt.h, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// store stubs
// ---------------------------------------------------------------------------

type comparisonJobNotFoundStore struct{ *stubStore }

func (s *comparisonJobNotFoundStore) GetJobByID(_ context.Context, _ string) (*db.Job, error) {
	return nil, db.ErrNotFound
}

type comparisonJobStoreErrStore struct{ *stubStore }

func (s *comparisonJobStoreErrStore) GetJobByID(_ context.Context, _ string) (*db.Job, error) {
	return nil, errTestDB
}

type comparisonSuccessStore struct {
	*stubStore
	job    *db.Job
	tasks  []*db.Task
	source *db.Source
}

func (s *comparisonSuccessStore) GetJobByID(_ context.Context, _ string) (*db.Job, error) {
	return s.job, nil
}

func (s *comparisonSuccessStore) ListTasksByJob(_ context.Context, _ string) ([]*db.Task, error) {
	return s.tasks, nil
}

// GetSourceByID returns the source if set, or ErrNotFound so the handler
// uses a zero-value source size rather than dereferencing nil.
func (s *comparisonSuccessStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	if s.source != nil {
		return s.source, nil
	}
	return nil, db.ErrNotFound
}

type comparisonListTasksErrStore struct{ *stubStore }

func (s *comparisonListTasksErrStore) GetJobByID(_ context.Context, _ string) (*db.Job, error) {
	return &db.Job{ID: "j1", SourceID: "s1"}, nil
}

func (s *comparisonListTasksErrStore) ListTasksByJob(_ context.Context, _ string) ([]*db.Task, error) {
	return nil, errTestDB
}

func (s *comparisonListTasksErrStore) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return nil, db.ErrNotFound
}

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
// JobExportURL / ArchivedJobExportURL — pure URL builders, no HTTP
// ---------------------------------------------------------------------------

func TestJobExportURL_AllParams(t *testing.T) {
	t.Parallel()
	c := New("http://controller:8080")
	got := c.JobExportURL("csv", "completed", "2024-01-01", "2024-01-31")
	if !strings.HasPrefix(got, "http://controller:8080/api/v1/jobs/export?") {
		t.Errorf("JobExportURL prefix = %q, want /api/v1/jobs/export?...", got)
	}
	for _, want := range []string{"format=csv", "status=completed", "from=2024-01-01", "to=2024-01-31"} {
		if !strings.Contains(got, want) {
			t.Errorf("JobExportURL missing %q in %q", want, got)
		}
	}
}

func TestJobExportURL_EmptyParams(t *testing.T) {
	t.Parallel()
	c := New("http://controller:8080")
	got := c.JobExportURL("", "", "", "")
	// All params empty — buildQuery returns "" so no query string.
	want := "http://controller:8080/api/v1/jobs/export"
	if got != want {
		t.Errorf("JobExportURL(all empty) = %q, want %q", got, want)
	}
}

func TestArchivedJobExportURL_AllParams(t *testing.T) {
	t.Parallel()
	c := New("http://controller:9090")
	got := c.ArchivedJobExportURL("json", "failed", "2024-06-01", "2024-06-30")
	if !strings.HasPrefix(got, "http://controller:9090/api/v1/archive/jobs/export?") {
		t.Errorf("ArchivedJobExportURL prefix = %q", got)
	}
	for _, want := range []string{"format=json", "status=failed"} {
		if !strings.Contains(got, want) {
			t.Errorf("ArchivedJobExportURL missing %q in %q", want, got)
		}
	}
}

func TestArchivedJobExportURL_RespectsBaseURL(t *testing.T) {
	t.Parallel()
	c := New("https://prod.example.com/")
	got := c.ArchivedJobExportURL("csv", "", "", "")
	if !strings.HasPrefix(got, "https://prod.example.com/api/v1/archive/jobs/export") {
		t.Errorf("ArchivedJobExportURL = %q, want https://prod.example.com/api/v1/archive/jobs/export", got)
	}
}

// ---------------------------------------------------------------------------
// ListJobs
// ---------------------------------------------------------------------------

func TestListJobs_ReturnsList(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	jobs := []Job{
		{ID: "j1", Status: JobQueued, CreatedAt: now, UpdatedAt: now},
		{ID: "j2", Status: JobRunning, CreatedAt: now, UpdatedAt: now},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/jobs" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, jobs, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.ListJobs(context.Background(), "", "")
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(jobs) = %d, want 2", len(got))
	}
	if got[0].ID != "j1" {
		t.Errorf("jobs[0].ID = %q, want j1", got[0].ID)
	}
}

func TestListJobs_WithStatusFilter(t *testing.T) {
	t.Parallel()
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, []Job{}, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, _ = c.ListJobs(context.Background(), "running", "")
	if !strings.Contains(gotQuery, "status=running") {
		t.Errorf("query %q does not contain status=running", gotQuery)
	}
}

func TestListJobs_ErrorResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write(problemResponse(t, http.StatusUnauthorized, "Unauthorized", ""))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.ListJobs(context.Background(), "", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got != nil {
		t.Errorf("expected nil slice on error, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// GetJob
// ---------------------------------------------------------------------------

func TestGetJob_ReturnsDetail(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	detail := JobDetail{
		Job:   Job{ID: "job-42", Status: JobCompleted, CreatedAt: now, UpdatedAt: now},
		Tasks: []Task{{ID: "t-1", JobID: "job-42", Status: TaskCompleted, CreatedAt: now}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/jobs/job-42" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, detail, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.GetJob(context.Background(), "job-42")
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if got.Job.ID != "job-42" {
		t.Errorf("Job.ID = %q, want job-42", got.Job.ID)
	}
	if len(got.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1", len(got.Tasks))
	}
}

func TestGetJob_ErrorResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(problemResponse(t, http.StatusNotFound, "Not Found", "job not found"))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.GetJob(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got != nil {
		t.Errorf("expected nil on error, got %v", got)
	}
	p, ok := err.(*Problem)
	if !ok {
		t.Fatalf("expected *Problem, got %T", err)
	}
	if p.Title != "Not Found" {
		t.Errorf("Title = %q, want Not Found", p.Title)
	}
}

// ---------------------------------------------------------------------------
// CreateJob
// ---------------------------------------------------------------------------

func TestCreateJob_SendsBodyAndReturnsJob(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	returnedJob := Job{ID: "new-job", Status: JobQueued, CreatedAt: now, UpdatedAt: now}

	var gotReq CreateJobRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/jobs" {
			t.Errorf("path = %q, want /api/v1/jobs", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(envelopeResponse(t, returnedJob, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	req := &CreateJobRequest{
		SourceID: "src-1",
		JobType:  "encode",
	}
	got, err := c.CreateJob(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	if got.ID != "new-job" {
		t.Errorf("ID = %q, want new-job", got.ID)
	}
	if gotReq.SourceID != "src-1" {
		t.Errorf("body SourceID = %q, want src-1", gotReq.SourceID)
	}
}

// ---------------------------------------------------------------------------
// CancelJob / RetryJob — fire-and-forget (nil result)
// ---------------------------------------------------------------------------

func TestCancelJob_Success(t *testing.T) {
	t.Parallel()
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL)
	if err := c.CancelJob(context.Background(), "j-abc"); err != nil {
		t.Fatalf("CancelJob() error = %v", err)
	}
	if gotPath != "/api/v1/jobs/j-abc/cancel" {
		t.Errorf("path = %q, want /api/v1/jobs/j-abc/cancel", gotPath)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
}

func TestRetryJob_Success(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL)
	if err := c.RetryJob(context.Background(), "j-failed"); err != nil {
		t.Fatalf("RetryJob() error = %v", err)
	}
	if gotPath != "/api/v1/jobs/j-failed/retry" {
		t.Errorf("path = %q, want /api/v1/jobs/j-failed/retry", gotPath)
	}
}

// ---------------------------------------------------------------------------
// UpdateJobPriority
// ---------------------------------------------------------------------------

func TestUpdateJobPriority_SendsBody(t *testing.T) {
	t.Parallel()
	var gotBody map[string]int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want PUT", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL)
	if err := c.UpdateJobPriority(context.Background(), "j-1", 10); err != nil {
		t.Fatalf("UpdateJobPriority() error = %v", err)
	}
	if gotBody["priority"] != 10 {
		t.Errorf("body priority = %d, want 10", gotBody["priority"])
	}
}

// ---------------------------------------------------------------------------
// ListJobsPaged
// ---------------------------------------------------------------------------

func TestListJobsPaged_PassesPageSize(t *testing.T) {
	t.Parallel()
	var gotQuery string
	meta := map[string]any{"total_count": float64(5), "request_id": "r1"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, []Job{}, meta))
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, _ = c.ListJobsPaged(context.Background(), "queued", "", "", 50)
	if !strings.Contains(gotQuery, "page_size=50") {
		t.Errorf("query %q does not contain page_size=50", gotQuery)
	}
}

func TestListJobsPaged_ZeroPageSize_NotIncluded(t *testing.T) {
	t.Parallel()
	var gotQuery string
	meta := map[string]any{"total_count": float64(0), "request_id": "r2"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, []Job{}, meta))
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, _ = c.ListJobsPaged(context.Background(), "", "", "", 0)
	// page_size=0 should be skipped by buildQuery (empty string).
	if strings.Contains(gotQuery, "page_size") {
		t.Errorf("query %q should not contain page_size when pageSize=0, got %q", gotQuery, gotQuery)
	}
}

// ---------------------------------------------------------------------------
// GetJobComparison
// ---------------------------------------------------------------------------

func TestGetJobComparison_ReturnsMetrics(t *testing.T) {
	t.Parallel()
	vmaf := 94.2
	cmp := ComparisonResponse{
		Source:           ComparisonSource{DurationSec: 3600, FileSizeMB: 5000},
		Output:           ComparisonSource{DurationSec: 3600, FileSizeMB: 1200},
		CompressionRatio: 4.17,
		SizeReductionPct: 76.0,
		VMafScore:        &vmaf,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/jobs/j-cmp/comparison" {
			t.Errorf("path = %q, want /api/v1/jobs/j-cmp/comparison", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(envelopeResponse(t, cmp, nil))
	}))
	defer srv.Close()

	c := New(srv.URL)
	got, err := c.GetJobComparison(context.Background(), "j-cmp")
	if err != nil {
		t.Fatalf("GetJobComparison() error = %v", err)
	}
	if got.VMafScore == nil || *got.VMafScore != vmaf {
		t.Errorf("VMafScore = %v, want %v", got.VMafScore, vmaf)
	}
	if got.CompressionRatio != 4.17 {
		t.Errorf("CompressionRatio = %v, want 4.17", got.CompressionRatio)
	}
}

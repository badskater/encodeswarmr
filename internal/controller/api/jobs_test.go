package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/badskater/encodeswarmr/internal/controller/webhooks"
	"github.com/badskater/encodeswarmr/internal/db"
	"github.com/badskater/encodeswarmr/internal/db/teststore"
)

// ---------------------------------------------------------------------------
// stubStore — zero-value base implementing db.Store.
// Every method returns zero values.  Test-specific overrides are applied by
// embedding stubStore in a concrete struct that shadows the needed methods.
// ---------------------------------------------------------------------------

type stubStore struct {
	teststore.Stub
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newTestServer(store db.Store) *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wh := webhooks.New(store, webhooks.Config{}, logger)
	return &Server{
		store:    store,
		logger:   logger,
		webhooks: wh,
	}
}

// decodeJSON is a test helper that decodes the recorder body into v.
func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rr.Body).Decode(v); err != nil {
		t.Fatalf("decode JSON body: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestHandleListJobs
// ---------------------------------------------------------------------------

func TestHandleListJobs(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := &listJobsStore{
			stubStore: &stubStore{},
			jobs: []*db.Job{
				{ID: "j1", SourceID: "s1", Status: "queued", JobType: "encode"},
				{ID: "j2", SourceID: "s2", Status: "running", JobType: "analysis"},
			},
			total: 2,
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
		srv.handleListJobs(rr, req)

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
		tc, ok := body.Meta["total_count"].(float64)
		if !ok || tc != 2 {
			t.Errorf("meta.total_count = %v, want 2", body.Meta["total_count"])
		}
	})

	t.Run("bad page_size", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs?page_size=abc", nil)
		srv.handleListJobs(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid cursor", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs?cursor=!!!", nil)
		srv.handleListJobs(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})
}

type listJobsStore struct {
	*stubStore
	jobs  []*db.Job
	total int64
}

func (s *listJobsStore) ListJobs(_ context.Context, _ db.ListJobsFilter) ([]*db.Job, int64, error) {
	return s.jobs, s.total, nil
}

// ---------------------------------------------------------------------------
// TestHandleGetJob
// ---------------------------------------------------------------------------

func TestHandleGetJob(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := &getJobStore{
			stubStore: &stubStore{},
			job:       &db.Job{ID: "j1", SourceID: "s1", Status: "queued", JobType: "encode"},
			tasks: []*db.Task{
				{ID: "t1", JobID: "j1", ChunkIndex: 0, Status: "pending"},
				{ID: "t2", JobID: "j1", ChunkIndex: 1, Status: "pending"},
			},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/j1", nil)
		req.SetPathValue("id", "j1")
		srv.handleGetJob(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body struct {
			Data struct {
				Job   json.RawMessage   `json:"job"`
				Tasks []json.RawMessage `json:"tasks"`
			} `json:"data"`
		}
		decodeJSON(t, rr, &body)

		if body.Data.Job == nil {
			t.Error("data.job is nil")
		}
		if len(body.Data.Tasks) != 2 {
			t.Errorf("len(data.tasks) = %d, want 2", len(body.Data.Tasks))
		}
	})

	t.Run("not found", func(t *testing.T) {
		store := &getJobNotFoundStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/missing", nil)
		req.SetPathValue("id", "missing")
		srv.handleGetJob(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})
}

type getJobStore struct {
	*stubStore
	job   *db.Job
	tasks []*db.Task
}

func (s *getJobStore) GetJobByID(_ context.Context, _ string) (*db.Job, error) {
	return s.job, nil
}

func (s *getJobStore) ListTasksByJob(_ context.Context, _ string) ([]*db.Task, error) {
	return s.tasks, nil
}

type getJobNotFoundStore struct {
	*stubStore
}

func (s *getJobNotFoundStore) GetJobByID(_ context.Context, _ string) (*db.Job, error) {
	return nil, db.ErrNotFound
}

// ---------------------------------------------------------------------------
// TestHandleCreateJob
// ---------------------------------------------------------------------------

func TestHandleCreateJob(t *testing.T) {
	t.Run("missing source_id", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		body := `{"job_type":"encode"}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewBufferString(body))
		srv.handleCreateJob(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("invalid job_type", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		body := `{"source_id":"s1","job_type":"invalid"}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewBufferString(body))
		srv.handleCreateJob(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("encode without chunk_boundaries", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		body := `{
			"source_id": "s1",
			"job_type": "encode",
			"encode_config": {
				"run_script_template_id": "tmpl-1",
				"chunk_boundaries": []
			}
		}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewBufferString(body))
		srv.handleCreateJob(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("valid request", func(t *testing.T) {
		store := &createJobStore{
			stubStore: &stubStore{},
			created:   &db.Job{ID: "j-new", SourceID: "s1", Status: "queued", JobType: "encode"},
		}
		srv := newTestServer(store)

		body := `{
			"source_id": "s1",
			"job_type": "encode",
			"encode_config": {
				"run_script_template_id": "tmpl-1",
				"chunk_boundaries": [{"start_frame": 0, "end_frame": 1000}],
				"output_root": "\\\\nas\\output"
			}
		}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewBufferString(body))
		srv.handleCreateJob(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
		}

		var resp struct {
			Data struct {
				ID string `json:"ID"`
			} `json:"data"`
		}
		decodeJSON(t, rr, &resp)
		if resp.Data.ID != "j-new" {
			t.Errorf("data.ID = %q, want %q", resp.Data.ID, "j-new")
		}
	})
}

type createJobStore struct {
	*stubStore
	created *db.Job
}

func (s *createJobStore) CreateJob(_ context.Context, _ db.CreateJobParams) (*db.Job, error) {
	return s.created, nil
}

// ---------------------------------------------------------------------------
// TestHandleCancelJob
// ---------------------------------------------------------------------------

func TestHandleCancelJob(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		store := &cancelJobNotFoundStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/missing/cancel", nil)
		req.SetPathValue("id", "missing")
		srv.handleCancelJob(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("already cancelled", func(t *testing.T) {
		store := &cancelJobAlreadyStore{
			stubStore: &stubStore{},
			job:       &db.Job{ID: "j1", Status: "cancelled"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/j1/cancel", nil)
		req.SetPathValue("id", "j1")
		srv.handleCancelJob(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("success", func(t *testing.T) {
		store := &cancelJobSuccessStore{
			stubStore: &stubStore{},
			job:       &db.Job{ID: "j1", SourceID: "s1", Status: "queued"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/j1/cancel", nil)
		req.SetPathValue("id", "j1")
		srv.handleCancelJob(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body struct {
			Data struct {
				OK bool `json:"ok"`
			} `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if !body.Data.OK {
			t.Error("expected data.ok to be true")
		}
	})
}

type cancelJobNotFoundStore struct {
	*stubStore
}

func (s *cancelJobNotFoundStore) GetJobByID(_ context.Context, _ string) (*db.Job, error) {
	return nil, db.ErrNotFound
}

type cancelJobAlreadyStore struct {
	*stubStore
	job *db.Job
}

func (s *cancelJobAlreadyStore) GetJobByID(_ context.Context, _ string) (*db.Job, error) {
	return s.job, nil
}

type cancelJobSuccessStore struct {
	*stubStore
	job *db.Job
}

func (s *cancelJobSuccessStore) GetJobByID(_ context.Context, _ string) (*db.Job, error) {
	return s.job, nil
}

// ---------------------------------------------------------------------------
// TestHandleRetryJob
// ---------------------------------------------------------------------------

func TestHandleRetryJob(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		store := &retryJobNotFoundStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/missing/retry", nil)
		req.SetPathValue("id", "missing")
		srv.handleRetryJob(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("non-failed job returns 422", func(t *testing.T) {
		store := &retryJobRunningStore{
			stubStore: &stubStore{},
			job:       &db.Job{ID: "j1", Status: "running"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/j1/retry", nil)
		req.SetPathValue("id", "j1")
		srv.handleRetryJob(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("queued job returns 422", func(t *testing.T) {
		store := &retryJobRunningStore{
			stubStore: &stubStore{},
			job:       &db.Job{ID: "j1", Status: "queued"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/j1/retry", nil)
		req.SetPathValue("id", "j1")
		srv.handleRetryJob(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("failed job retried successfully", func(t *testing.T) {
		store := &retryJobSuccessStore{
			stubStore: &stubStore{},
			job:       &db.Job{ID: "j1", SourceID: "s1", Status: "failed"},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/j1/retry", nil)
		req.SetPathValue("id", "j1")
		srv.handleRetryJob(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body struct {
			Data struct {
				OK bool `json:"ok"`
			} `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if !body.Data.OK {
			t.Error("expected data.ok to be true")
		}
	})
}

type retryJobNotFoundStore struct{ *stubStore }

func (s *retryJobNotFoundStore) GetJobByID(_ context.Context, _ string) (*db.Job, error) {
	return nil, db.ErrNotFound
}

type retryJobRunningStore struct {
	*stubStore
	job *db.Job
}

func (s *retryJobRunningStore) GetJobByID(_ context.Context, _ string) (*db.Job, error) {
	return s.job, nil
}

type retryJobSuccessStore struct {
	*stubStore
	job *db.Job
}

func (s *retryJobSuccessStore) GetJobByID(_ context.Context, _ string) (*db.Job, error) {
	return s.job, nil
}

// ---------------------------------------------------------------------------
// TestHandleCreateJobChain
// ---------------------------------------------------------------------------

func TestHandleCreateJobChain(t *testing.T) {
	t.Run("valid chain creates jobs with depends_on links", func(t *testing.T) {
		var calls int
		store := &createChainStore{
			stubStore: &stubStore{},
			jobFn: func(params db.CreateJobParams) (*db.Job, error) {
				calls++
				id := fmt.Sprintf("j%d", calls)
				return &db.Job{
					ID:        id,
					SourceID:  params.SourceID,
					JobType:   params.JobType,
					Status:    "queued",
					DependsOn: params.DependsOn,
				}, nil
			},
		}
		srv := newTestServer(store)

		body := `{
			"source_id": "src-1",
			"steps": [
				{"job_type": "analysis"},
				{"job_type": "encode"},
				{"job_type": "audio"}
			]
		}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/job-chains", bytes.NewBufferString(body))
		srv.handleCreateJobChain(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
		}

		var resp struct {
			Data struct {
				ChainGroup string     `json:"chain_group"`
				Jobs       []*db.Job  `json:"jobs"`
			} `json:"data"`
		}
		decodeJSON(t, rr, &resp)

		if len(resp.Data.Jobs) != 3 {
			t.Fatalf("len(jobs) = %d, want 3", len(resp.Data.Jobs))
		}
		if resp.Data.ChainGroup == "" {
			t.Error("chain_group should be non-empty")
		}

		// Second job should depend on the first.
		if resp.Data.Jobs[1].DependsOn == nil || *resp.Data.Jobs[1].DependsOn != "j1" {
			t.Errorf("jobs[1].depends_on = %v, want 'j1'", resp.Data.Jobs[1].DependsOn)
		}
		// Third job should depend on the second.
		if resp.Data.Jobs[2].DependsOn == nil || *resp.Data.Jobs[2].DependsOn != "j2" {
			t.Errorf("jobs[2].depends_on = %v, want 'j2'", resp.Data.Jobs[2].DependsOn)
		}
	})

	t.Run("missing source_id returns 422", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		body := `{"steps": [{"job_type": "analysis"}]}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/job-chains", bytes.NewBufferString(body))
		srv.handleCreateJobChain(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422", rr.Code)
		}
	})

	t.Run("empty steps returns 422", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		body := `{"source_id": "src-1", "steps": []}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/job-chains", bytes.NewBufferString(body))
		srv.handleCreateJobChain(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422", rr.Code)
		}
	})

	t.Run("invalid job_type in step returns 422", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		body := `{"source_id": "src-1", "steps": [{"job_type": "bad-type"}]}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/job-chains", bytes.NewBufferString(body))
		srv.handleCreateJobChain(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422", rr.Code)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &createChainErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		body := `{"source_id": "src-1", "steps": [{"job_type": "analysis"}]}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/job-chains", bytes.NewBufferString(body))
		srv.handleCreateJobChain(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rr.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleGetJobChain
// ---------------------------------------------------------------------------

func TestHandleGetJobChain(t *testing.T) {
	t.Run("returns chain members in order", func(t *testing.T) {
		j1 := &db.Job{ID: "j1", SourceID: "s1", JobType: "analysis", Status: "completed"}
		j2 := &db.Job{ID: "j2", SourceID: "s1", JobType: "encode", Status: "queued", DependsOn: &j1.ID}
		store := &getChainStore{
			stubStore: &stubStore{},
			jobs:      []*db.Job{j1, j2},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/job-chains/chain-abc", nil)
		req.SetPathValue("chain_group", "chain-abc")
		srv.handleGetJobChain(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}

		var resp struct {
			Data struct {
				ChainGroup string    `json:"chain_group"`
				Jobs       []db.Job  `json:"jobs"`
			} `json:"data"`
		}
		decodeJSON(t, rr, &resp)
		if resp.Data.ChainGroup != "chain-abc" {
			t.Errorf("chain_group = %q, want chain-abc", resp.Data.ChainGroup)
		}
		if len(resp.Data.Jobs) != 2 {
			t.Fatalf("len(jobs) = %d, want 2", len(resp.Data.Jobs))
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &getChainErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/job-chains/chain-abc", nil)
		req.SetPathValue("chain_group", "chain-abc")
		srv.handleGetJobChain(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rr.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// store stubs for job chain tests
// ---------------------------------------------------------------------------

type createChainStore struct {
	*stubStore
	jobFn func(db.CreateJobParams) (*db.Job, error)
}

func (s *createChainStore) CreateJob(_ context.Context, p db.CreateJobParams) (*db.Job, error) {
	return s.jobFn(p)
}

type createChainErrStore struct{ *stubStore }

func (s *createChainErrStore) CreateJob(_ context.Context, _ db.CreateJobParams) (*db.Job, error) {
	return nil, errors.New("db error")
}

type getChainStore struct {
	*stubStore
	jobs []*db.Job
}

func (s *getChainStore) ListJobsByChainGroup(_ context.Context, _ string) ([]*db.Job, error) {
	return s.jobs, nil
}

type getChainErrStore struct{ *stubStore }

func (s *getChainErrStore) ListJobsByChainGroup(_ context.Context, _ string) ([]*db.Job, error) {
	return nil, errors.New("db error")
}

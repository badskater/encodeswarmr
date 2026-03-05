package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/badskater/distributed-encoder/internal/controller/webhooks"
	"github.com/badskater/distributed-encoder/internal/db"
)

// ---------------------------------------------------------------------------
// stubStore — zero-value base implementing db.Store.
// Every method returns zero values.  Test-specific overrides are applied by
// embedding stubStore in a concrete struct that shadows the needed methods.
// ---------------------------------------------------------------------------

type stubStore struct{}

func (s *stubStore) CreateUser(context.Context, db.CreateUserParams) (*db.User, error) {
	return nil, nil
}
func (s *stubStore) GetUserByUsername(context.Context, string) (*db.User, error) {
	return nil, nil
}
func (s *stubStore) GetUserByOIDCSub(context.Context, string) (*db.User, error) {
	return nil, nil
}
func (s *stubStore) GetUserByID(context.Context, string) (*db.User, error) { return nil, nil }
func (s *stubStore) ListUsers(context.Context) ([]*db.User, error)         { return nil, nil }
func (s *stubStore) UpdateUserRole(context.Context, string, string) error   { return nil }
func (s *stubStore) DeleteUser(context.Context, string) error               { return nil }
func (s *stubStore) CountAdminUsers(context.Context) (int64, error)         { return 1, nil }

func (s *stubStore) UpsertAgent(context.Context, db.UpsertAgentParams) (*db.Agent, error) {
	return nil, nil
}
func (s *stubStore) GetAgentByID(context.Context, string) (*db.Agent, error)   { return nil, nil }
func (s *stubStore) GetAgentByName(context.Context, string) (*db.Agent, error) { return nil, nil }
func (s *stubStore) ListAgents(context.Context) ([]*db.Agent, error)           { return nil, nil }
func (s *stubStore) UpdateAgentStatus(context.Context, string, string) error   { return nil }
func (s *stubStore) UpdateAgentHeartbeat(context.Context, db.UpdateAgentHeartbeatParams) error {
	return nil
}
func (s *stubStore) SetAgentAPIKey(context.Context, string, string) error { return nil }
func (s *stubStore) MarkStaleAgents(context.Context, time.Duration) (int64, error) {
	return 0, nil
}

func (s *stubStore) CreateSource(context.Context, db.CreateSourceParams) (*db.Source, error) {
	return nil, nil
}
func (s *stubStore) GetSourceByID(context.Context, string) (*db.Source, error)      { return nil, nil }
func (s *stubStore) GetSourceByUNCPath(context.Context, string) (*db.Source, error) { return nil, nil }
func (s *stubStore) ListSources(context.Context, db.ListSourcesFilter) ([]*db.Source, int64, error) {
	return nil, 0, nil
}
func (s *stubStore) UpdateSourceState(context.Context, string, string) error  { return nil }
func (s *stubStore) UpdateSourceVMAF(context.Context, string, float64) error  { return nil }
func (s *stubStore) DeleteSource(context.Context, string) error               { return nil }

func (s *stubStore) CreateJob(context.Context, db.CreateJobParams) (*db.Job, error) {
	return nil, nil
}
func (s *stubStore) GetJobByID(context.Context, string) (*db.Job, error) { return nil, nil }
func (s *stubStore) ListJobs(context.Context, db.ListJobsFilter) ([]*db.Job, int64, error) {
	return nil, 0, nil
}
func (s *stubStore) UpdateJobStatus(context.Context, string, string) error { return nil }
func (s *stubStore) UpdateJobTaskCounts(context.Context, string) error     { return nil }
func (s *stubStore) GetJobsNeedingExpansion(context.Context) ([]*db.Job, error) {
	return nil, nil
}

func (s *stubStore) CreateTask(context.Context, db.CreateTaskParams) (*db.Task, error) {
	return nil, nil
}
func (s *stubStore) GetTaskByID(context.Context, string) (*db.Task, error) { return nil, nil }
func (s *stubStore) ListTasksByJob(context.Context, string) ([]*db.Task, error) {
	return nil, nil
}
func (s *stubStore) ClaimNextTask(context.Context, string, []string) (*db.Task, error) {
	return nil, nil
}
func (s *stubStore) UpdateTaskStatus(context.Context, string, string) error { return nil }
func (s *stubStore) SetTaskScriptDir(context.Context, string, string) error { return nil }
func (s *stubStore) CompleteTask(context.Context, db.CompleteTaskParams) error {
	return nil
}
func (s *stubStore) FailTask(context.Context, string, int, string) error    { return nil }
func (s *stubStore) CancelPendingTasksForJob(context.Context, string) error { return nil }

func (s *stubStore) InsertTaskLog(context.Context, db.InsertTaskLogParams) error { return nil }
func (s *stubStore) ListTaskLogs(context.Context, db.ListTaskLogsParams) ([]*db.TaskLog, error) {
	return nil, nil
}
func (s *stubStore) TailTaskLogs(context.Context, string, int64) ([]*db.TaskLog, error) {
	return nil, nil
}

func (s *stubStore) CreateTemplate(context.Context, db.CreateTemplateParams) (*db.Template, error) {
	return nil, nil
}
func (s *stubStore) GetTemplateByID(context.Context, string) (*db.Template, error) {
	return nil, nil
}
func (s *stubStore) ListTemplates(context.Context, string) ([]*db.Template, error) {
	return nil, nil
}
func (s *stubStore) UpdateTemplate(context.Context, db.UpdateTemplateParams) error { return nil }
func (s *stubStore) DeleteTemplate(context.Context, string) error                  { return nil }

func (s *stubStore) UpsertVariable(context.Context, db.UpsertVariableParams) (*db.Variable, error) {
	return nil, nil
}
func (s *stubStore) GetVariableByName(context.Context, string) (*db.Variable, error) {
	return nil, nil
}
func (s *stubStore) ListVariables(context.Context, string) ([]*db.Variable, error) {
	return nil, nil
}
func (s *stubStore) DeleteVariable(context.Context, string) error { return nil }

func (s *stubStore) CreateWebhook(context.Context, db.CreateWebhookParams) (*db.Webhook, error) {
	return nil, nil
}
func (s *stubStore) GetWebhookByID(context.Context, string) (*db.Webhook, error) {
	return nil, nil
}
func (s *stubStore) ListWebhooksByEvent(context.Context, string) ([]*db.Webhook, error) {
	return nil, nil
}
func (s *stubStore) ListWebhooks(context.Context) ([]*db.Webhook, error) { return nil, nil }
func (s *stubStore) UpdateWebhook(context.Context, db.UpdateWebhookParams) error {
	return nil
}
func (s *stubStore) DeleteWebhook(context.Context, string) error                      { return nil }
func (s *stubStore) InsertWebhookDelivery(context.Context, db.InsertWebhookDeliveryParams) error {
	return nil
}
func (s *stubStore) ListWebhookDeliveries(context.Context, string, int, int) ([]*db.WebhookDelivery, error) {
	return nil, nil
}

func (s *stubStore) UpsertAnalysisResult(context.Context, db.UpsertAnalysisResultParams) (*db.AnalysisResult, error) {
	return nil, nil
}
func (s *stubStore) GetAnalysisResult(context.Context, string, string) (*db.AnalysisResult, error) {
	return nil, nil
}
func (s *stubStore) ListAnalysisResults(context.Context, string) ([]*db.AnalysisResult, error) {
	return nil, nil
}

func (s *stubStore) CreateSession(context.Context, db.CreateSessionParams) (*db.Session, error) {
	return nil, nil
}
func (s *stubStore) GetSessionByToken(context.Context, string) (*db.Session, error) {
	return nil, nil
}
func (s *stubStore) DeleteSession(context.Context, string) error  { return nil }
func (s *stubStore) PruneExpiredSessions(context.Context) error   { return nil }

func (s *stubStore) CreateEnrollmentToken(context.Context, db.CreateEnrollmentTokenParams) (*db.EnrollmentToken, error) {
	return nil, nil
}
func (s *stubStore) GetEnrollmentToken(context.Context, string) (*db.EnrollmentToken, error) {
	return nil, nil
}
func (s *stubStore) ConsumeEnrollmentToken(context.Context, db.ConsumeEnrollmentTokenParams) error {
	return nil
}
func (s *stubStore) ListEnrollmentTokens(context.Context) ([]*db.EnrollmentToken, error) {
	return nil, nil
}
func (s *stubStore) DeleteEnrollmentToken(context.Context, string) error      { return nil }
func (s *stubStore) PruneExpiredEnrollmentTokens(context.Context) error       { return nil }

func (s *stubStore) RetryFailedTasksForJob(context.Context, string) error { return nil }
func (s *stubStore) ListJobLogs(context.Context, db.ListJobLogsParams) ([]*db.TaskLog, error) {
	return nil, nil
}
func (s *stubStore) PruneOldTaskLogs(context.Context, time.Time) error { return nil }

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

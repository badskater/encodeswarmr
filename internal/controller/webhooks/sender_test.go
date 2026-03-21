package webhooks

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/badskater/distributed-encoder/internal/db"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ---------------------------------------------------------------------------
// Minimal store stub for sender tests
// ---------------------------------------------------------------------------

type senderStubStore struct {
	deliveries []db.InsertWebhookDeliveryParams
}

func (s *senderStubStore) InsertWebhookDelivery(_ context.Context, p db.InsertWebhookDeliveryParams) error {
	s.deliveries = append(s.deliveries, p)
	return nil
}

// Satisfy the full db.Store interface with no-ops.
func (s *senderStubStore) CreateUser(context.Context, db.CreateUserParams) (*db.User, error) {
	return nil, nil
}
func (s *senderStubStore) GetUserByUsername(context.Context, string) (*db.User, error) {
	return nil, nil
}
func (s *senderStubStore) GetUserByOIDCSub(context.Context, string) (*db.User, error) {
	return nil, nil
}
func (s *senderStubStore) GetUserByID(context.Context, string) (*db.User, error) { return nil, nil }
func (s *senderStubStore) ListUsers(context.Context) ([]*db.User, error)          { return nil, nil }
func (s *senderStubStore) UpdateUserRole(context.Context, string, string) error   { return nil }
func (s *senderStubStore) DeleteUser(context.Context, string) error               { return nil }
func (s *senderStubStore) CountAdminUsers(context.Context) (int64, error)         { return 1, nil }

func (s *senderStubStore) UpsertAgent(context.Context, db.UpsertAgentParams) (*db.Agent, error) {
	return nil, nil
}
func (s *senderStubStore) GetAgentByID(context.Context, string) (*db.Agent, error)   { return nil, nil }
func (s *senderStubStore) GetAgentByName(context.Context, string) (*db.Agent, error) { return nil, nil }
func (s *senderStubStore) ListAgents(context.Context) ([]*db.Agent, error)           { return nil, nil }
func (s *senderStubStore) UpdateAgentStatus(context.Context, string, string) error   { return nil }
func (s *senderStubStore) UpdateAgentHeartbeat(context.Context, db.UpdateAgentHeartbeatParams) error {
	return nil
}
func (s *senderStubStore) UpdateAgentVNCPort(context.Context, string, int) error { return nil }
func (s *senderStubStore) SetAgentAPIKey(context.Context, string, string) error  { return nil }
func (s *senderStubStore) MarkStaleAgents(context.Context, time.Duration) (int64, error) {
	return 0, nil
}

func (s *senderStubStore) CreateSource(context.Context, db.CreateSourceParams) (*db.Source, error) {
	return nil, nil
}
func (s *senderStubStore) GetSourceByID(context.Context, string) (*db.Source, error)      { return nil, nil }
func (s *senderStubStore) GetSourceByUNCPath(context.Context, string) (*db.Source, error) { return nil, nil }
func (s *senderStubStore) ListSources(context.Context, db.ListSourcesFilter) ([]*db.Source, int64, error) {
	return nil, 0, nil
}
func (s *senderStubStore) UpdateSourceState(context.Context, string, string) error         { return nil }
func (s *senderStubStore) UpdateSourceVMAF(context.Context, string, float64) error         { return nil }
func (s *senderStubStore) UpdateSourceHDR(context.Context, db.UpdateSourceHDRParams) error { return nil }
func (s *senderStubStore) DeleteSource(context.Context, string) error                      { return nil }

func (s *senderStubStore) CreateJob(context.Context, db.CreateJobParams) (*db.Job, error) {
	return nil, nil
}
func (s *senderStubStore) GetJobByID(context.Context, string) (*db.Job, error) { return nil, nil }
func (s *senderStubStore) ListJobs(context.Context, db.ListJobsFilter) ([]*db.Job, int64, error) {
	return nil, 0, nil
}
func (s *senderStubStore) UpdateJobStatus(context.Context, string, string) error { return nil }
func (s *senderStubStore) UpdateJobTaskCounts(context.Context, string) error     { return nil }
func (s *senderStubStore) GetJobsNeedingExpansion(context.Context) ([]*db.Job, error) {
	return nil, nil
}

func (s *senderStubStore) CreateTask(context.Context, db.CreateTaskParams) (*db.Task, error) {
	return nil, nil
}
func (s *senderStubStore) GetTaskByID(context.Context, string) (*db.Task, error) { return nil, nil }
func (s *senderStubStore) ListTasksByJob(context.Context, string) ([]*db.Task, error) {
	return nil, nil
}
func (s *senderStubStore) ClaimNextTask(context.Context, string, []string) (*db.Task, error) {
	return nil, nil
}
func (s *senderStubStore) UpdateTaskStatus(context.Context, string, string) error { return nil }
func (s *senderStubStore) SetTaskScriptDir(context.Context, string, string) error { return nil }
func (s *senderStubStore) CompleteTask(context.Context, db.CompleteTaskParams) error {
	return nil
}
func (s *senderStubStore) FailTask(context.Context, string, int, string) error    { return nil }
func (s *senderStubStore) CancelPendingTasksForJob(context.Context, string) error { return nil }

func (s *senderStubStore) InsertTaskLog(context.Context, db.InsertTaskLogParams) error { return nil }
func (s *senderStubStore) ListTaskLogs(context.Context, db.ListTaskLogsParams) ([]*db.TaskLog, error) {
	return nil, nil
}
func (s *senderStubStore) TailTaskLogs(context.Context, string, int64) ([]*db.TaskLog, error) {
	return nil, nil
}

func (s *senderStubStore) CreateTemplate(context.Context, db.CreateTemplateParams) (*db.Template, error) {
	return nil, nil
}
func (s *senderStubStore) GetTemplateByID(context.Context, string) (*db.Template, error) {
	return nil, nil
}
func (s *senderStubStore) ListTemplates(context.Context, string) ([]*db.Template, error) {
	return nil, nil
}
func (s *senderStubStore) UpdateTemplate(context.Context, db.UpdateTemplateParams) error { return nil }
func (s *senderStubStore) DeleteTemplate(context.Context, string) error                  { return nil }

func (s *senderStubStore) UpsertVariable(context.Context, db.UpsertVariableParams) (*db.Variable, error) {
	return nil, nil
}
func (s *senderStubStore) GetVariableByName(context.Context, string) (*db.Variable, error) {
	return nil, nil
}
func (s *senderStubStore) ListVariables(context.Context, string) ([]*db.Variable, error) {
	return nil, nil
}
func (s *senderStubStore) DeleteVariable(context.Context, string) error { return nil }

func (s *senderStubStore) CreateWebhook(context.Context, db.CreateWebhookParams) (*db.Webhook, error) {
	return nil, nil
}
func (s *senderStubStore) GetWebhookByID(context.Context, string) (*db.Webhook, error) {
	return nil, nil
}
func (s *senderStubStore) ListWebhooksByEvent(context.Context, string) ([]*db.Webhook, error) {
	return nil, nil
}
func (s *senderStubStore) ListWebhooks(context.Context) ([]*db.Webhook, error) { return nil, nil }
func (s *senderStubStore) UpdateWebhook(context.Context, db.UpdateWebhookParams) error {
	return nil
}
func (s *senderStubStore) DeleteWebhook(context.Context, string) error { return nil }
func (s *senderStubStore) ListWebhookDeliveries(context.Context, string, int, int) ([]*db.WebhookDelivery, error) {
	return nil, nil
}
func (s *senderStubStore) Ping(context.Context) error { return nil }

func (s *senderStubStore) UpsertAnalysisResult(context.Context, db.UpsertAnalysisResultParams) (*db.AnalysisResult, error) {
	return nil, nil
}
func (s *senderStubStore) GetAnalysisResult(context.Context, string, string) (*db.AnalysisResult, error) {
	return nil, nil
}
func (s *senderStubStore) ListAnalysisResults(context.Context, string) ([]*db.AnalysisResult, error) {
	return nil, nil
}

func (s *senderStubStore) CreateSession(context.Context, db.CreateSessionParams) (*db.Session, error) {
	return nil, nil
}
func (s *senderStubStore) GetSessionByToken(context.Context, string) (*db.Session, error) {
	return nil, nil
}
func (s *senderStubStore) DeleteSession(context.Context, string) error  { return nil }
func (s *senderStubStore) PruneExpiredSessions(context.Context) error   { return nil }

func (s *senderStubStore) CreateEnrollmentToken(context.Context, db.CreateEnrollmentTokenParams) (*db.EnrollmentToken, error) {
	return nil, nil
}
func (s *senderStubStore) GetEnrollmentToken(context.Context, string) (*db.EnrollmentToken, error) {
	return nil, nil
}
func (s *senderStubStore) ConsumeEnrollmentToken(context.Context, db.ConsumeEnrollmentTokenParams) error {
	return nil
}
func (s *senderStubStore) ListEnrollmentTokens(context.Context) ([]*db.EnrollmentToken, error) {
	return nil, nil
}
func (s *senderStubStore) DeleteEnrollmentToken(context.Context, string) error      { return nil }
func (s *senderStubStore) PruneExpiredEnrollmentTokens(context.Context) error       { return nil }

func (s *senderStubStore) RetryFailedTasksForJob(context.Context, string) error { return nil }
func (s *senderStubStore) ListJobLogs(context.Context, db.ListJobLogsParams) ([]*db.TaskLog, error) {
	return nil, nil
}
func (s *senderStubStore) PruneOldTaskLogs(context.Context, time.Time) error { return nil }
func (s *senderStubStore) CreatePathMapping(context.Context, db.CreatePathMappingParams) (*db.PathMapping, error) {
	return nil, nil
}
func (s *senderStubStore) GetPathMappingByID(context.Context, string) (*db.PathMapping, error) {
	return nil, nil
}
func (s *senderStubStore) ListPathMappings(context.Context) ([]*db.PathMapping, error) { return nil, nil }
func (s *senderStubStore) UpdatePathMapping(context.Context, db.UpdatePathMappingParams) (*db.PathMapping, error) {
	return nil, nil
}
func (s *senderStubStore) DeletePathMapping(context.Context, string) error                         { return nil }
func (s *senderStubStore) DeleteTasksByJobID(_ context.Context, _ string) error                    { return nil }
func (s *senderStubStore) CreateAuditEntry(_ context.Context, _ db.CreateAuditEntryParams) error   { return nil }
func (s *senderStubStore) InsertAgentMetric(_ context.Context, _ db.InsertAgentMetricParams) error { return nil }
func (s *senderStubStore) ListAgentMetrics(_ context.Context, _ string, _ time.Time) ([]*db.AgentMetric, error) { return nil, nil }
func (s *senderStubStore) ListAuditLog(_ context.Context, _, _ int) ([]*db.AuditEntry, int, error) { return nil, 0, nil }

// ---------------------------------------------------------------------------
// TestSenderSend_success — delivers on first attempt to a real HTTP server
// ---------------------------------------------------------------------------

func TestSenderSend_success(t *testing.T) {
	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store := &senderStubStore{}
	snd := &sender{
		store:  store,
		cfg:    Config{MaxRetries: 3, DeliveryTimeout: 5 * time.Second},
		logger: discardLogger(),
	}

	secret := "test-secret"
	wh := &db.Webhook{ID: "wh-1", URL: srv.URL, Provider: "generic", Secret: &secret}
	event := Event{Type: "job.completed", Payload: map[string]any{"job_id": "j-1"}}

	snd.send(context.Background(), wh, event)

	if received.Load() != 1 {
		t.Errorf("server received %d requests, want 1", received.Load())
	}
	if len(store.deliveries) != 1 {
		t.Fatalf("logged %d deliveries, want 1", len(store.deliveries))
	}
	d := store.deliveries[0]
	if !d.Success {
		t.Error("delivery logged as failed, want success")
	}
	if d.Attempt != 1 {
		t.Errorf("attempt = %d, want 1", d.Attempt)
	}
	if d.Event != "job.completed" {
		t.Errorf("event = %q, want job.completed", d.Event)
	}
}

// ---------------------------------------------------------------------------
// TestSenderSend_HMAC — X-Signature header is set when secret is configured
// ---------------------------------------------------------------------------

func TestSenderSend_HMAC(t *testing.T) {
	var sigHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sigHeader = r.Header.Get("X-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store := &senderStubStore{}
	snd := &sender{store: store, cfg: Config{MaxRetries: 1, DeliveryTimeout: 5 * time.Second}, logger: discardLogger()}

	secret := "hmac-key"
	wh := &db.Webhook{ID: "wh-2", URL: srv.URL, Provider: "generic", Secret: &secret}
	event := Event{Type: "agent.online", Payload: map[string]any{"agent_id": "a-1"}}

	snd.send(context.Background(), wh, event)

	if sigHeader == "" {
		t.Fatal("X-Signature header not set")
	}
	if len(sigHeader) < 7 || sigHeader[:7] != "sha256=" {
		t.Errorf("X-Signature format wrong: %q", sigHeader)
	}
}

// ---------------------------------------------------------------------------
// TestSenderSend_noSecret — no X-Signature when secret is nil
// ---------------------------------------------------------------------------

func TestSenderSend_noSecret(t *testing.T) {
	var sigHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sigHeader = r.Header.Get("X-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store := &senderStubStore{}
	snd := &sender{store: store, cfg: Config{MaxRetries: 1, DeliveryTimeout: 5 * time.Second}, logger: discardLogger()}

	wh := &db.Webhook{ID: "wh-3", URL: srv.URL, Provider: "generic", Secret: nil}
	event := Event{Type: "job.failed", Payload: map[string]any{"job_id": "j-2"}}

	snd.send(context.Background(), wh, event)

	if sigHeader != "" {
		t.Errorf("expected no X-Signature, got %q", sigHeader)
	}
}

// ---------------------------------------------------------------------------
// TestSenderSend_serverError — 500 response logs failure, does NOT retry in
// the test (retry delays are skipped by cancelling context)
// ---------------------------------------------------------------------------

func TestSenderSend_serverError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	store := &senderStubStore{}
	snd := &sender{store: store, cfg: Config{MaxRetries: 1, DeliveryTimeout: 5 * time.Second}, logger: discardLogger()}

	// Override retry delays to zero so the test doesn't sleep.
	orig := retryDelays
	retryDelays = []time.Duration{0, 0, 0}
	t.Cleanup(func() { retryDelays = orig })

	wh := &db.Webhook{ID: "wh-4", URL: srv.URL, Provider: "generic"}
	event := Event{Type: "job.failed", Payload: map[string]any{"job_id": "j-3"}}

	snd.send(context.Background(), wh, event)

	// MaxRetries=1 → 2 attempts total
	if len(store.deliveries) != 2 {
		t.Fatalf("logged %d deliveries, want 2", len(store.deliveries))
	}
	for _, d := range store.deliveries {
		if d.Success {
			t.Error("delivery logged as success, want failure")
		}
	}
}

// ---------------------------------------------------------------------------
// TestSenderSend_unreachableURL — network error is recorded as failure
// ---------------------------------------------------------------------------

func TestSenderSend_unreachableURL(t *testing.T) {
	// Zero out retry delays so the test completes quickly.
	orig := retryDelays
	retryDelays = []time.Duration{0, 0, 0}
	t.Cleanup(func() { retryDelays = orig })

	store := &senderStubStore{}
	// MaxRetries: 1 → 2 total attempts.
	snd := &sender{store: store, cfg: Config{MaxRetries: 1, DeliveryTimeout: 100 * time.Millisecond}, logger: discardLogger()}

	wh := &db.Webhook{ID: "wh-5", URL: "http://127.0.0.1:1", Provider: "generic"}
	event := Event{Type: "job.completed", Payload: map[string]any{}}

	snd.send(context.Background(), wh, event)

	if len(store.deliveries) != 2 {
		t.Fatalf("logged %d deliveries, want 2", len(store.deliveries))
	}
	for _, d := range store.deliveries {
		if d.Success {
			t.Error("expected failure delivery, got success")
		}
	}
}

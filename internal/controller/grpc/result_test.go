package grpc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/badskater/distributed-encoder/internal/controller/config"
	"github.com/badskater/distributed-encoder/internal/controller/webhooks"
	"github.com/badskater/distributed-encoder/internal/db"
	pb "github.com/badskater/distributed-encoder/internal/proto/encoderv1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ---------------------------------------------------------------------------
// Stub store
// ---------------------------------------------------------------------------

// resultStub is a minimal db.Store that records which task operations were
// called and controls the job returned by GetJobByID.
type resultStub struct {
	// Injected responses
	job    *db.Job
	jobErr error
	// Recorded calls
	completedID string
	failedID    string
	failCode    int
	failMsg     string
	updatedJobStatus string
}

func (s *resultStub) CompleteTask(_ context.Context, p db.CompleteTaskParams) error {
	s.completedID = p.ID
	return nil
}
func (s *resultStub) FailTask(_ context.Context, id string, code int, msg string) error {
	s.failedID = id
	s.failCode = code
	s.failMsg = msg
	return nil
}
func (s *resultStub) UpdateJobTaskCounts(context.Context, string) error { return nil }
func (s *resultStub) GetJobByID(_ context.Context, _ string) (*db.Job, error) {
	return s.job, s.jobErr
}
func (s *resultStub) UpdateJobStatus(_ context.Context, _ string, newStatus string) error {
	s.updatedJobStatus = newStatus
	return nil
}

// The remaining Store methods are no-ops; they satisfy the interface.
func (s *resultStub) CreateUser(context.Context, db.CreateUserParams) (*db.User, error) {
	return nil, nil
}
func (s *resultStub) GetUserByUsername(context.Context, string) (*db.User, error)  { return nil, nil }
func (s *resultStub) GetUserByOIDCSub(context.Context, string) (*db.User, error)   { return nil, nil }
func (s *resultStub) GetUserByID(context.Context, string) (*db.User, error)        { return nil, nil }
func (s *resultStub) ListUsers(context.Context) ([]*db.User, error)                { return nil, nil }
func (s *resultStub) UpdateUserRole(context.Context, string, string) error         { return nil }
func (s *resultStub) DeleteUser(context.Context, string) error                     { return nil }
func (s *resultStub) CountAdminUsers(context.Context) (int64, error)               { return 1, nil }
func (s *resultStub) UpsertAgent(context.Context, db.UpsertAgentParams) (*db.Agent, error) {
	return nil, nil
}
func (s *resultStub) GetAgentByID(context.Context, string) (*db.Agent, error)   { return nil, nil }
func (s *resultStub) GetAgentByName(context.Context, string) (*db.Agent, error) { return nil, nil }
func (s *resultStub) ListAgents(context.Context) ([]*db.Agent, error)           { return nil, nil }
func (s *resultStub) UpdateAgentStatus(context.Context, string, string) error   { return nil }
func (s *resultStub) UpdateAgentHeartbeat(context.Context, db.UpdateAgentHeartbeatParams) error {
	return nil
}
func (s *resultStub) SetAgentAPIKey(context.Context, string, string) error { return nil }
func (s *resultStub) MarkStaleAgents(context.Context, time.Duration) (int64, error) {
	return 0, nil
}
func (s *resultStub) CreateSource(context.Context, db.CreateSourceParams) (*db.Source, error) {
	return nil, nil
}
func (s *resultStub) GetSourceByID(context.Context, string) (*db.Source, error)      { return nil, nil }
func (s *resultStub) GetSourceByUNCPath(context.Context, string) (*db.Source, error) { return nil, nil }
func (s *resultStub) ListSources(context.Context, db.ListSourcesFilter) ([]*db.Source, int64, error) {
	return nil, 0, nil
}
func (s *resultStub) UpdateSourceState(context.Context, string, string) error  { return nil }
func (s *resultStub) UpdateSourceVMAF(context.Context, string, float64) error  { return nil }
func (s *resultStub) DeleteSource(context.Context, string) error               { return nil }
func (s *resultStub) CreateJob(context.Context, db.CreateJobParams) (*db.Job, error) {
	return nil, nil
}
func (s *resultStub) ListJobs(context.Context, db.ListJobsFilter) ([]*db.Job, int64, error) {
	return nil, 0, nil
}
func (s *resultStub) GetJobsNeedingExpansion(context.Context) ([]*db.Job, error) { return nil, nil }
func (s *resultStub) CreateTask(context.Context, db.CreateTaskParams) (*db.Task, error) {
	return nil, nil
}
func (s *resultStub) GetTaskByID(context.Context, string) (*db.Task, error) { return nil, nil }
func (s *resultStub) ListTasksByJob(context.Context, string) ([]*db.Task, error) {
	return nil, nil
}
func (s *resultStub) ClaimNextTask(context.Context, string, []string) (*db.Task, error) {
	return nil, nil
}
func (s *resultStub) UpdateTaskStatus(context.Context, string, string) error { return nil }
func (s *resultStub) SetTaskScriptDir(context.Context, string, string) error { return nil }
func (s *resultStub) CancelPendingTasksForJob(context.Context, string) error { return nil }
func (s *resultStub) InsertTaskLog(context.Context, db.InsertTaskLogParams) error { return nil }
func (s *resultStub) ListTaskLogs(context.Context, db.ListTaskLogsParams) ([]*db.TaskLog, error) {
	return nil, nil
}
func (s *resultStub) TailTaskLogs(context.Context, string, int64) ([]*db.TaskLog, error) {
	return nil, nil
}
func (s *resultStub) CreateTemplate(context.Context, db.CreateTemplateParams) (*db.Template, error) {
	return nil, nil
}
func (s *resultStub) GetTemplateByID(context.Context, string) (*db.Template, error) {
	return nil, nil
}
func (s *resultStub) ListTemplates(context.Context, string) ([]*db.Template, error) {
	return nil, nil
}
func (s *resultStub) UpdateTemplate(context.Context, db.UpdateTemplateParams) error { return nil }
func (s *resultStub) DeleteTemplate(context.Context, string) error                  { return nil }
func (s *resultStub) UpsertVariable(context.Context, db.UpsertVariableParams) (*db.Variable, error) {
	return nil, nil
}
func (s *resultStub) GetVariableByName(context.Context, string) (*db.Variable, error) {
	return nil, nil
}
func (s *resultStub) ListVariables(context.Context, string) ([]*db.Variable, error) {
	return nil, nil
}
func (s *resultStub) DeleteVariable(context.Context, string) error { return nil }
func (s *resultStub) CreateWebhook(context.Context, db.CreateWebhookParams) (*db.Webhook, error) {
	return nil, nil
}
func (s *resultStub) GetWebhookByID(context.Context, string) (*db.Webhook, error) {
	return nil, nil
}
func (s *resultStub) ListWebhooksByEvent(context.Context, string) ([]*db.Webhook, error) {
	return nil, nil
}
func (s *resultStub) ListWebhooks(context.Context) ([]*db.Webhook, error) { return nil, nil }
func (s *resultStub) UpdateWebhook(context.Context, db.UpdateWebhookParams) error {
	return nil
}
func (s *resultStub) DeleteWebhook(context.Context, string) error                      { return nil }
func (s *resultStub) InsertWebhookDelivery(context.Context, db.InsertWebhookDeliveryParams) error {
	return nil
}
func (s *resultStub) ListWebhookDeliveries(context.Context, string, int, int) ([]*db.WebhookDelivery, error) {
	return nil, nil
}
func (s *resultStub) UpsertAnalysisResult(context.Context, db.UpsertAnalysisResultParams) (*db.AnalysisResult, error) {
	return nil, nil
}
func (s *resultStub) GetAnalysisResult(context.Context, string, string) (*db.AnalysisResult, error) {
	return nil, nil
}
func (s *resultStub) ListAnalysisResults(context.Context, string) ([]*db.AnalysisResult, error) {
	return nil, nil
}
func (s *resultStub) CreateSession(context.Context, db.CreateSessionParams) (*db.Session, error) {
	return nil, nil
}
func (s *resultStub) GetSessionByToken(context.Context, string) (*db.Session, error) {
	return nil, nil
}
func (s *resultStub) DeleteSession(context.Context, string) error  { return nil }
func (s *resultStub) PruneExpiredSessions(context.Context) error   { return nil }
func (s *resultStub) CreateEnrollmentToken(context.Context, db.CreateEnrollmentTokenParams) (*db.EnrollmentToken, error) {
	return nil, nil
}
func (s *resultStub) GetEnrollmentToken(context.Context, string) (*db.EnrollmentToken, error) {
	return nil, nil
}
func (s *resultStub) ConsumeEnrollmentToken(context.Context, db.ConsumeEnrollmentTokenParams) error {
	return nil
}
func (s *resultStub) ListEnrollmentTokens(context.Context) ([]*db.EnrollmentToken, error) {
	return nil, nil
}
func (s *resultStub) DeleteEnrollmentToken(context.Context, string) error      { return nil }
func (s *resultStub) PruneExpiredEnrollmentTokens(context.Context) error       { return nil }
func (s *resultStub) RetryFailedTasksForJob(context.Context, string) error     { return nil }
func (s *resultStub) ListJobLogs(context.Context, db.ListJobLogsParams) ([]*db.TaskLog, error) {
	return nil, nil
}
func (s *resultStub) PruneOldTaskLogs(context.Context, time.Time) error { return nil }
func (s *resultStub) Ping(context.Context) error                        { return nil }

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newResultServer(store *resultStub) *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wh := webhooks.New(store, webhooks.Config{}, logger)
	return &Server{
		store:    store,
		cfg:      &config.GRPCConfig{},
		agentCfg: &config.AgentConfig{TaskTimeoutSec: 3600},
		logger:   logger,
		webhooks: wh,
	}
}

// allDoneJob returns a job where all tasks are complete (no pending/running).
func allDoneJob(id string, failed int) *db.Job {
	return &db.Job{
		ID:             id,
		Status:         "running",
		TasksTotal:     3,
		TasksCompleted: 3 - failed,
		TasksFailed:    failed,
		TasksPending:   0,
		TasksRunning:   0,
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestReportResult_MissingFields(t *testing.T) {
	srv := newResultServer(&resultStub{})
	ctx := context.Background()

	cases := []struct {
		name string
		req  *pb.TaskResult
	}{
		{"empty", &pb.TaskResult{}},
		{"missing job_id", &pb.TaskResult{TaskId: "t1"}},
		{"missing task_id", &pb.TaskResult{JobId: "j1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := srv.ReportResult(ctx, tc.req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if st, ok := status.FromError(err); !ok || st.Code() != codes.InvalidArgument {
				t.Errorf("expected InvalidArgument, got %v", err)
			}
		})
	}
}

func TestReportResult_SuccessPath(t *testing.T) {
	stub := &resultStub{
		job: allDoneJob("job-1", 0), // all completed, no failures
	}
	srv := newResultServer(stub)
	ctx := context.Background()

	req := &pb.TaskResult{
		TaskId:  "task-1",
		JobId:   "job-1",
		Success: true,
		Metrics: &pb.EncodeMetrics{
			FramesEncoded: 1000,
			AvgFps:        24.5,
			OutputSize:    1024 * 1024,
		},
	}

	ack, err := srv.ReportResult(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ack.Ok {
		t.Error("expected ack.ok = true")
	}
	if stub.completedID != "task-1" {
		t.Errorf("CompleteTask not called with task-1, got %q", stub.completedID)
	}
	if stub.updatedJobStatus != "completed" {
		t.Errorf("expected job status 'completed', got %q", stub.updatedJobStatus)
	}
}

func TestReportResult_FailurePath(t *testing.T) {
	stub := &resultStub{
		job: allDoneJob("job-2", 1), // one failure
	}
	srv := newResultServer(stub)
	ctx := context.Background()

	req := &pb.TaskResult{
		TaskId:   "task-2",
		JobId:    "job-2",
		Success:  false,
		ExitCode: 1,
		ErrorMsg: "encoder crashed",
	}

	ack, err := srv.ReportResult(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ack.Ok {
		t.Error("expected ack.ok = true")
	}
	if stub.failedID != "task-2" {
		t.Errorf("FailTask not called with task-2, got %q", stub.failedID)
	}
	if stub.failCode != 1 {
		t.Errorf("expected exit code 1, got %d", stub.failCode)
	}
	if stub.failMsg != "encoder crashed" {
		t.Errorf("expected error msg 'encoder crashed', got %q", stub.failMsg)
	}
	if stub.updatedJobStatus != "failed" {
		t.Errorf("expected job status 'failed', got %q", stub.updatedJobStatus)
	}
}

func TestReportResult_PendingTasksRemain(t *testing.T) {
	// Job still has pending tasks — should NOT transition to terminal state.
	stub := &resultStub{
		job: &db.Job{
			ID:           "job-3",
			Status:       "running",
			TasksTotal:   5,
			TasksPending: 2, // more work to do
			TasksRunning: 0,
		},
	}
	srv := newResultServer(stub)
	ctx := context.Background()

	req := &pb.TaskResult{
		TaskId:  "task-3",
		JobId:   "job-3",
		Success: true,
	}

	if _, err := srv.ReportResult(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.updatedJobStatus != "" {
		t.Errorf("job status should not change while tasks remain, got %q", stub.updatedJobStatus)
	}
}

func TestReportResult_GetJobError(t *testing.T) {
	stub := &resultStub{
		jobErr: errors.New("db unavailable"),
	}
	srv := newResultServer(stub)
	ctx := context.Background()

	req := &pb.TaskResult{TaskId: "t", JobId: "j", Success: true}

	_, err := srv.ReportResult(ctx, req)
	if err == nil {
		t.Fatal("expected error when GetJobByID fails")
	}
	if st, ok := status.FromError(err); !ok || st.Code() != codes.Internal {
		t.Errorf("expected Internal error, got %v", err)
	}
}

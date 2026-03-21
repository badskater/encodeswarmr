// Package db provides the database layer for the controller.
//
// Layout:
//   - db.go       — Store interface + pgx pool wiring
//   - queries.go  — All SQL query implementations
//   - migrate.go  — golang-migrate runner (embedded SQL files)
package db

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store is the single interface the rest of the application uses for database
// access.  It is intentionally broad so callers never import pgx directly.
type Store interface {
	// --- Users ---
	CreateUser(ctx context.Context, p CreateUserParams) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	GetUserByOIDCSub(ctx context.Context, sub string) (*User, error)
	GetUserByID(ctx context.Context, id string) (*User, error)
	ListUsers(ctx context.Context) ([]*User, error)
	UpdateUserRole(ctx context.Context, id, role string) error
	DeleteUser(ctx context.Context, id string) error
	CountAdminUsers(ctx context.Context) (int64, error)

	// --- Agents ---
	UpsertAgent(ctx context.Context, p UpsertAgentParams) (*Agent, error)
	GetAgentByID(ctx context.Context, id string) (*Agent, error)
	GetAgentByName(ctx context.Context, name string) (*Agent, error)
	ListAgents(ctx context.Context) ([]*Agent, error)
	UpdateAgentStatus(ctx context.Context, id, status string) error
	UpdateAgentHeartbeat(ctx context.Context, p UpdateAgentHeartbeatParams) error
	UpdateAgentVNCPort(ctx context.Context, id string, port int) error
	SetAgentAPIKey(ctx context.Context, id, hash string) error
	MarkStaleAgents(ctx context.Context, olderThan time.Duration) (int64, error)

	// --- Sources ---
	CreateSource(ctx context.Context, p CreateSourceParams) (*Source, error)
	GetSourceByID(ctx context.Context, id string) (*Source, error)
	GetSourceByUNCPath(ctx context.Context, uncPath string) (*Source, error)
	ListSources(ctx context.Context, filter ListSourcesFilter) ([]*Source, int64, error)
	UpdateSourceState(ctx context.Context, id, state string) error
	UpdateSourceVMAF(ctx context.Context, id string, score float64) error
	UpdateSourceHDR(ctx context.Context, p UpdateSourceHDRParams) error
	DeleteSource(ctx context.Context, id string) error

	// --- Jobs ---
	CreateJob(ctx context.Context, p CreateJobParams) (*Job, error)
	GetJobByID(ctx context.Context, id string) (*Job, error)
	ListJobs(ctx context.Context, filter ListJobsFilter) ([]*Job, int64, error)
	UpdateJobStatus(ctx context.Context, id, status string) error
	UpdateJobTaskCounts(ctx context.Context, id string) error
	GetJobsNeedingExpansion(ctx context.Context) ([]*Job, error)

	// --- Tasks ---
	CreateTask(ctx context.Context, p CreateTaskParams) (*Task, error)
	GetTaskByID(ctx context.Context, id string) (*Task, error)
	ListTasksByJob(ctx context.Context, jobID string) ([]*Task, error)
	ClaimNextTask(ctx context.Context, agentID string, tags []string) (*Task, error)
	UpdateTaskStatus(ctx context.Context, id, status string) error
	SetTaskScriptDir(ctx context.Context, id, scriptDir string) error
	CompleteTask(ctx context.Context, p CompleteTaskParams) error
	FailTask(ctx context.Context, id string, exitCode int, errMsg string) error
	CancelPendingTasksForJob(ctx context.Context, jobID string) error
	DeleteTasksByJobID(ctx context.Context, jobID string) error

	// --- Task Logs ---
	InsertTaskLog(ctx context.Context, p InsertTaskLogParams) error
	ListTaskLogs(ctx context.Context, p ListTaskLogsParams) ([]*TaskLog, error)
	TailTaskLogs(ctx context.Context, taskID string, afterID int64) ([]*TaskLog, error)

	// --- Templates ---
	CreateTemplate(ctx context.Context, p CreateTemplateParams) (*Template, error)
	GetTemplateByID(ctx context.Context, id string) (*Template, error)
	ListTemplates(ctx context.Context, templateType string) ([]*Template, error)
	UpdateTemplate(ctx context.Context, p UpdateTemplateParams) error
	DeleteTemplate(ctx context.Context, id string) error

	// --- Variables ---
	UpsertVariable(ctx context.Context, p UpsertVariableParams) (*Variable, error)
	GetVariableByName(ctx context.Context, name string) (*Variable, error)
	ListVariables(ctx context.Context, category string) ([]*Variable, error)
	DeleteVariable(ctx context.Context, id string) error

	// --- Webhooks ---
	CreateWebhook(ctx context.Context, p CreateWebhookParams) (*Webhook, error)
	GetWebhookByID(ctx context.Context, id string) (*Webhook, error)
	ListWebhooksByEvent(ctx context.Context, event string) ([]*Webhook, error)
	ListWebhooks(ctx context.Context) ([]*Webhook, error)
	UpdateWebhook(ctx context.Context, p UpdateWebhookParams) error
	DeleteWebhook(ctx context.Context, id string) error
	InsertWebhookDelivery(ctx context.Context, p InsertWebhookDeliveryParams) error
	ListWebhookDeliveries(ctx context.Context, webhookID string, limit, offset int) ([]*WebhookDelivery, error)

	// --- Analysis ---
	UpsertAnalysisResult(ctx context.Context, p UpsertAnalysisResultParams) (*AnalysisResult, error)
	GetAnalysisResult(ctx context.Context, sourceID, analysisType string) (*AnalysisResult, error)
	ListAnalysisResults(ctx context.Context, sourceID string) ([]*AnalysisResult, error)

	// --- Path Mappings ---
	CreatePathMapping(ctx context.Context, p CreatePathMappingParams) (*PathMapping, error)
	GetPathMappingByID(ctx context.Context, id string) (*PathMapping, error)
	ListPathMappings(ctx context.Context) ([]*PathMapping, error)
	UpdatePathMapping(ctx context.Context, p UpdatePathMappingParams) (*PathMapping, error)
	DeletePathMapping(ctx context.Context, id string) error

	// --- Sessions ---
	CreateSession(ctx context.Context, p CreateSessionParams) (*Session, error)
	GetSessionByToken(ctx context.Context, token string) (*Session, error)
	DeleteSession(ctx context.Context, token string) error
	PruneExpiredSessions(ctx context.Context) error

	// --- Enrollment Tokens ---
	CreateEnrollmentToken(ctx context.Context, p CreateEnrollmentTokenParams) (*EnrollmentToken, error)
	GetEnrollmentToken(ctx context.Context, token string) (*EnrollmentToken, error)
	ConsumeEnrollmentToken(ctx context.Context, p ConsumeEnrollmentTokenParams) error
	ListEnrollmentTokens(ctx context.Context) ([]*EnrollmentToken, error)
	DeleteEnrollmentToken(ctx context.Context, id string) error
	PruneExpiredEnrollmentTokens(ctx context.Context) error

	// --- Extended queries ---
	RetryFailedTasksForJob(ctx context.Context, jobID string) error
	ListJobLogs(ctx context.Context, p ListJobLogsParams) ([]*TaskLog, error)
	PruneOldTaskLogs(ctx context.Context, olderThan time.Time) error

	// --- Audit Log ---
	CreateAuditEntry(ctx context.Context, params CreateAuditEntryParams) error
	ListAuditLog(ctx context.Context, limit, offset int) ([]*AuditEntry, int, error)

	// --- Agent Metrics ---
	InsertAgentMetric(ctx context.Context, p InsertAgentMetricParams) error
	ListAgentMetrics(ctx context.Context, agentID string, since time.Time) ([]*AgentMetric, error)

	// --- Schedules ---
	CreateSchedule(ctx context.Context, p CreateScheduleParams) (*Schedule, error)
	GetScheduleByID(ctx context.Context, id string) (*Schedule, error)
	ListSchedules(ctx context.Context) ([]*Schedule, error)
	UpdateSchedule(ctx context.Context, p UpdateScheduleParams) (*Schedule, error)
	DeleteSchedule(ctx context.Context, id string) error
	ListDueSchedules(ctx context.Context) ([]*Schedule, error)
	MarkScheduleRun(ctx context.Context, p MarkScheduleRunParams) error

	// Ping verifies the database connection is alive.
	Ping(ctx context.Context) error
}

// pgStore implements Store using a pgx connection pool.
type pgStore struct {
	pool poolIface
}

// New opens a pgx connection pool and returns a Store.
// The caller is responsible for closing the pool via Close().
func New(ctx context.Context, dsn string) (Store, *pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("db: parse config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("db: open pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("db: ping: %w", err)
	}

	return &pgStore{pool: pool}, pool, nil
}

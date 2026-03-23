// Package teststore provides a no-op implementation of db.Store for use in
// unit tests.
//
// # Usage
//
// Embed teststore.Stub in your test stub struct.  Only override the methods
// relevant to the test; everything else returns zero values.
//
//	type myStub struct {
//	    teststore.Stub
//	    job *db.Job
//	}
//
//	func (s *myStub) GetJobByID(_ context.Context, _ string) (*db.Job, error) {
//	    return s.job, nil
//	}
//
// When new methods are added to db.Store, add them here once and all test
// stubs automatically satisfy the interface without any other changes.
package teststore

import (
	"context"
	"time"

	"github.com/badskater/encodeswarmr/internal/db"
)

// Stub is a zero-value implementation of db.Store.  Every method returns the
// zero value for its result types and a nil error.  Embed this in test stubs
// and override only the methods that matter for a given test.
type Stub struct{}

// Verify Stub satisfies db.Store at compile time.
var _ db.Store = (*Stub)(nil)

// --- Users ---
func (Stub) CreateUser(_ context.Context, _ db.CreateUserParams) (*db.User, error)       { return nil, nil }
func (Stub) GetUserByUsername(_ context.Context, _ string) (*db.User, error)             { return nil, nil }
func (Stub) GetUserByOIDCSub(_ context.Context, _ string) (*db.User, error)              { return nil, nil }
func (Stub) GetUserByID(_ context.Context, _ string) (*db.User, error)                   { return nil, nil }
func (Stub) ListUsers(_ context.Context) ([]*db.User, error)                             { return nil, nil }
func (Stub) UpdateUserRole(_ context.Context, _, _ string) error                         { return nil }
func (Stub) DeleteUser(_ context.Context, _ string) error                                { return nil }
func (Stub) CountAdminUsers(_ context.Context) (int64, error)                            { return 0, nil }

// --- Agents ---
func (Stub) UpsertAgent(_ context.Context, _ db.UpsertAgentParams) (*db.Agent, error)              { return nil, nil }
func (Stub) GetAgentByID(_ context.Context, _ string) (*db.Agent, error)                           { return nil, nil }
func (Stub) GetAgentByName(_ context.Context, _ string) (*db.Agent, error)                         { return nil, nil }
func (Stub) ListAgents(_ context.Context) ([]*db.Agent, error)                                     { return nil, nil }
func (Stub) UpdateAgentStatus(_ context.Context, _, _ string) error                                { return nil }
func (Stub) UpdateAgentHeartbeat(_ context.Context, _ db.UpdateAgentHeartbeatParams) error         { return nil }
func (Stub) UpdateAgentVNCPort(_ context.Context, _ string, _ int) error                           { return nil }
func (Stub) SetAgentAPIKey(_ context.Context, _, _ string) error                                   { return nil }
func (Stub) MarkStaleAgents(_ context.Context, _ time.Duration) (int64, error)                     { return 0, nil }
func (Stub) SetAgentUpgradeRequested(_ context.Context, _ string, _ bool) error                    { return nil }
func (Stub) ClearAgentUpgradeRequested(_ context.Context, _ string) error                          { return nil }
func (Stub) UpdateAgentTags(_ context.Context, _ db.UpdateAgentTagsParams) error                   { return nil }

// --- Agent Pools ---
func (Stub) CreateAgentPool(_ context.Context, _ db.CreateAgentPoolParams) (*db.AgentPool, error) { return nil, nil }
func (Stub) GetAgentPoolByID(_ context.Context, _ string) (*db.AgentPool, error)                  { return nil, nil }
func (Stub) ListAgentPools(_ context.Context) ([]*db.AgentPool, error)                            { return nil, nil }
func (Stub) UpdateAgentPool(_ context.Context, _ db.UpdateAgentPoolParams) (*db.AgentPool, error) { return nil, nil }
func (Stub) DeleteAgentPool(_ context.Context, _ string) error                                    { return nil }

// --- Sources ---
func (Stub) CreateSource(_ context.Context, _ db.CreateSourceParams) (*db.Source, error)            { return nil, nil }
func (Stub) GetSourceByID(_ context.Context, _ string) (*db.Source, error)                          { return nil, nil }
func (Stub) GetSourceByUNCPath(_ context.Context, _ string) (*db.Source, error)                     { return nil, nil }
func (Stub) ListSources(_ context.Context, _ db.ListSourcesFilter) ([]*db.Source, int64, error)     { return nil, 0, nil }
func (Stub) UpdateSourceState(_ context.Context, _, _ string) error                                         { return nil }
func (Stub) UpdateSourceVMAF(_ context.Context, _ string, _ float64) error                                  { return nil }
func (Stub) UpdateSourceHDR(_ context.Context, _ db.UpdateSourceHDRParams) error                            { return nil }
func (Stub) UpdateSourceThumbnails(_ context.Context, _ db.UpdateSourceThumbnailsParams) error              { return nil }
func (Stub) DeleteSource(_ context.Context, _ string) error                                                  { return nil }

// --- Jobs ---
func (Stub) CreateJob(_ context.Context, _ db.CreateJobParams) (*db.Job, error)                  { return nil, nil }
func (Stub) GetJobByID(_ context.Context, _ string) (*db.Job, error)                             { return nil, nil }
func (Stub) ListJobs(_ context.Context, _ db.ListJobsFilter) ([]*db.Job, int64, error)           { return nil, 0, nil }
func (Stub) UpdateJobStatus(_ context.Context, _, _ string) error                                { return nil }
func (Stub) UpdateJobPriority(_ context.Context, _ db.UpdateJobPriorityParams) error             { return nil }
func (Stub) UpdateJobTaskCounts(_ context.Context, _ string) error                               { return nil }
func (Stub) GetJobsNeedingExpansion(_ context.Context) ([]*db.Job, error)                        { return nil, nil }
func (Stub) UnblockDependentJobs(_ context.Context, _ string) error                              { return nil }
func (Stub) ListJobsByChainGroup(_ context.Context, _ string) ([]*db.Job, error)                 { return nil, nil }
func (Stub) ListPendingJobs(_ context.Context) ([]*db.Job, error)                                { return nil, nil }

// --- Tasks ---
func (Stub) CreateTask(_ context.Context, _ db.CreateTaskParams) (*db.Task, error)       { return nil, nil }
func (Stub) GetTaskByID(_ context.Context, _ string) (*db.Task, error)                   { return nil, nil }
func (Stub) ListTasksByJob(_ context.Context, _ string) ([]*db.Task, error)              { return nil, nil }
func (Stub) ClaimNextTask(_ context.Context, _ string, _ []string) (*db.Task, error)     { return nil, nil }
func (Stub) ClaimConcatTask(_ context.Context, _ string) error                           { return nil }
func (Stub) UpdateTaskStatus(_ context.Context, _, _ string) error                       { return nil }
func (Stub) SetTaskScriptDir(_ context.Context, _, _ string) error                       { return nil }
func (Stub) CompleteTask(_ context.Context, _ db.CompleteTaskParams) error               { return nil }
func (Stub) FailTask(_ context.Context, _ string, _ int, _ string) error                 { return nil }
func (Stub) CancelPendingTasksForJob(_ context.Context, _ string) error                  { return nil }
func (Stub) DeleteTasksByJobID(_ context.Context, _ string) error                        { return nil }
func (Stub) RetryTaskWithBackoff(_ context.Context, _ string, _ int) (*db.Task, error)              { return nil, nil }
func (Stub) RetryTaskWithBackoffJitter(_ context.Context, _ string, _ int, _ int) (*db.Task, error) { return nil, nil }

// --- Task Logs ---
func (Stub) InsertTaskLog(_ context.Context, _ db.InsertTaskLogParams) error                     { return nil }
func (Stub) ListTaskLogs(_ context.Context, _ db.ListTaskLogsParams) ([]*db.TaskLog, error)      { return nil, nil }
func (Stub) TailTaskLogs(_ context.Context, _ string, _ int64) ([]*db.TaskLog, error)            { return nil, nil }

// --- Templates ---
func (Stub) CreateTemplate(_ context.Context, _ db.CreateTemplateParams) (*db.Template, error) { return nil, nil }
func (Stub) GetTemplateByID(_ context.Context, _ string) (*db.Template, error)                 { return nil, nil }
func (Stub) ListTemplates(_ context.Context, _ string) ([]*db.Template, error)                 { return nil, nil }
func (Stub) UpdateTemplate(_ context.Context, _ db.UpdateTemplateParams) error                 { return nil }
func (Stub) DeleteTemplate(_ context.Context, _ string) error                                  { return nil }

// --- Template Versions ---
func (Stub) CreateTemplateVersion(_ context.Context, _ db.CreateTemplateVersionParams) (*db.TemplateVersion, error) { return nil, nil }
func (Stub) ListTemplateVersions(_ context.Context, _ string) ([]*db.TemplateVersion, error)                        { return nil, nil }
func (Stub) GetTemplateVersion(_ context.Context, _ string, _ int) (*db.TemplateVersion, error)                     { return nil, nil }
func (Stub) GetLatestTemplateVersion(_ context.Context, _ string) (int, error)                                      { return 0, nil }

// --- Variables ---
func (Stub) UpsertVariable(_ context.Context, _ db.UpsertVariableParams) (*db.Variable, error) { return nil, nil }
func (Stub) GetVariableByName(_ context.Context, _ string) (*db.Variable, error)               { return nil, nil }
func (Stub) ListVariables(_ context.Context, _ string) ([]*db.Variable, error)                 { return nil, nil }
func (Stub) DeleteVariable(_ context.Context, _ string) error                                  { return nil }

// --- Webhooks ---
func (Stub) CreateWebhook(_ context.Context, _ db.CreateWebhookParams) (*db.Webhook, error)          { return nil, nil }
func (Stub) GetWebhookByID(_ context.Context, _ string) (*db.Webhook, error)                         { return nil, nil }
func (Stub) ListWebhooksByEvent(_ context.Context, _ string) ([]*db.Webhook, error)                  { return nil, nil }
func (Stub) ListWebhooks(_ context.Context) ([]*db.Webhook, error)                                   { return nil, nil }
func (Stub) UpdateWebhook(_ context.Context, _ db.UpdateWebhookParams) error                         { return nil }
func (Stub) DeleteWebhook(_ context.Context, _ string) error                                         { return nil }
func (Stub) InsertWebhookDelivery(_ context.Context, _ db.InsertWebhookDeliveryParams) error         { return nil }
func (Stub) ListWebhookDeliveries(_ context.Context, _ string, _, _ int) ([]*db.WebhookDelivery, error) { return nil, nil }

// --- Analysis ---
func (Stub) UpsertAnalysisResult(_ context.Context, _ db.UpsertAnalysisResultParams) (*db.AnalysisResult, error) { return nil, nil }
func (Stub) GetAnalysisResult(_ context.Context, _, _ string) (*db.AnalysisResult, error)                        { return nil, nil }
func (Stub) ListAnalysisResults(_ context.Context, _ string) ([]*db.AnalysisResult, error)                       { return nil, nil }

// --- Path Mappings ---
func (Stub) CreatePathMapping(_ context.Context, _ db.CreatePathMappingParams) (*db.PathMapping, error) { return nil, nil }
func (Stub) GetPathMappingByID(_ context.Context, _ string) (*db.PathMapping, error)                    { return nil, nil }
func (Stub) ListPathMappings(_ context.Context) ([]*db.PathMapping, error)                              { return nil, nil }
func (Stub) UpdatePathMapping(_ context.Context, _ db.UpdatePathMappingParams) (*db.PathMapping, error) { return nil, nil }
func (Stub) DeletePathMapping(_ context.Context, _ string) error                                        { return nil }

// --- Sessions ---
func (Stub) CreateSession(_ context.Context, _ db.CreateSessionParams) (*db.Session, error) { return nil, nil }
func (Stub) GetSessionByToken(_ context.Context, _ string) (*db.Session, error)             { return nil, nil }
func (Stub) DeleteSession(_ context.Context, _ string) error                                { return nil }
func (Stub) PruneExpiredSessions(_ context.Context) error                                   { return nil }

// --- Enrollment Tokens ---
func (Stub) CreateEnrollmentToken(_ context.Context, _ db.CreateEnrollmentTokenParams) (*db.EnrollmentToken, error) { return nil, nil }
func (Stub) GetEnrollmentToken(_ context.Context, _ string) (*db.EnrollmentToken, error)                            { return nil, nil }
func (Stub) ConsumeEnrollmentToken(_ context.Context, _ db.ConsumeEnrollmentTokenParams) error                      { return nil }
func (Stub) ListEnrollmentTokens(_ context.Context) ([]*db.EnrollmentToken, error)                                  { return nil, nil }
func (Stub) DeleteEnrollmentToken(_ context.Context, _ string) error                                                { return nil }
func (Stub) PruneExpiredEnrollmentTokens(_ context.Context) error                                                   { return nil }

// --- Extended queries ---
func (Stub) RetryFailedTasksForJob(_ context.Context, _ string) error                         { return nil }
func (Stub) ListJobLogs(_ context.Context, _ db.ListJobLogsParams) ([]*db.TaskLog, error)     { return nil, nil }
func (Stub) PruneOldTaskLogs(_ context.Context, _ time.Time) error                            { return nil }

// --- Audit Log ---
func (Stub) CreateAuditEntry(_ context.Context, _ db.CreateAuditEntryParams) error               { return nil }
func (Stub) ListAuditLog(_ context.Context, _, _ int) ([]*db.AuditEntry, int, error)             { return nil, 0, nil }

// --- Agent Metrics ---
func (Stub) InsertAgentMetric(_ context.Context, _ db.InsertAgentMetricParams) error                    { return nil }
func (Stub) ListAgentMetrics(_ context.Context, _ string, _ time.Time) ([]*db.AgentMetric, error)       { return nil, nil }

// --- API Keys ---
func (Stub) CreateAPIKey(_ context.Context, _ db.CreateAPIKeyParams) (*db.APIKey, error)  { return nil, nil }
func (Stub) GetAPIKeyByHash(_ context.Context, _ string) (*db.APIKey, error)              { return nil, nil }
func (Stub) ListAPIKeysByUser(_ context.Context, _ string) ([]*db.APIKey, error)          { return nil, nil }
func (Stub) DeleteAPIKey(_ context.Context, _ string) error                               { return nil }
func (Stub) UpdateAPIKeyLastUsed(_ context.Context, _ string) error                       { return nil }

// --- Notification Preferences ---
func (Stub) GetNotificationPrefs(_ context.Context, _ string) (*db.NotificationPrefs, error)             { return nil, nil }
func (Stub) UpsertNotificationPrefs(_ context.Context, _ db.UpsertNotificationPrefsParams) error         { return nil }
func (Stub) ListUsersWithEmailNotifications(_ context.Context) ([]*db.NotificationPrefs, error)           { return nil, nil }

// --- Schedules ---
func (Stub) CreateSchedule(_ context.Context, _ db.CreateScheduleParams) (*db.Schedule, error)    { return nil, nil }
func (Stub) GetScheduleByID(_ context.Context, _ string) (*db.Schedule, error)                    { return nil, nil }
func (Stub) ListSchedules(_ context.Context) ([]*db.Schedule, error)                              { return nil, nil }
func (Stub) UpdateSchedule(_ context.Context, _ db.UpdateScheduleParams) (*db.Schedule, error)    { return nil, nil }
func (Stub) DeleteSchedule(_ context.Context, _ string) error                                     { return nil }
func (Stub) ListDueSchedules(_ context.Context) ([]*db.Schedule, error)                           { return nil, nil }
func (Stub) MarkScheduleRun(_ context.Context, _ db.MarkScheduleRunParams) error                  { return nil }

// --- Task Preemption ---
func (Stub) PreemptTask(_ context.Context, _ string) error { return nil }

// --- Estimation ---
func (Stub) GetAvgFPSStats(_ context.Context, _ string) (float64, int64, error) { return 0, 0, nil }

// --- Encoding Stats ---
func (Stub) UpsertEncodingStats(_ context.Context, _ db.UpsertEncodingStatsParams) error              { return nil }
func (Stub) GetEncodingStats(_ context.Context, _, _, _ string) (*db.EncodingStats, error)            { return nil, nil }

// --- Agent FPS ---
func (Stub) GetAgentAvgFPS(_ context.Context, _ string) (float64, error) { return 0, nil }

// --- Dashboard metrics ---
func (Stub) GetThroughputStats(_ context.Context, _ int) ([]*db.ThroughputPoint, error) { return nil, nil }
func (Stub) GetQueueStats(_ context.Context) (*db.QueueStats, error)                    { return nil, nil }
func (Stub) GetRecentActivity(_ context.Context, _ int) ([]*db.ActivityEvent, error)    { return nil, nil }

// --- Flows ---
func (Stub) CreateFlow(_ context.Context, _ db.CreateFlowParams) (*db.Flow, error) { return nil, nil }
func (Stub) GetFlowByID(_ context.Context, _ string) (*db.Flow, error)             { return nil, nil }
func (Stub) ListFlows(_ context.Context) ([]*db.Flow, error)                       { return nil, nil }
func (Stub) UpdateFlow(_ context.Context, _ db.UpdateFlowParams) (*db.Flow, error) { return nil, nil }
func (Stub) DeleteFlow(_ context.Context, _ string) error                          { return nil }

// --- Job Archive ---
func (Stub) ArchiveOldJobs(_ context.Context, _ time.Duration) (int64, error)                        { return 0, nil }
func (Stub) ListArchivedJobs(_ context.Context, _ db.ListJobsFilter) ([]*db.Job, int64, error)       { return nil, 0, nil }
func (Stub) ExportJobs(_ context.Context, _ db.ExportJobsFilter) ([]*db.Job, error)                  { return nil, nil }
func (Stub) ExportArchivedJobs(_ context.Context, _ db.ExportJobsFilter) ([]*db.Job, error)          { return nil, nil }

// --- Encoding Rules ---
func (Stub) CreateEncodingRule(_ context.Context, _ db.CreateEncodingRuleParams) (*db.EncodingRule, error) { return nil, nil }
func (Stub) GetEncodingRuleByID(_ context.Context, _ string) (*db.EncodingRule, error)                     { return nil, nil }
func (Stub) ListEncodingRules(_ context.Context) ([]*db.EncodingRule, error)                               { return nil, nil }
func (Stub) UpdateEncodingRule(_ context.Context, _ db.UpdateEncodingRuleParams) (*db.EncodingRule, error) { return nil, nil }
func (Stub) DeleteEncodingRule(_ context.Context, _ string) error                                          { return nil }

// --- Sources (watch folder extensions) ---
func (Stub) UpdateSourceWatch(_ context.Context, _ db.UpdateSourceWatchParams) error { return nil }

// --- Misc ---
func (Stub) Ping(_ context.Context) error { return nil }

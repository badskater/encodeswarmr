package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/badskater/distributed-encoder/internal/controller/config"
	"github.com/badskater/distributed-encoder/internal/db"
)

// ---------------------------------------------------------------------------
// Minimal db.Store stub — only the methods auth.Service actually calls.
// ---------------------------------------------------------------------------

type authStubStore struct {
	session *db.Session
	sessErr error
	user    *db.User
	userErr error
}

func (s *authStubStore) GetSessionByToken(_ context.Context, _ string) (*db.Session, error) {
	return s.session, s.sessErr
}
func (s *authStubStore) GetUserByID(_ context.Context, _ string) (*db.User, error) {
	return s.user, s.userErr
}

// All remaining Store methods — zero-value stubs so the type satisfies the interface.
func (s *authStubStore) CreateUser(context.Context, db.CreateUserParams) (*db.User, error) {
	return nil, nil
}
func (s *authStubStore) GetUserByUsername(context.Context, string) (*db.User, error) {
	return nil, nil
}
func (s *authStubStore) GetUserByOIDCSub(context.Context, string) (*db.User, error) {
	return nil, nil
}
func (s *authStubStore) ListUsers(context.Context) ([]*db.User, error)       { return nil, nil }
func (s *authStubStore) UpdateUserRole(context.Context, string, string) error { return nil }
func (s *authStubStore) DeleteUser(context.Context, string) error             { return nil }
func (s *authStubStore) CountAdminUsers(context.Context) (int64, error)       { return 1, nil }

func (s *authStubStore) UpsertAgent(context.Context, db.UpsertAgentParams) (*db.Agent, error) {
	return nil, nil
}
func (s *authStubStore) GetAgentByID(context.Context, string) (*db.Agent, error)   { return nil, nil }
func (s *authStubStore) GetAgentByName(context.Context, string) (*db.Agent, error) { return nil, nil }
func (s *authStubStore) ListAgents(context.Context) ([]*db.Agent, error)           { return nil, nil }
func (s *authStubStore) UpdateAgentStatus(context.Context, string, string) error   { return nil }
func (s *authStubStore) UpdateAgentHeartbeat(context.Context, db.UpdateAgentHeartbeatParams) error {
	return nil
}
func (s *authStubStore) UpdateAgentVNCPort(context.Context, string, int) error { return nil }
func (s *authStubStore) SetAgentAPIKey(context.Context, string, string) error  { return nil }
func (s *authStubStore) MarkStaleAgents(context.Context, time.Duration) (int64, error) {
	return 0, nil
}

func (s *authStubStore) CreateSource(context.Context, db.CreateSourceParams) (*db.Source, error) {
	return nil, nil
}
func (s *authStubStore) GetSourceByID(context.Context, string) (*db.Source, error)      { return nil, nil }
func (s *authStubStore) GetSourceByUNCPath(context.Context, string) (*db.Source, error) { return nil, nil }
func (s *authStubStore) ListSources(context.Context, db.ListSourcesFilter) ([]*db.Source, int64, error) {
	return nil, 0, nil
}
func (s *authStubStore) UpdateSourceState(context.Context, string, string) error         { return nil }
func (s *authStubStore) UpdateSourceVMAF(context.Context, string, float64) error         { return nil }
func (s *authStubStore) UpdateSourceHDR(context.Context, db.UpdateSourceHDRParams) error { return nil }
func (s *authStubStore) DeleteSource(context.Context, string) error                      { return nil }

func (s *authStubStore) CreateJob(context.Context, db.CreateJobParams) (*db.Job, error) {
	return nil, nil
}
func (s *authStubStore) GetJobByID(context.Context, string) (*db.Job, error) { return nil, nil }
func (s *authStubStore) ListJobs(context.Context, db.ListJobsFilter) ([]*db.Job, int64, error) {
	return nil, 0, nil
}
func (s *authStubStore) UpdateJobStatus(context.Context, string, string) error { return nil }
func (s *authStubStore) UpdateJobTaskCounts(context.Context, string) error     { return nil }
func (s *authStubStore) GetJobsNeedingExpansion(context.Context) ([]*db.Job, error) {
	return nil, nil
}

func (s *authStubStore) CreateTask(context.Context, db.CreateTaskParams) (*db.Task, error) {
	return nil, nil
}
func (s *authStubStore) GetTaskByID(context.Context, string) (*db.Task, error) { return nil, nil }
func (s *authStubStore) ListTasksByJob(context.Context, string) ([]*db.Task, error) {
	return nil, nil
}
func (s *authStubStore) ClaimNextTask(context.Context, string, []string) (*db.Task, error) {
	return nil, nil
}
func (s *authStubStore) UpdateTaskStatus(context.Context, string, string) error { return nil }
func (s *authStubStore) SetTaskScriptDir(context.Context, string, string) error { return nil }
func (s *authStubStore) CompleteTask(context.Context, db.CompleteTaskParams) error {
	return nil
}
func (s *authStubStore) FailTask(context.Context, string, int, string) error    { return nil }
func (s *authStubStore) CancelPendingTasksForJob(context.Context, string) error { return nil }
func (s *authStubStore) DeleteTasksByJobID(context.Context, string) error       { return nil }

func (s *authStubStore) InsertTaskLog(context.Context, db.InsertTaskLogParams) error { return nil }
func (s *authStubStore) ListTaskLogs(context.Context, db.ListTaskLogsParams) ([]*db.TaskLog, error) {
	return nil, nil
}
func (s *authStubStore) TailTaskLogs(context.Context, string, int64) ([]*db.TaskLog, error) {
	return nil, nil
}

func (s *authStubStore) CreateTemplate(context.Context, db.CreateTemplateParams) (*db.Template, error) {
	return nil, nil
}
func (s *authStubStore) GetTemplateByID(context.Context, string) (*db.Template, error) {
	return nil, nil
}
func (s *authStubStore) ListTemplates(context.Context, string) ([]*db.Template, error) {
	return nil, nil
}
func (s *authStubStore) UpdateTemplate(context.Context, db.UpdateTemplateParams) error { return nil }
func (s *authStubStore) DeleteTemplate(context.Context, string) error                  { return nil }

func (s *authStubStore) UpsertVariable(context.Context, db.UpsertVariableParams) (*db.Variable, error) {
	return nil, nil
}
func (s *authStubStore) GetVariableByName(context.Context, string) (*db.Variable, error) {
	return nil, nil
}
func (s *authStubStore) ListVariables(context.Context, string) ([]*db.Variable, error) {
	return nil, nil
}
func (s *authStubStore) DeleteVariable(context.Context, string) error { return nil }

func (s *authStubStore) CreateWebhook(context.Context, db.CreateWebhookParams) (*db.Webhook, error) {
	return nil, nil
}
func (s *authStubStore) GetWebhookByID(context.Context, string) (*db.Webhook, error) {
	return nil, nil
}
func (s *authStubStore) ListWebhooksByEvent(context.Context, string) ([]*db.Webhook, error) {
	return nil, nil
}
func (s *authStubStore) ListWebhooks(context.Context) ([]*db.Webhook, error) { return nil, nil }
func (s *authStubStore) UpdateWebhook(context.Context, db.UpdateWebhookParams) error {
	return nil
}
func (s *authStubStore) DeleteWebhook(context.Context, string) error { return nil }
func (s *authStubStore) InsertWebhookDelivery(context.Context, db.InsertWebhookDeliveryParams) error {
	return nil
}
func (s *authStubStore) ListWebhookDeliveries(context.Context, string, int, int) ([]*db.WebhookDelivery, error) {
	return nil, nil
}
func (s *authStubStore) Ping(context.Context) error { return nil }

func (s *authStubStore) UpsertAnalysisResult(context.Context, db.UpsertAnalysisResultParams) (*db.AnalysisResult, error) {
	return nil, nil
}
func (s *authStubStore) GetAnalysisResult(context.Context, string, string) (*db.AnalysisResult, error) {
	return nil, nil
}
func (s *authStubStore) ListAnalysisResults(context.Context, string) ([]*db.AnalysisResult, error) {
	return nil, nil
}

func (s *authStubStore) CreateSession(context.Context, db.CreateSessionParams) (*db.Session, error) {
	return nil, nil
}
func (s *authStubStore) DeleteSession(context.Context, string) error  { return nil }
func (s *authStubStore) PruneExpiredSessions(context.Context) error   { return nil }

func (s *authStubStore) CreateEnrollmentToken(context.Context, db.CreateEnrollmentTokenParams) (*db.EnrollmentToken, error) {
	return nil, nil
}
func (s *authStubStore) GetEnrollmentToken(context.Context, string) (*db.EnrollmentToken, error) {
	return nil, nil
}
func (s *authStubStore) ConsumeEnrollmentToken(context.Context, db.ConsumeEnrollmentTokenParams) error {
	return nil
}
func (s *authStubStore) ListEnrollmentTokens(context.Context) ([]*db.EnrollmentToken, error) {
	return nil, nil
}
func (s *authStubStore) DeleteEnrollmentToken(context.Context, string) error      { return nil }
func (s *authStubStore) PruneExpiredEnrollmentTokens(context.Context) error       { return nil }

func (s *authStubStore) RetryFailedTasksForJob(context.Context, string) error { return nil }
func (s *authStubStore) ListJobLogs(context.Context, db.ListJobLogsParams) ([]*db.TaskLog, error) {
	return nil, nil
}
func (s *authStubStore) PruneOldTaskLogs(context.Context, time.Time) error { return nil }
func (s *authStubStore) CreatePathMapping(context.Context, db.CreatePathMappingParams) (*db.PathMapping, error) {
	return nil, nil
}
func (s *authStubStore) GetPathMappingByID(context.Context, string) (*db.PathMapping, error) {
	return nil, nil
}
func (s *authStubStore) ListPathMappings(context.Context) ([]*db.PathMapping, error) { return nil, nil }
func (s *authStubStore) UpdatePathMapping(context.Context, db.UpdatePathMappingParams) (*db.PathMapping, error) {
	return nil, nil
}
func (s *authStubStore) DeletePathMapping(context.Context, string) error                        { return nil }
func (s *authStubStore) CreateAuditEntry(_ context.Context, _ db.CreateAuditEntryParams) error          { return nil }
func (s *authStubStore) InsertAgentMetric(_ context.Context, _ db.InsertAgentMetricParams) error        { return nil }
func (s *authStubStore) ListAgentMetrics(_ context.Context, _ string, _ time.Time) ([]*db.AgentMetric, error) { return nil, nil }
func (s *authStubStore) ListAuditLog(_ context.Context, _, _ int) ([]*db.AuditEntry, int, error)              { return nil, 0, nil }
func (s *authStubStore) CreateSchedule(_ context.Context, _ db.CreateScheduleParams) (*db.Schedule, error)    { return nil, nil }
func (s *authStubStore) GetScheduleByID(_ context.Context, _ string) (*db.Schedule, error)                    { return nil, nil }
func (s *authStubStore) ListSchedules(_ context.Context) ([]*db.Schedule, error)                              { return nil, nil }
func (s *authStubStore) UpdateSchedule(_ context.Context, _ db.UpdateScheduleParams) (*db.Schedule, error)    { return nil, nil }
func (s *authStubStore) DeleteSchedule(_ context.Context, _ string) error                                     { return nil }
func (s *authStubStore) ListDueSchedules(_ context.Context) ([]*db.Schedule, error)                           { return nil, nil }
func (s *authStubStore) MarkScheduleRun(_ context.Context, _ db.MarkScheduleRunParams) error                  { return nil }

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestService(store *authStubStore) *Service {
	return &Service{
		store: store,
		cfg:   &config.AuthConfig{SessionTTL: time.Hour},
	}
}

func okHandler(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

// ---------------------------------------------------------------------------
// TestMiddleware
// ---------------------------------------------------------------------------

func TestMiddleware(t *testing.T) {
	t.Run("no token returns 401", func(t *testing.T) {
		svc := newTestService(&authStubStore{})
		h := svc.Middleware(http.HandlerFunc(okHandler))

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", rr.Code)
		}
	})

	t.Run("unknown session token returns 401", func(t *testing.T) {
		store := &authStubStore{sessErr: db.ErrNotFound}
		svc := newTestService(store)
		h := svc.Middleware(http.HandlerFunc(okHandler))

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "bad-token"})
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", rr.Code)
		}
	})

	t.Run("valid cookie populates context claims", func(t *testing.T) {
		store := &authStubStore{
			session: &db.Session{Token: "tok", UserID: "u1"},
			user:    &db.User{ID: "u1", Username: "alice", Role: "admin"},
		}
		svc := newTestService(store)

		var gotClaims *Claims
		h := svc.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotClaims, _ = FromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		}))

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "tok"})
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("got %d, want 200", rr.Code)
		}
		if gotClaims == nil {
			t.Fatal("claims not set in context")
		}
		if gotClaims.Username != "alice" {
			t.Errorf("username = %q, want alice", gotClaims.Username)
		}
		if gotClaims.Role != "admin" {
			t.Errorf("role = %q, want admin", gotClaims.Role)
		}
	})

	t.Run("valid Bearer header populates context claims", func(t *testing.T) {
		store := &authStubStore{
			session: &db.Session{Token: "bearer-tok", UserID: "u2"},
			user:    &db.User{ID: "u2", Username: "bob", Role: "operator"},
		}
		svc := newTestService(store)

		var gotClaims *Claims
		h := svc.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotClaims, _ = FromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		}))

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer bearer-tok")
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("got %d, want 200", rr.Code)
		}
		if gotClaims == nil || gotClaims.Username != "bob" {
			t.Errorf("unexpected claims: %+v", gotClaims)
		}
	})
}

// ---------------------------------------------------------------------------
// TestRequireRole
// ---------------------------------------------------------------------------

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name     string
		role     string
		minRole  string
		wantCode int
	}{
		{"admin satisfies admin", "admin", "admin", http.StatusOK},
		{"admin satisfies operator", "admin", "operator", http.StatusOK},
		{"admin satisfies viewer", "admin", "viewer", http.StatusOK},
		{"operator satisfies operator", "operator", "operator", http.StatusOK},
		{"operator satisfies viewer", "operator", "viewer", http.StatusOK},
		{"operator denied admin", "operator", "admin", http.StatusForbidden},
		{"viewer satisfies viewer", "viewer", "viewer", http.StatusOK},
		{"viewer denied operator", "viewer", "operator", http.StatusForbidden},
		{"viewer denied admin", "viewer", "admin", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := RequireRole(tt.minRole, http.HandlerFunc(okHandler))

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			// Inject claims directly into context.
			req = req.WithContext(withClaims(req.Context(), &Claims{
				UserID:   "u1",
				Username: "user",
				Role:     tt.role,
			}))
			h.ServeHTTP(rr, req)

			if rr.Code != tt.wantCode {
				t.Errorf("got %d, want %d", rr.Code, tt.wantCode)
			}
		})
	}

	t.Run("no claims in context returns 401", func(t *testing.T) {
		h := RequireRole("viewer", http.HandlerFunc(okHandler))

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", rr.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// TestHasRole
// ---------------------------------------------------------------------------

func TestHasRole(t *testing.T) {
	tests := []struct {
		role    string
		minRole string
		want    bool
	}{
		{"admin", "admin", true},
		{"admin", "operator", true},
		{"admin", "viewer", true},
		{"operator", "admin", false},
		{"operator", "operator", true},
		{"operator", "viewer", true},
		{"viewer", "admin", false},
		{"viewer", "operator", false},
		{"viewer", "viewer", true},
		{"unknown", "viewer", false},
	}
	for _, tt := range tests {
		if got := hasRole(tt.role, tt.minRole); got != tt.want {
			t.Errorf("hasRole(%q, %q) = %v, want %v", tt.role, tt.minRole, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// TestExtractToken
// ---------------------------------------------------------------------------

func TestExtractToken(t *testing.T) {
	svc := newTestService(&authStubStore{})

	t.Run("returns empty string when no auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if tok := svc.extractToken(req); tok != "" {
			t.Errorf("expected empty, got %q", tok)
		}
	})

	t.Run("reads session cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "cookie-token"})
		if tok := svc.extractToken(req); tok != "cookie-token" {
			t.Errorf("got %q, want cookie-token", tok)
		}
	})

	t.Run("reads Bearer header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer hdr-token")
		if tok := svc.extractToken(req); tok != "hdr-token" {
			t.Errorf("got %q, want hdr-token", tok)
		}
	})

	t.Run("cookie takes precedence over Bearer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "cookie-wins"})
		req.Header.Set("Authorization", "Bearer bearer-token")
		if tok := svc.extractToken(req); tok != "cookie-wins" {
			t.Errorf("got %q, want cookie-wins", tok)
		}
	})
}

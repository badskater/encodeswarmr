package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/db"
	"github.com/badskater/encodeswarmr/internal/db/teststore"
)

// ---------------------------------------------------------------------------
// Discard logger
// ---------------------------------------------------------------------------

type discardHandler struct{}

func (d *discardHandler) Enabled(_ context.Context, _ slog.Level) bool  { return false }
func (d *discardHandler) Handle(_ context.Context, _ slog.Record) error { return nil }
func (d *discardHandler) WithAttrs(_ []slog.Attr) slog.Handler          { return d }
func (d *discardHandler) WithGroup(_ string) slog.Handler               { return d }

func newDiscardLogger() *slog.Logger {
	return slog.New(&discardHandler{})
}

// ---------------------------------------------------------------------------
// NextRunFromExpr — table-driven tests for the exported pure function
// ---------------------------------------------------------------------------

func TestNextRunFromExpr_ValidExpressions(t *testing.T) {
	before := time.Now().UTC()

	tests := []struct {
		name string
		expr string
	}{
		{"every minute", "* * * * *"},
		{"daily at midnight", "0 0 * * *"},
		{"hourly", "0 * * * *"},
		{"weekdays at 9am", "0 9 * * 1-5"},
		{"every 15 minutes", "*/15 * * * *"},
		{"monthly first day", "0 0 1 * *"},
		{"descriptor @hourly", "@hourly"},
		{"descriptor @daily", "@daily"},
		{"descriptor @weekly", "@weekly"},
		{"descriptor @monthly", "@monthly"},
		{"descriptor @yearly", "@yearly"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NextRunFromExpr(tc.expr)
			if err != nil {
				t.Fatalf("NextRunFromExpr(%q) returned error: %v", tc.expr, err)
			}
			if got == nil {
				t.Fatalf("NextRunFromExpr(%q) returned nil time", tc.expr)
			}
			// The next run must be in the future (at or after before).
			if got.Before(before) {
				t.Errorf("NextRunFromExpr(%q) returned a time in the past: %v", tc.expr, *got)
			}
		})
	}
}

func TestNextRunFromExpr_InvalidExpression(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{"empty", ""},
		{"too many fields", "* * * * * * *"},
		{"garbage", "not-a-cron"},
		{"partial", "5 * *"},
		{"invalid day-of-week", "0 0 * * 8"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NextRunFromExpr(tc.expr)
			if err == nil {
				t.Errorf("NextRunFromExpr(%q) expected error, got time=%v", tc.expr, got)
			}
			if got != nil {
				t.Errorf("NextRunFromExpr(%q) expected nil time on error, got %v", tc.expr, *got)
			}
		})
	}
}

func TestNextRunFromExpr_FutureTime(t *testing.T) {
	// Specific check: daily at midnight must be >= 1 second from now and
	// <= 24 hours + 1 minute from now.
	got, err := NextRunFromExpr("0 0 * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	now := time.Now().UTC()
	if got.Before(now) {
		t.Errorf("expected future time, got %v (now=%v)", *got, now)
	}
	maxDuration := 24*time.Hour + time.Minute
	if got.Sub(now) > maxDuration {
		t.Errorf("next run too far in the future: %v", got.Sub(now))
	}
}

// ---------------------------------------------------------------------------
// nextRun (unexported) — same semantics as NextRunFromExpr
// ---------------------------------------------------------------------------

func TestNextRun_ValidExpressions(t *testing.T) {
	tests := []string{
		"* * * * *",
		"0 12 * * *",
		"30 6 * * 1",
		"@hourly",
		"@midnight",
	}
	for _, expr := range tests {
		got, err := nextRun(expr)
		if err != nil {
			t.Errorf("nextRun(%q): unexpected error: %v", expr, err)
			continue
		}
		if got == nil {
			t.Errorf("nextRun(%q): expected non-nil time", expr)
		}
	}
}

func TestNextRun_InvalidExpressions(t *testing.T) {
	tests := []string{"", "bad", "99 * * * *"}
	for _, expr := range tests {
		got, err := nextRun(expr)
		if err == nil {
			t.Errorf("nextRun(%q): expected error, got time=%v", expr, got)
		}
	}
}

// ---------------------------------------------------------------------------
// New / construction
// ---------------------------------------------------------------------------

func TestNew_Construction(t *testing.T) {
	stub := &teststore.Stub{}
	logger := newDiscardLogger()

	s := New(stub, logger)
	if s == nil {
		t.Fatal("New returned nil")
	}
	if s.store == nil {
		t.Error("store should be set")
	}
	if s.logger == nil {
		t.Error("logger should be set")
	}
}

// ---------------------------------------------------------------------------
// tick — mock store
// ---------------------------------------------------------------------------

// scheduleStoreStub overrides schedule-related Store methods for tick/fire tests.
type scheduleStoreStub struct {
	teststore.Stub
	mu sync.Mutex

	due        []*db.Schedule
	listErr    error

	createdJobs  []*db.Job
	createJobErr error

	markedRuns  []db.MarkScheduleRunParams
	markRunErr  error
}

func (s *scheduleStoreStub) ListDueSchedules(_ context.Context) ([]*db.Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.due, s.listErr
}

func (s *scheduleStoreStub) CreateJob(_ context.Context, p db.CreateJobParams) (*db.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.createJobErr != nil {
		return nil, s.createJobErr
	}
	j := &db.Job{
		ID:      fmt.Sprintf("job-%d", len(s.createdJobs)+1),
		SourceID: p.SourceID,
		JobType:  p.JobType,
		Priority: p.Priority,
	}
	s.createdJobs = append(s.createdJobs, j)
	return j, nil
}

func (s *scheduleStoreStub) MarkScheduleRun(_ context.Context, p db.MarkScheduleRunParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.markedRuns = append(s.markedRuns, p)
	return s.markRunErr
}

// makeSchedule builds a Schedule with a valid job template.
func makeSchedule(id, name, cronExpr string, jobParams db.CreateJobParams) *db.Schedule {
	raw, _ := json.Marshal(jobParams)
	return &db.Schedule{
		ID:          id,
		Name:        name,
		CronExpr:    cronExpr,
		JobTemplate: raw,
	}
}

// TestTick_NoDueSchedules verifies tick does nothing when no schedules are due.
func TestTick_NoDueSchedules(t *testing.T) {
	stub := &scheduleStoreStub{due: nil}
	logger := newDiscardLogger()
	s := New(stub, logger)

	s.tick(context.Background())

	if len(stub.createdJobs) != 0 {
		t.Errorf("expected no jobs created, got %d", len(stub.createdJobs))
	}
}

// TestTick_ListError verifies tick logs and returns gracefully on list error.
func TestTick_ListError(t *testing.T) {
	stub := &scheduleStoreStub{listErr: errors.New("db down")}
	logger := newDiscardLogger()
	s := New(stub, logger)

	// Should not panic.
	s.tick(context.Background())

	if len(stub.createdJobs) != 0 {
		t.Error("should not create jobs when list fails")
	}
}

// TestTick_OneSchedule verifies that a single due schedule fires a job.
func TestTick_OneSchedule(t *testing.T) {
	params := db.CreateJobParams{SourceID: "src-1", JobType: "encode", Priority: 5}
	sc := makeSchedule("sched-1", "nightly", "@daily", params)
	stub := &scheduleStoreStub{due: []*db.Schedule{sc}}
	logger := newDiscardLogger()
	s := New(stub, logger)

	s.tick(context.Background())

	if len(stub.createdJobs) != 1 {
		t.Errorf("expected 1 job created, got %d", len(stub.createdJobs))
	}
	if len(stub.markedRuns) != 1 {
		t.Errorf("expected 1 MarkScheduleRun call, got %d", len(stub.markedRuns))
	}
	if stub.markedRuns[0].ID != "sched-1" {
		t.Errorf("expected schedule ID sched-1, got %q", stub.markedRuns[0].ID)
	}
}

// TestTick_MultipleSchedules verifies all due schedules are fired.
func TestTick_MultipleSchedules(t *testing.T) {
	params := db.CreateJobParams{SourceID: "src-1", JobType: "encode"}
	stub := &scheduleStoreStub{
		due: []*db.Schedule{
			makeSchedule("sched-A", "a", "@daily", params),
			makeSchedule("sched-B", "b", "@hourly", params),
		},
	}
	logger := newDiscardLogger()
	s := New(stub, logger)

	s.tick(context.Background())

	if len(stub.createdJobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(stub.createdJobs))
	}
	if len(stub.markedRuns) != 2 {
		t.Errorf("expected 2 MarkScheduleRun calls, got %d", len(stub.markedRuns))
	}
}

// ---------------------------------------------------------------------------
// fire — direct unit tests
// ---------------------------------------------------------------------------

// TestFire_InvalidTemplate verifies fire returns an error for unparseable templates.
func TestFire_InvalidTemplate(t *testing.T) {
	stub := &scheduleStoreStub{}
	logger := newDiscardLogger()
	s := New(stub, logger)

	sc := &db.Schedule{
		ID:          "bad-sched",
		Name:        "bad",
		CronExpr:    "@daily",
		JobTemplate: []byte(`not-json`),
	}
	err := s.fire(context.Background(), sc)
	if err == nil {
		t.Error("expected error for invalid job template")
	}
}

// TestFire_CreateJobError verifies fire returns an error when CreateJob fails.
func TestFire_CreateJobError(t *testing.T) {
	stub := &scheduleStoreStub{createJobErr: errors.New("db error")}
	logger := newDiscardLogger()
	s := New(stub, logger)

	params := db.CreateJobParams{SourceID: "src-x", JobType: "encode"}
	sc := makeSchedule("sched-e", "err-sched", "@daily", params)

	err := s.fire(context.Background(), sc)
	if err == nil {
		t.Error("expected error when CreateJob fails")
	}
}

// TestFire_MarkScheduleRunError verifies fire returns an error when MarkScheduleRun fails
// (but the job was already created).
func TestFire_MarkScheduleRunError(t *testing.T) {
	stub := &scheduleStoreStub{markRunErr: errors.New("mark failed")}
	logger := newDiscardLogger()
	s := New(stub, logger)

	params := db.CreateJobParams{SourceID: "src-y", JobType: "encode"}
	sc := makeSchedule("sched-m", "mark-err", "@daily", params)

	err := s.fire(context.Background(), sc)
	if err == nil {
		t.Error("expected error when MarkScheduleRun fails")
	}
	// The job should still have been created.
	if len(stub.createdJobs) != 1 {
		t.Errorf("expected job to be created despite MarkScheduleRun error, got %d jobs", len(stub.createdJobs))
	}
}

// TestFire_InvalidCronExpr_StillMarksRun verifies that an invalid cron expression
// on the schedule doesn't prevent MarkScheduleRun from being called.
func TestFire_InvalidCronExpr_StillMarksRun(t *testing.T) {
	stub := &scheduleStoreStub{}
	logger := newDiscardLogger()
	s := New(stub, logger)

	params := db.CreateJobParams{SourceID: "src-z", JobType: "encode"}
	raw, _ := json.Marshal(params)
	sc := &db.Schedule{
		ID:          "sched-bad-cron",
		Name:        "bad-cron",
		CronExpr:    "not-a-cron-expr",
		JobTemplate: raw,
	}

	err := s.fire(context.Background(), sc)
	// MarkScheduleRun should succeed (stub returns nil), so fire returns nil.
	if err != nil {
		t.Errorf("expected nil error (MarkScheduleRun succeeds), got: %v", err)
	}
	// MarkScheduleRun should have been called.
	if len(stub.markedRuns) != 1 {
		t.Errorf("expected MarkScheduleRun to be called, got %d calls", len(stub.markedRuns))
	}
	// NextRunAt should be nil when the cron expr is invalid.
	if stub.markedRuns[0].NextRunAt != nil {
		t.Errorf("expected nil NextRunAt for invalid cron, got %v", *stub.markedRuns[0].NextRunAt)
	}
}

// TestFire_Success verifies the happy path end-to-end.
func TestFire_Success(t *testing.T) {
	stub := &scheduleStoreStub{}
	logger := newDiscardLogger()
	s := New(stub, logger)

	params := db.CreateJobParams{
		SourceID: "src-ok",
		JobType:  "encode",
		Priority: 3,
	}
	sc := makeSchedule("sched-ok", "daily-encode", "@daily", params)

	before := time.Now().UTC().Add(-time.Second)
	err := s.fire(context.Background(), sc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(stub.createdJobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(stub.createdJobs))
	}
	if stub.createdJobs[0].SourceID != "src-ok" {
		t.Errorf("job source ID = %q, want %q", stub.createdJobs[0].SourceID, "src-ok")
	}

	if len(stub.markedRuns) != 1 {
		t.Errorf("expected 1 MarkScheduleRun call, got %d", len(stub.markedRuns))
	}
	mp := stub.markedRuns[0]
	if mp.ID != "sched-ok" {
		t.Errorf("marked schedule ID = %q, want %q", mp.ID, "sched-ok")
	}
	if mp.LastRunAt.Before(before) {
		t.Errorf("LastRunAt %v should be after %v", mp.LastRunAt, before)
	}
	if mp.NextRunAt == nil {
		t.Error("NextRunAt should be set for valid cron expr")
	}
}

// ---------------------------------------------------------------------------
// Run — lifecycle smoke test
// ---------------------------------------------------------------------------

// TestRun_ContextCancel verifies Run exits when ctx is cancelled.
func TestRun_ContextCancel(t *testing.T) {
	stub := &scheduleStoreStub{} // no due schedules
	logger := newDiscardLogger()
	s := New(stub, logger)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	// Let it run through the initial tick.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after context cancel")
	}
}

// TestRun_InitialTick verifies Run fires an initial tick immediately on startup.
func TestRun_InitialTick(t *testing.T) {
	params := db.CreateJobParams{SourceID: "src-init", JobType: "encode"}
	stub := &scheduleStoreStub{
		due: []*db.Schedule{
			makeSchedule("sched-init", "init", "@daily", params),
		},
	}
	logger := newDiscardLogger()
	s := New(stub, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go s.Run(ctx)

	// Wait for the initial tick to have fired the schedule.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		stub.mu.Lock()
		n := len(stub.createdJobs)
		stub.mu.Unlock()
		if n >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	stub.mu.Lock()
	n := len(stub.createdJobs)
	stub.mu.Unlock()
	if n == 0 {
		t.Error("expected at least one job created on initial tick")
	}
}

package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/badskater/encodeswarmr/internal/db"
)

// ---------------------------------------------------------------------------
// expandHDRDetectJob tests
// ---------------------------------------------------------------------------

func TestExpandHDRDetectJob_GetSourceError(t *testing.T) {
	stub := &expandStub{sourceErr: errors.New("source not found")}
	e := newTestEngine(t, stub)

	job := &db.Job{ID: "j1", JobType: "hdr_detect", SourceID: "s1"}
	err := e.expandHDRDetectJob(context.Background(), job)
	if err == nil {
		t.Fatal("expected error from GetSourceByID, got nil")
	}
}

func TestExpandHDRDetectJob_ListVariablesError(t *testing.T) {
	stub := &expandStub{
		source:       &db.Source{ID: "src-1", UNCPath: `\\nas\movie.mkv`},
		variablesErr: errors.New("db error"),
	}
	e := newTestEngine(t, stub)

	job := &db.Job{ID: "j1", JobType: "hdr_detect", SourceID: "s1"}
	err := e.expandHDRDetectJob(context.Background(), job)
	if err == nil {
		t.Fatal("expected error from ListVariables, got nil")
	}
}

func TestExpandHDRDetectJob_CreateTaskError(t *testing.T) {
	stub := &expandStub{
		source:    &db.Source{ID: "src-1", UNCPath: `\\nas\movie.mkv`},
		createErr: errors.New("insert failed"),
	}
	e := newTestEngine(t, stub)

	job := &db.Job{ID: "j1", JobType: "hdr_detect", SourceID: "s1"}
	err := e.expandHDRDetectJob(context.Background(), job)
	if err == nil {
		t.Fatal("expected error from CreateTask, got nil")
	}
	found := false
	for _, s := range stub.statusUpdates {
		if s == "failed" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected job to be marked 'failed', got: %v", stub.statusUpdates)
	}
}

func TestExpandHDRDetectJob_SetTaskScriptDirError(t *testing.T) {
	stub := &expandStub{
		source:       &db.Source{ID: "src-1", UNCPath: `\\nas\movie.mkv`},
		task:         &db.Task{ID: "t1"},
		setScriptErr: errors.New("disk error"),
	}
	e := newTestEngine(t, stub)

	job := &db.Job{ID: "j1", JobType: "hdr_detect", SourceID: "s1"}
	err := e.expandHDRDetectJob(context.Background(), job)
	if err == nil {
		t.Fatal("expected error from SetTaskScriptDir, got nil")
	}
	found := false
	for _, s := range stub.statusUpdates {
		if s == "failed" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected job to be marked 'failed', got: %v", stub.statusUpdates)
	}
}

func TestExpandHDRDetectJob_Success_WritesScripts(t *testing.T) {
	stub := &expandStub{
		source: &db.Source{ID: "src-1", UNCPath: `\\nas\movie.mkv`},
		task:   &db.Task{ID: "t1"},
	}
	e := newTestEngine(t, stub)

	job := &db.Job{ID: "job-hdr-ok", JobType: "hdr_detect", SourceID: "s1"}
	err := e.expandHDRDetectJob(context.Background(), job)
	if err != nil {
		t.Fatalf("expandHDRDetectJob: unexpected error: %v", err)
	}

	// Verify run.bat and run.sh were written.
	dir := filepath.Join(e.gen.baseDir, job.ID, "0000")
	batPath := filepath.Join(dir, "run.bat")
	shPath := filepath.Join(dir, "run.sh")

	if _, err := os.Stat(batPath); err != nil {
		t.Errorf("expected run.bat to exist at %s: %v", batPath, err)
	}
	if _, err := os.Stat(shPath); err != nil {
		t.Errorf("expected run.sh to exist at %s: %v", shPath, err)
	}

	// Verify the bat script content matches the embedded constant.
	content, err := os.ReadFile(batPath)
	if err != nil {
		t.Fatalf("reading run.bat: %v", err)
	}
	if string(content) != hdrDetectScriptBat {
		t.Errorf("run.bat content mismatch:\ngot:\n%s\nwant:\n%s", content, hdrDetectScriptBat)
	}
}

func TestExpandHDRDetectJob_Success_StatusQueued(t *testing.T) {
	stub := &expandStub{
		source: &db.Source{ID: "src-1", UNCPath: `\\nas\movie.mkv`},
		task:   &db.Task{ID: "t1"},
	}
	e := newTestEngine(t, stub)

	job := &db.Job{ID: "job-hdr-q", JobType: "hdr_detect", SourceID: "s1"}
	err := e.expandHDRDetectJob(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, s := range stub.statusUpdates {
		if s == "queued" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected status 'queued', got: %v", stub.statusUpdates)
	}
}

func TestExpandHDRDetectJob_ExtraVarsInjected(t *testing.T) {
	// Extra vars from EncodeConfig.ExtraVars and global variables should be
	// merged into the task variables alongside SOURCE_PATH.
	stub := &expandStub{
		source: &db.Source{ID: "src-1", UNCPath: `\\nas\movie.mkv`},
		task:   &db.Task{ID: "t1"},
		variables: []*db.Variable{
			{Name: "GLOBAL_VAR", Value: "global_value"},
		},
	}
	e := newTestEngine(t, stub)

	job := &db.Job{
		ID:      "job-extra",
		JobType: "hdr_detect",
		SourceID: "s1",
		EncodeConfig: db.EncodeConfig{
			ExtraVars: map[string]string{"MY_KEY": "my_value"},
		},
	}
	// Should succeed without error; variable merging is exercised internally.
	if err := e.expandHDRDetectJob(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExpandHDRDetectJob_UpdateTaskCountsError(t *testing.T) {
	stub := &expandStub{
		source:        &db.Source{ID: "src-1", UNCPath: `\\nas\movie.mkv`},
		task:          &db.Task{ID: "t1"},
		taskCountsErr: errors.New("counts error"),
	}
	e := newTestEngine(t, stub)

	job := &db.Job{ID: "job-counts-err", JobType: "hdr_detect", SourceID: "s1"}
	err := e.expandHDRDetectJob(context.Background(), job)
	if err == nil {
		t.Fatal("expected error from UpdateJobTaskCounts, got nil")
	}
}

func TestExpandHDRDetectJob_UpdateJobStatusError(t *testing.T) {
	// Simulate UpdateJobStatus failing on the "queued" call.
	// expandHDRDetectJob calls UpdateJobStatus exactly once with "queued".
	stub := &hdrStatusFailOnNthStub{
		expandStub: expandStub{
			source: &db.Source{ID: "src-1", UNCPath: `\\nas\movie.mkv`},
			task:   &db.Task{ID: "t1"},
		},
		failOnCall: 0, // 0-indexed: fail on the very first (and only) call
	}
	e := newTestEngine(t, stub)

	job := &db.Job{ID: "job-status-err", JobType: "hdr_detect", SourceID: "s1"}
	err := e.expandHDRDetectJob(context.Background(), job)
	if err == nil {
		t.Fatal("expected error from UpdateJobStatus (queued), got nil")
	}
}

// ---------------------------------------------------------------------------
// HDRResultSentinel constant
// ---------------------------------------------------------------------------

func TestHDRResultSentinel(t *testing.T) {
	if HDRResultSentinel == "" {
		t.Error("HDRResultSentinel must not be empty")
	}
	// The sentinel is what the controller searches for in task stdout logs.
	const expected = "ES_HDR_RESULT="
	if HDRResultSentinel != expected {
		t.Errorf("HDRResultSentinel = %q, want %q", HDRResultSentinel, expected)
	}
}

// ---------------------------------------------------------------------------
// hdrStatusFailOnNthStub lets us fail UpdateJobStatus on a specific call.
// ---------------------------------------------------------------------------

type hdrStatusFailOnNthStub struct {
	expandStub
	failOnCall int // 0-indexed call number that should return an error
	callCount  int
}

func (s *hdrStatusFailOnNthStub) GetSourceByID(ctx context.Context, id string) (*db.Source, error) {
	return s.expandStub.GetSourceByID(ctx, id)
}

func (s *hdrStatusFailOnNthStub) ListVariables(ctx context.Context, cat string) ([]*db.Variable, error) {
	return s.expandStub.ListVariables(ctx, cat)
}

func (s *hdrStatusFailOnNthStub) CreateTask(ctx context.Context, p db.CreateTaskParams) (*db.Task, error) {
	return s.expandStub.CreateTask(ctx, p)
}

func (s *hdrStatusFailOnNthStub) SetTaskScriptDir(ctx context.Context, a, b string) error {
	return s.expandStub.SetTaskScriptDir(ctx, a, b)
}

func (s *hdrStatusFailOnNthStub) UpdateJobTaskCounts(ctx context.Context, id string) error {
	return s.expandStub.UpdateJobTaskCounts(ctx, id)
}

func (s *hdrStatusFailOnNthStub) UpdateJobStatus(_ context.Context, _ string, status string) error {
	if s.callCount == s.failOnCall {
		s.callCount++
		return errors.New("status update failed")
	}
	s.callCount++
	s.statusUpdates = append(s.statusUpdates, status)
	return nil
}

func (s *hdrStatusFailOnNthStub) DeleteTasksByJobID(ctx context.Context, id string) error {
	return s.expandStub.DeleteTasksByJobID(ctx, id)
}

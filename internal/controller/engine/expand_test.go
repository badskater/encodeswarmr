package engine

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/badskater/distributed-encoder/internal/db"
	"github.com/badskater/distributed-encoder/internal/db/teststore"
)

// ---------------------------------------------------------------------------
// Stubs for expand tests
// ---------------------------------------------------------------------------

// expandStub embeds teststore.Stub and overrides the methods needed for the
// expand code paths.
type expandStub struct {
	teststore.Stub
	mu sync.Mutex

	// GetJobsNeedingExpansion
	jobs    []*db.Job
	jobsErr error

	// GetSourceByID
	source    *db.Source
	sourceErr error

	// CreateTask
	task      *db.Task
	createErr error

	// SetTaskScriptDir
	setScriptErr error

	// UpdateJobStatus
	statusUpdates []string
	statusErr     error

	// UpdateJobTaskCounts
	taskCountsErr error

	// DeleteTasksByJobID
	deleteTasksErr error

	// ListVariables (needed by expandHDRDetectJob)
	variables    []*db.Variable
	variablesErr error

	// GetTemplateByID (needed by Render/RenderSingle when a template ID is set)
	template    *db.Template
	templateErr error
}

func (s *expandStub) GetJobsNeedingExpansion(_ context.Context) ([]*db.Job, error) {
	return s.jobs, s.jobsErr
}

func (s *expandStub) GetSourceByID(_ context.Context, _ string) (*db.Source, error) {
	return s.source, s.sourceErr
}

func (s *expandStub) CreateTask(_ context.Context, _ db.CreateTaskParams) (*db.Task, error) {
	return s.task, s.createErr
}

func (s *expandStub) SetTaskScriptDir(_ context.Context, _, _ string) error {
	return s.setScriptErr
}

func (s *expandStub) UpdateJobStatus(_ context.Context, _ string, status string) error {
	s.mu.Lock()
	s.statusUpdates = append(s.statusUpdates, status)
	s.mu.Unlock()
	return s.statusErr
}

func (s *expandStub) UpdateJobTaskCounts(_ context.Context, _ string) error {
	return s.taskCountsErr
}

func (s *expandStub) DeleteTasksByJobID(_ context.Context, _ string) error {
	return s.deleteTasksErr
}

func (s *expandStub) ListVariables(_ context.Context, _ string) ([]*db.Variable, error) {
	return s.variables, s.variablesErr
}

func (s *expandStub) GetTemplateByID(_ context.Context, _ string) (*db.Template, error) {
	return s.template, s.templateErr
}

// ---------------------------------------------------------------------------
// scriptGenStubForExpand satisfies the ScriptGenerator interface expected by
// the engine.  We embed a real ScriptGenerator backed by a no-op store so
// that calls to Render/RenderSingle succeed without touching the DB.
// ---------------------------------------------------------------------------

// newTestEngine creates an Engine wired with the given store, a real
// ScriptGenerator using a temp dir, and a discard logger.
func newTestEngine(t *testing.T, store db.Store) *Engine {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := Config{ScriptBaseDir: t.TempDir()}
	e := New(store, cfg, logger)
	return e
}

// ---------------------------------------------------------------------------
// isControllerAnalysisJob
// ---------------------------------------------------------------------------

func TestIsControllerAnalysisJob(t *testing.T) {
	tests := []struct {
		jobType string
		want    bool
	}{
		{"analysis", true},
		{"hdr_detect", true},
		{"audio", true},
		{"encode", false},
		{"concat", false},
		{"", false},
		{"unknown", false},
	}
	for _, tt := range tests {
		t.Run(tt.jobType, func(t *testing.T) {
			got := isControllerAnalysisJob(tt.jobType)
			if got != tt.want {
				t.Errorf("isControllerAnalysisJob(%q) = %v, want %v", tt.jobType, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// expandPendingJobs
// ---------------------------------------------------------------------------

func TestExpandPendingJobs_StoreError(t *testing.T) {
	stub := &expandStub{jobsErr: errors.New("db down")}
	e := newTestEngine(t, stub)

	err := e.expandPendingJobs(context.Background())
	if err == nil {
		t.Fatal("expected error when GetJobsNeedingExpansion fails")
	}
}

func TestExpandPendingJobs_NoJobs(t *testing.T) {
	stub := &expandStub{jobs: nil}
	e := newTestEngine(t, stub)

	if err := e.expandPendingJobs(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExpandPendingJobs_ExpandJobError_ContinuesOthers(t *testing.T) {
	// First job will fail expansion (unknown type), second is also unknown.
	// expandPendingJobs should log the warning but not return an error.
	jobs := []*db.Job{
		{ID: "j1", JobType: "unknown_type", SourceID: "s1"},
		{ID: "j2", JobType: "unknown_type", SourceID: "s2"},
	}
	stub := &expandStub{jobs: jobs}
	e := newTestEngine(t, stub)

	err := e.expandPendingJobs(context.Background())
	if err != nil {
		t.Fatalf("expandPendingJobs should not return error for individual job failures, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// expandJob routing
// ---------------------------------------------------------------------------

func TestExpandJob_UnknownType_ReturnsNil(t *testing.T) {
	stub := &expandStub{}
	e := newTestEngine(t, stub)

	job := &db.Job{ID: "j1", JobType: "mystery", SourceID: "s1"}
	err := e.expandJob(context.Background(), job)
	if err != nil {
		t.Fatalf("expandJob with unknown type should return nil, got: %v", err)
	}
}

func TestExpandJob_EncodeType_SourceError(t *testing.T) {
	stub := &expandStub{sourceErr: errors.New("source not found")}
	e := newTestEngine(t, stub)

	job := &db.Job{
		ID:      "j1",
		JobType: "encode",
		SourceID: "s1",
		EncodeConfig: db.EncodeConfig{
			ChunkBoundaries: []db.ChunkBoundary{{StartFrame: 0, EndFrame: 100}},
		},
	}
	err := e.expandJob(context.Background(), job)
	if err == nil {
		t.Fatal("expected error when GetSourceByID fails for encode job")
	}
}

func TestExpandJob_AnalysisType_RoutesToSingleTask(t *testing.T) {
	stub := &expandStub{
		source: &db.Source{ID: "s1", UNCPath: `\\nas\movie.mkv`},
		task:   &db.Task{ID: "t1"},
	}
	e := newTestEngine(t, stub)

	// analysis job with no run script template → RenderSingle returns a dir
	job := &db.Job{ID: "j1", JobType: "analysis", SourceID: "s1"}
	err := e.expandJob(context.Background(), job)
	if err != nil {
		t.Fatalf("expandJob analysis: unexpected error: %v", err)
	}
}

func TestExpandJob_AudioType_RoutesToSingleTask(t *testing.T) {
	stub := &expandStub{
		source: &db.Source{ID: "s1", UNCPath: `\\nas\movie.mkv`},
		task:   &db.Task{ID: "t1"},
	}
	e := newTestEngine(t, stub)

	job := &db.Job{ID: "j1", JobType: "audio", SourceID: "s1"}
	err := e.expandJob(context.Background(), job)
	if err != nil {
		t.Fatalf("expandJob audio: unexpected error: %v", err)
	}
}

func TestExpandJob_AnalysisType_WithAnalysisRunner(t *testing.T) {
	// When an AnalysisRunner is attached, analysis jobs should be routed to
	// expandControllerAnalysisJob instead of expandSingleTaskJob.
	stub := &expandStub{
		source: &db.Source{ID: "s1", UNCPath: `\\nas\movie.mkv`},
	}
	e := newTestEngine(t, stub)
	e.SetAnalysisRunner(&noopAnalysisRunner{})

	job := &db.Job{ID: "j1", JobType: "analysis", SourceID: "s1"}
	err := e.expandJob(context.Background(), job)
	if err != nil {
		t.Fatalf("expandJob analysis with runner: unexpected error: %v", err)
	}
	// expandControllerAnalysisJob spawns a goroutine that also calls
	// UpdateJobStatus("completed"). Wait briefly for it to finish to
	// avoid a data race on statusUpdates.
	time.Sleep(50 * time.Millisecond)

	stub.mu.Lock()
	updates := make([]string, len(stub.statusUpdates))
	copy(updates, stub.statusUpdates)
	stub.mu.Unlock()

	// The job should have been set to "running" (synchronously).
	if len(updates) == 0 || updates[0] != "running" {
		t.Errorf("expected job status set to 'running', got: %v", updates)
	}
}

// noopAnalysisRunner satisfies the AnalysisRunner interface for tests.
type noopAnalysisRunner struct{}

func (n *noopAnalysisRunner) RunHDRDetect(_ context.Context, _ *db.Job, _ *db.Source) error {
	return nil
}
func (n *noopAnalysisRunner) RunAnalysis(_ context.Context, _ *db.Job, _ *db.Source) error {
	return nil
}
func (n *noopAnalysisRunner) RunAudio(_ context.Context, _ *db.Job, _ *db.Source) error {
	return nil
}

// ---------------------------------------------------------------------------
// expandEncodeJob
// ---------------------------------------------------------------------------

func TestExpandEncodeJob_NoChunkBoundaries_ReturnsNil(t *testing.T) {
	stub := &expandStub{}
	e := newTestEngine(t, stub)

	job := &db.Job{
		ID:           "j1",
		JobType:      "encode",
		SourceID:     "s1",
		EncodeConfig: db.EncodeConfig{},
	}
	err := e.expandEncodeJob(context.Background(), job)
	if err != nil {
		t.Fatalf("expected nil for job with no chunk boundaries, got: %v", err)
	}
}

func TestExpandEncodeJob_GetSourceError(t *testing.T) {
	stub := &expandStub{
		sourceErr: errors.New("no such source"),
	}
	e := newTestEngine(t, stub)

	job := &db.Job{
		ID:      "j1",
		JobType: "encode",
		EncodeConfig: db.EncodeConfig{
			ChunkBoundaries: []db.ChunkBoundary{{StartFrame: 0, EndFrame: 100}},
		},
	}
	err := e.expandEncodeJob(context.Background(), job)
	if err == nil {
		t.Fatal("expected error when GetSourceByID fails")
	}
}

func TestExpandEncodeJob_CreateTaskError_FailsJob(t *testing.T) {
	stub := &expandStub{
		source:    &db.Source{ID: "s1", UNCPath: `\\nas\src.mkv`},
		createErr: errors.New("insert failed"),
	}
	e := newTestEngine(t, stub)

	job := &db.Job{
		ID:      "j1",
		JobType: "encode",
		EncodeConfig: db.EncodeConfig{
			ChunkBoundaries: []db.ChunkBoundary{
				{StartFrame: 0, EndFrame: 100},
			},
			OutputRoot: `\\nas\out`,
		},
	}
	err := e.expandEncodeJob(context.Background(), job)
	if err == nil {
		t.Fatal("expected error when CreateTask fails")
	}
	// The job should have been marked failed.
	found := false
	for _, s := range stub.statusUpdates {
		if s == "failed" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected job to be marked 'failed', statusUpdates=%v", stub.statusUpdates)
	}
}

func TestExpandEncodeJob_Success(t *testing.T) {
	stub := &expandStub{
		source: &db.Source{ID: "s1", UNCPath: `\\nas\src.mkv`},
		task:   &db.Task{ID: "task-1"},
		template: &db.Template{
			ID:        "tpl-1",
			Name:      "encode_run",
			Type:      "bat",
			Extension: "bat",
			Content:   `@echo off`,
		},
	}
	e := newTestEngine(t, stub)

	job := &db.Job{
		ID:      "job-enc-1",
		JobType: "encode",
		EncodeConfig: db.EncodeConfig{
			ChunkBoundaries: []db.ChunkBoundary{
				{StartFrame: 0, EndFrame: 99},
				{StartFrame: 100, EndFrame: 199},
			},
			OutputRoot:          t.TempDir(),
			OutputExtension:     "mkv",
			RunScriptTemplateID: "tpl-1",
		},
	}
	err := e.expandEncodeJob(context.Background(), job)
	if err != nil {
		t.Fatalf("expandEncodeJob: unexpected error: %v", err)
	}
	// Status should have been updated to "queued".
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

func TestExpandEncodeJob_DefaultExtension(t *testing.T) {
	// When OutputExtension is empty, the default "mkv" must be used (no error).
	stub := &expandStub{
		source: &db.Source{ID: "s1", UNCPath: `\\nas\src.mkv`},
		task:   &db.Task{ID: "t1"},
		template: &db.Template{
			ID:        "tpl-ext",
			Name:      "run",
			Type:      "bat",
			Extension: "bat",
			Content:   `@echo off`,
		},
	}
	e := newTestEngine(t, stub)

	job := &db.Job{
		ID:      "job-ext",
		JobType: "encode",
		EncodeConfig: db.EncodeConfig{
			ChunkBoundaries:     []db.ChunkBoundary{{StartFrame: 0, EndFrame: 10}},
			OutputRoot:          t.TempDir(),
			RunScriptTemplateID: "tpl-ext",
			// OutputExtension intentionally empty
		},
	}
	// Should not return an error; the code sets ext = "mkv" internally.
	err := e.expandEncodeJob(context.Background(), job)
	if err != nil {
		t.Fatalf("expected no error with empty OutputExtension: %v", err)
	}
}

// ---------------------------------------------------------------------------
// expandSingleTaskJob
// ---------------------------------------------------------------------------

func TestExpandSingleTaskJob_GetSourceError(t *testing.T) {
	stub := &expandStub{sourceErr: errors.New("src gone")}
	e := newTestEngine(t, stub)

	job := &db.Job{ID: "j1", JobType: "analysis", SourceID: "s1"}
	err := e.expandSingleTaskJob(context.Background(), job)
	if err == nil {
		t.Fatal("expected error from GetSourceByID")
	}
}

func TestExpandSingleTaskJob_CreateTaskError(t *testing.T) {
	stub := &expandStub{
		source:    &db.Source{ID: "s1", UNCPath: `\\nas\a.mkv`},
		createErr: errors.New("db error"),
	}
	e := newTestEngine(t, stub)

	job := &db.Job{ID: "j1", JobType: "analysis", SourceID: "s1"}
	err := e.expandSingleTaskJob(context.Background(), job)
	if err == nil {
		t.Fatal("expected error from CreateTask")
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

func TestExpandSingleTaskJob_Success(t *testing.T) {
	stub := &expandStub{
		source: &db.Source{ID: "s1", UNCPath: `\\nas\movie.mkv`},
		task:   &db.Task{ID: "t1"},
	}
	e := newTestEngine(t, stub)

	job := &db.Job{ID: "j-single", JobType: "analysis", SourceID: "s1"}
	err := e.expandSingleTaskJob(context.Background(), job)
	if err != nil {
		t.Fatalf("expandSingleTaskJob: unexpected error: %v", err)
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

func TestExpandSingleTaskJob_SetTaskScriptDirError(t *testing.T) {
	stub := &expandStub{
		source:       &db.Source{ID: "s1", UNCPath: `\\nas\a.mkv`},
		task:         &db.Task{ID: "t1"},
		setScriptErr: errors.New("disk full"),
	}
	e := newTestEngine(t, stub)

	job := &db.Job{ID: "j1", JobType: "analysis", SourceID: "s1"}
	err := e.expandSingleTaskJob(context.Background(), job)
	if err == nil {
		t.Fatal("expected error from SetTaskScriptDir")
	}
	found := false
	for _, s := range stub.statusUpdates {
		if s == "failed" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected job marked 'failed', got: %v", stub.statusUpdates)
	}
}

// ---------------------------------------------------------------------------
// expandControllerAnalysisJob
// ---------------------------------------------------------------------------

func TestExpandControllerAnalysisJob_GetSourceError(t *testing.T) {
	stub := &expandStub{sourceErr: errors.New("source missing")}
	e := newTestEngine(t, stub)
	e.SetAnalysisRunner(&noopAnalysisRunner{})

	job := &db.Job{ID: "j1", JobType: "analysis", SourceID: "s1"}
	err := e.expandControllerAnalysisJob(context.Background(), job)
	if err == nil {
		t.Fatal("expected error from GetSourceByID")
	}
}

func TestExpandControllerAnalysisJob_UpdateStatusError(t *testing.T) {
	stub := &expandStub{
		source:    &db.Source{ID: "s1", UNCPath: `\\nas\a.mkv`},
		statusErr: errors.New("db down"),
	}
	e := newTestEngine(t, stub)
	e.SetAnalysisRunner(&noopAnalysisRunner{})

	job := &db.Job{ID: "j1", JobType: "analysis", SourceID: "s1"}
	err := e.expandControllerAnalysisJob(context.Background(), job)
	if err == nil {
		t.Fatal("expected error from UpdateJobStatus")
	}
}

func TestExpandControllerAnalysisJob_RoutesHDRDetect(t *testing.T) {
	stub := &expandStub{
		source: &db.Source{ID: "s1", UNCPath: `\\nas\a.mkv`},
	}
	runner := newRecordingAnalysisRunner()
	e := newTestEngine(t, stub)
	e.SetAnalysisRunner(runner)

	job := &db.Job{ID: "j1", JobType: "hdr_detect", SourceID: "s1"}
	err := e.expandControllerAnalysisJob(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Allow the goroutine to complete.
	runner.waitForCall(t)
	if runner.getCalledMethod() != "RunHDRDetect" {
		t.Errorf("expected RunHDRDetect to be called, got %q", runner.getCalledMethod())
	}
}

func TestExpandControllerAnalysisJob_RoutesAnalysis(t *testing.T) {
	stub := &expandStub{
		source: &db.Source{ID: "s1", UNCPath: `\\nas\a.mkv`},
	}
	runner := newRecordingAnalysisRunner()
	e := newTestEngine(t, stub)
	e.SetAnalysisRunner(runner)

	job := &db.Job{ID: "j2", JobType: "analysis", SourceID: "s1"}
	if err := e.expandControllerAnalysisJob(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	runner.waitForCall(t)
	if runner.getCalledMethod() != "RunAnalysis" {
		t.Errorf("expected RunAnalysis, got %q", runner.getCalledMethod())
	}
}

func TestExpandControllerAnalysisJob_RoutesAudio(t *testing.T) {
	stub := &expandStub{
		source: &db.Source{ID: "s1", UNCPath: `\\nas\a.mkv`},
	}
	runner := newRecordingAnalysisRunner()
	e := newTestEngine(t, stub)
	e.SetAnalysisRunner(runner)

	job := &db.Job{ID: "j3", JobType: "audio", SourceID: "s1"}
	if err := e.expandControllerAnalysisJob(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	runner.waitForCall(t)
	if runner.getCalledMethod() != "RunAudio" {
		t.Errorf("expected RunAudio, got %q", runner.getCalledMethod())
	}
}

// recordingAnalysisRunner records which method was invoked and signals via a
// channel so tests can synchronise with the goroutine spawned by
// expandControllerAnalysisJob.
type recordingAnalysisRunner struct {
	mu           sync.Mutex
	calledMethod string
	done         chan struct{}
}

func newRecordingAnalysisRunner() *recordingAnalysisRunner {
	return &recordingAnalysisRunner{done: make(chan struct{}, 1)}
}

func (r *recordingAnalysisRunner) waitForCall(t *testing.T) {
	t.Helper()
	select {
	case <-r.done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for analysis runner call")
	}
}

func (r *recordingAnalysisRunner) getCalledMethod() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calledMethod
}

func (r *recordingAnalysisRunner) RunHDRDetect(_ context.Context, _ *db.Job, _ *db.Source) error {
	r.mu.Lock()
	r.calledMethod = "RunHDRDetect"
	r.mu.Unlock()
	r.done <- struct{}{}
	return nil
}

func (r *recordingAnalysisRunner) RunAnalysis(_ context.Context, _ *db.Job, _ *db.Source) error {
	r.mu.Lock()
	r.calledMethod = "RunAnalysis"
	r.mu.Unlock()
	r.done <- struct{}{}
	return nil
}

func (r *recordingAnalysisRunner) RunAudio(_ context.Context, _ *db.Job, _ *db.Source) error {
	r.mu.Lock()
	r.calledMethod = "RunAudio"
	r.mu.Unlock()
	r.done <- struct{}{}
	return nil
}

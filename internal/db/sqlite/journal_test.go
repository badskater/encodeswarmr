package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/badskater/distributed-encoder/internal/db/sqlite"
	_ "modernc.org/sqlite"
)

// openTempJournal opens a Journal backed by a temporary file that is cleaned
// up automatically when the test ends.  We use a file rather than ":memory:"
// because the Open() function appends "?_journal=WAL" to the DSN, and
// in-memory SQLite databases do not support WAL mode.
func openTempJournal(t *testing.T) *sqlite.Journal {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/journal.db"
	j, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("sqlite.Open(%s): %v", path, err)
	}
	t.Cleanup(func() { _ = j.Close() })
	return j
}

// now returns a rounded-to-second time to avoid sub-second precision issues
// when round-tripping through SQLite TEXT columns.
func now() time.Time {
	return time.Now().UTC().Truncate(time.Second)
}

// --- Open ---

func TestOpen_TempFile(t *testing.T) {
	j := openTempJournal(t)
	if j == nil {
		t.Fatal("Open returned nil Journal")
	}
}

// --- WriteResult / UnsyncedResults ---

func TestWriteResult_StoresEntry(t *testing.T) {
	j := openTempJournal(t)
	ctx := context.Background()

	r := sqlite.OfflineResult{
		TaskID:      "task-1",
		JobID:       "job-1",
		Success:     true,
		ExitCode:    0,
		ErrorMsg:    "",
		Metrics:     map[string]any{"fps": 24.5},
		StartedAt:   now().Add(-10 * time.Second),
		CompletedAt: now(),
	}

	if err := j.WriteResult(ctx, r); err != nil {
		t.Fatalf("WriteResult: %v", err)
	}

	results, err := j.UnsyncedResults(ctx)
	if err != nil {
		t.Fatalf("UnsyncedResults: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	got := results[0]
	if got.TaskID != r.TaskID {
		t.Errorf("TaskID = %q, want %q", got.TaskID, r.TaskID)
	}
	if got.JobID != r.JobID {
		t.Errorf("JobID = %q, want %q", got.JobID, r.JobID)
	}
	if !got.Success {
		t.Error("Success = false, want true")
	}
	if got.ExitCode != r.ExitCode {
		t.Errorf("ExitCode = %d, want %d", got.ExitCode, r.ExitCode)
	}
	if got.ID == 0 {
		t.Error("ID should be non-zero (autoincrement)")
	}
}

func TestUnsyncedResults_Empty(t *testing.T) {
	j := openTempJournal(t)
	ctx := context.Background()

	results, err := j.UnsyncedResults(ctx)
	if err != nil {
		t.Fatalf("UnsyncedResults: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestWriteResult_FailureFields(t *testing.T) {
	j := openTempJournal(t)
	ctx := context.Background()

	r := sqlite.OfflineResult{
		TaskID:      "task-fail",
		JobID:       "job-fail",
		Success:     false,
		ExitCode:    2,
		ErrorMsg:    "encode crashed",
		Metrics:     nil,
		StartedAt:   now(),
		CompletedAt: now(),
	}

	if err := j.WriteResult(ctx, r); err != nil {
		t.Fatalf("WriteResult: %v", err)
	}

	results, _ := j.UnsyncedResults(ctx)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	got := results[0]
	if got.Success {
		t.Error("Success = true, want false")
	}
	if got.ExitCode != 2 {
		t.Errorf("ExitCode = %d, want 2", got.ExitCode)
	}
	if got.ErrorMsg != "encode crashed" {
		t.Errorf("ErrorMsg = %q, want %q", got.ErrorMsg, "encode crashed")
	}
}

func TestWriteResult_MultipleOrdered(t *testing.T) {
	j := openTempJournal(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		r := sqlite.OfflineResult{
			TaskID:      "task",
			JobID:       "job",
			Success:     true,
			StartedAt:   now(),
			CompletedAt: now(),
		}
		if err := j.WriteResult(ctx, r); err != nil {
			t.Fatalf("WriteResult %d: %v", i, err)
		}
	}

	results, err := j.UnsyncedResults(ctx)
	if err != nil {
		t.Fatalf("UnsyncedResults: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	for i := 1; i < len(results); i++ {
		if results[i].ID <= results[i-1].ID {
			t.Errorf("results not ordered by ID: index %d (%d) <= index %d (%d)",
				i, results[i].ID, i-1, results[i-1].ID)
		}
	}
}

// --- MarkResultSynced ---

func TestMarkResultSynced_RemovesFromUnsynced(t *testing.T) {
	j := openTempJournal(t)
	ctx := context.Background()

	r := sqlite.OfflineResult{
		TaskID:      "task-1",
		JobID:       "job-1",
		Success:     true,
		StartedAt:   now(),
		CompletedAt: now(),
	}
	if err := j.WriteResult(ctx, r); err != nil {
		t.Fatalf("WriteResult: %v", err)
	}

	results, _ := j.UnsyncedResults(ctx)
	if len(results) != 1 {
		t.Fatalf("setup: expected 1 unsynced result")
	}

	if err := j.MarkResultSynced(ctx, results[0].ID); err != nil {
		t.Fatalf("MarkResultSynced: %v", err)
	}

	unsynced, err := j.UnsyncedResults(ctx)
	if err != nil {
		t.Fatalf("UnsyncedResults after MarkResultSynced: %v", err)
	}
	if len(unsynced) != 0 {
		t.Errorf("expected 0 unsynced results after MarkResultSynced, got %d", len(unsynced))
	}
}

// --- WriteLog / (no list func in package — just verify write succeeds) ---

func TestWriteLog_StoresEntry(t *testing.T) {
	j := openTempJournal(t)
	ctx := context.Background()

	l := sqlite.OfflineLog{
		TaskID:   "task-1",
		JobID:    "job-1",
		Stream:   "stdout",
		Level:    "info",
		Message:  "encoding started",
		Metadata: map[string]any{"key": "value"},
		LoggedAt: now(),
	}

	if err := j.WriteLog(ctx, l); err != nil {
		t.Fatalf("WriteLog: %v", err)
	}
}

func TestWriteLog_NilMetadata(t *testing.T) {
	j := openTempJournal(t)
	ctx := context.Background()

	l := sqlite.OfflineLog{
		TaskID:   "task-2",
		JobID:    "job-2",
		Stream:   "stderr",
		Level:    "warn",
		Message:  "low disk space",
		Metadata: nil,
		LoggedAt: now(),
	}

	if err := j.WriteLog(ctx, l); err != nil {
		t.Fatalf("WriteLog with nil metadata: %v", err)
	}
}

// --- MarkLogsSynced ---

func TestMarkLogsSynced_ByTaskID(t *testing.T) {
	j := openTempJournal(t)
	ctx := context.Background()

	// Write two log entries for the same task.
	for i := 0; i < 2; i++ {
		l := sqlite.OfflineLog{
			TaskID:   "task-A",
			JobID:    "job-1",
			Stream:   "stdout",
			Level:    "info",
			Message:  "line",
			LoggedAt: now(),
		}
		if err := j.WriteLog(ctx, l); err != nil {
			t.Fatalf("WriteLog %d: %v", i, err)
		}
	}

	if err := j.MarkLogsSynced(ctx, "task-A"); err != nil {
		t.Fatalf("MarkLogsSynced: %v", err)
	}

	// We can't directly query the logs table from outside the package, but
	// PruneSynced should remove them.
	if err := j.PruneSynced(ctx); err != nil {
		t.Fatalf("PruneSynced after MarkLogsSynced: %v", err)
	}
	// No panic / error is sufficient to verify the operation completed.
}

// --- PruneSynced ---

func TestPruneSynced_RemovesSyncedResults(t *testing.T) {
	j := openTempJournal(t)
	ctx := context.Background()

	r := sqlite.OfflineResult{
		TaskID:      "task-1",
		JobID:       "job-1",
		Success:     true,
		StartedAt:   now(),
		CompletedAt: now(),
	}
	if err := j.WriteResult(ctx, r); err != nil {
		t.Fatalf("WriteResult: %v", err)
	}

	results, _ := j.UnsyncedResults(ctx)
	if err := j.MarkResultSynced(ctx, results[0].ID); err != nil {
		t.Fatalf("MarkResultSynced: %v", err)
	}

	if err := j.PruneSynced(ctx); err != nil {
		t.Fatalf("PruneSynced: %v", err)
	}

	unsynced, err := j.UnsyncedResults(ctx)
	if err != nil {
		t.Fatalf("UnsyncedResults after PruneSynced: %v", err)
	}
	if len(unsynced) != 0 {
		t.Errorf("expected 0 unsynced results after prune, got %d", len(unsynced))
	}
}

func TestPruneSynced_KeepsUnsyncedResults(t *testing.T) {
	j := openTempJournal(t)
	ctx := context.Background()

	// Write result but do NOT mark it synced.
	r := sqlite.OfflineResult{
		TaskID:      "task-2",
		JobID:       "job-2",
		Success:     true,
		StartedAt:   now(),
		CompletedAt: now(),
	}
	if err := j.WriteResult(ctx, r); err != nil {
		t.Fatalf("WriteResult: %v", err)
	}

	if err := j.PruneSynced(ctx); err != nil {
		t.Fatalf("PruneSynced: %v", err)
	}

	unsynced, err := j.UnsyncedResults(ctx)
	if err != nil {
		t.Fatalf("UnsyncedResults: %v", err)
	}
	if len(unsynced) != 1 {
		t.Errorf("expected 1 unsynced result to survive prune, got %d", len(unsynced))
	}
}

func TestPruneSynced_EmptyStore(t *testing.T) {
	j := openTempJournal(t)
	ctx := context.Background()
	if err := j.PruneSynced(ctx); err != nil {
		t.Errorf("PruneSynced on empty store: %v", err)
	}
}

// --- Metrics JSON round-trip ---

func TestWriteResult_MetricsRoundTrip(t *testing.T) {
	j := openTempJournal(t)
	ctx := context.Background()

	metrics := map[string]any{
		"fps":      48.5,
		"vmaf":     94.2,
		"duration": "00:01:30",
	}
	r := sqlite.OfflineResult{
		TaskID:      "task-metrics",
		JobID:       "job-metrics",
		Success:     true,
		Metrics:     metrics,
		StartedAt:   now(),
		CompletedAt: now(),
	}

	if err := j.WriteResult(ctx, r); err != nil {
		t.Fatalf("WriteResult: %v", err)
	}

	results, err := j.UnsyncedResults(ctx)
	if err != nil {
		t.Fatalf("UnsyncedResults: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	got := results[0]
	if got.Metrics == nil {
		t.Fatal("Metrics is nil after round-trip")
	}
	if _, ok := got.Metrics["fps"]; !ok {
		t.Error("Metrics missing 'fps' key after round-trip")
	}
}

// --- Error paths (closed DB) ---

// openClosedJournal opens a Journal, closes it, and returns it so that
// subsequent method calls will operate on a closed database.
func openClosedJournal(t *testing.T) *sqlite.Journal {
	t.Helper()
	j := openTempJournal(t)
	// Close immediately; subsequent calls must return errors.
	if err := j.Close(); err != nil {
		t.Fatalf("openClosedJournal: close: %v", err)
	}
	return j
}

func TestOpen_InvalidPath(t *testing.T) {
	// A path whose parent directory does not exist causes SQLite to fail.
	badPath := t.TempDir() + "/nonexistent/deep/journal.db"
	_, err := sqlite.Open(badPath)
	if err == nil {
		t.Fatal("expected error for Open with unreachable path, got nil")
	}
}

func TestWriteResult_ClosedDB(t *testing.T) {
	j := openClosedJournal(t)
	ctx := context.Background()

	r := sqlite.OfflineResult{
		TaskID:      "task-x",
		JobID:       "job-x",
		Success:     true,
		StartedAt:   now(),
		CompletedAt: now(),
	}
	err := j.WriteResult(ctx, r)
	if err == nil {
		t.Fatal("expected error from WriteResult on closed DB, got nil")
	}
}

func TestWriteLog_ClosedDB(t *testing.T) {
	j := openClosedJournal(t)
	ctx := context.Background()

	l := sqlite.OfflineLog{
		TaskID:   "task-x",
		JobID:    "job-x",
		Stream:   "stdout",
		Level:    "info",
		Message:  "hello",
		LoggedAt: now(),
	}
	err := j.WriteLog(ctx, l)
	if err == nil {
		t.Fatal("expected error from WriteLog on closed DB, got nil")
	}
}

func TestUnsyncedResults_ClosedDB(t *testing.T) {
	j := openClosedJournal(t)
	ctx := context.Background()

	_, err := j.UnsyncedResults(ctx)
	if err == nil {
		t.Fatal("expected error from UnsyncedResults on closed DB, got nil")
	}
}

func TestMarkResultSynced_ClosedDB(t *testing.T) {
	j := openClosedJournal(t)
	ctx := context.Background()

	err := j.MarkResultSynced(ctx, 999)
	if err == nil {
		t.Fatal("expected error from MarkResultSynced on closed DB, got nil")
	}
}

func TestMarkLogsSynced_ClosedDB(t *testing.T) {
	j := openClosedJournal(t)
	ctx := context.Background()

	err := j.MarkLogsSynced(ctx, "task-x")
	if err == nil {
		t.Fatal("expected error from MarkLogsSynced on closed DB, got nil")
	}
}

func TestPruneSynced_ClosedDB(t *testing.T) {
	j := openClosedJournal(t)
	ctx := context.Background()

	err := j.PruneSynced(ctx)
	if err == nil {
		t.Fatal("expected error from PruneSynced on closed DB, got nil")
	}
}

// --- Close ---

func TestClose_Idempotent(t *testing.T) {
	dir := t.TempDir()
	j, err := sqlite.Open(dir + "/close_test.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := j.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second close — driver may return an error ("sql: database is closed"),
	// but must not panic.
	_ = j.Close()
}

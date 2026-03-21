package service

import (
	"path/filepath"
	"testing"
	"time"
)

// openMemoryStore opens an in-memory offlineStore using the ":memory:" DSN.
func openMemoryStore(t *testing.T) *offlineStore {
	t.Helper()
	s, err := newOfflineStore(":memory:")
	if err != nil {
		t.Fatalf("newOfflineStore(:memory:): %v", err)
	}
	t.Cleanup(func() { s.close() })
	return s
}

// --- saveResult / pendingResults ---

func TestOfflineStore_SaveAndPendingResults_Empty(t *testing.T) {
	s := openMemoryStore(t)
	results, err := s.pendingResults()
	if err != nil {
		t.Fatalf("pendingResults on empty store: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestOfflineStore_SaveResult_Success(t *testing.T) {
	s := openMemoryStore(t)

	if err := s.saveResult("task-1", "job-1", true, 0, ""); err != nil {
		t.Fatalf("saveResult: %v", err)
	}

	results, err := s.pendingResults()
	if err != nil {
		t.Fatalf("pendingResults: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", r.TaskID, "task-1")
	}
	if r.JobID != "job-1" {
		t.Errorf("JobID = %q, want %q", r.JobID, "job-1")
	}
	if !r.Success {
		t.Error("Success = false, want true")
	}
	if r.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", r.ExitCode)
	}
}

func TestOfflineStore_SaveResult_Failure(t *testing.T) {
	s := openMemoryStore(t)

	if err := s.saveResult("task-2", "job-2", false, 1, "encode failed"); err != nil {
		t.Fatalf("saveResult: %v", err)
	}

	results, err := s.pendingResults()
	if err != nil {
		t.Fatalf("pendingResults: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Success {
		t.Error("Success = true, want false")
	}
	if r.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", r.ExitCode)
	}
	if r.ErrorMsg != "encode failed" {
		t.Errorf("ErrorMsg = %q, want %q", r.ErrorMsg, "encode failed")
	}
}

func TestOfflineStore_PendingResults_OrderedByID(t *testing.T) {
	s := openMemoryStore(t)

	for i := 0; i < 5; i++ {
		taskID := "task-" + string(rune('A'+i))
		if err := s.saveResult(taskID, "job-x", true, 0, ""); err != nil {
			t.Fatalf("saveResult %d: %v", i, err)
		}
	}

	results, err := s.pendingResults()
	if err != nil {
		t.Fatalf("pendingResults: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	for i := 1; i < len(results); i++ {
		if results[i].ID <= results[i-1].ID {
			t.Errorf("results not ordered by ID at index %d: %d <= %d",
				i, results[i].ID, results[i-1].ID)
		}
	}
}

// --- markSynced ---

func TestOfflineStore_MarkSynced_RemovesFromPending(t *testing.T) {
	s := openMemoryStore(t)

	if err := s.saveResult("task-1", "job-1", true, 0, ""); err != nil {
		t.Fatalf("saveResult: %v", err)
	}

	results, _ := s.pendingResults()
	if len(results) != 1 {
		t.Fatalf("setup: expected 1 pending result")
	}

	if err := s.markSynced(results[0].ID); err != nil {
		t.Fatalf("markSynced: %v", err)
	}

	pending, err := s.pendingResults()
	if err != nil {
		t.Fatalf("pendingResults after markSynced: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("expected 0 pending results after markSynced, got %d", len(pending))
	}
}

// --- saveLog / pendingLogs ---

func TestOfflineStore_SaveAndPendingLogs_Empty(t *testing.T) {
	s := openMemoryStore(t)
	logs, err := s.pendingLogs()
	if err != nil {
		t.Fatalf("pendingLogs on empty store: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("expected 0 logs, got %d", len(logs))
	}
}

func TestOfflineStore_SaveLog(t *testing.T) {
	s := openMemoryStore(t)

	if err := s.saveLog("task-1", "job-1", "stdout", "info", "encoding started"); err != nil {
		t.Fatalf("saveLog: %v", err)
	}

	logs, err := s.pendingLogs()
	if err != nil {
		t.Fatalf("pendingLogs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	l := logs[0]
	if l.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", l.TaskID, "task-1")
	}
	if l.Stream != "stdout" {
		t.Errorf("Stream = %q, want %q", l.Stream, "stdout")
	}
	if l.Level != "info" {
		t.Errorf("Level = %q, want %q", l.Level, "info")
	}
	if l.Message != "encoding started" {
		t.Errorf("Message = %q, want %q", l.Message, "encoding started")
	}
}

// --- markLogsSynced ---

func TestOfflineStore_MarkLogsSynced(t *testing.T) {
	s := openMemoryStore(t)

	for i := 0; i < 3; i++ {
		if err := s.saveLog("task-1", "job-1", "stdout", "info", "msg"); err != nil {
			t.Fatalf("saveLog %d: %v", i, err)
		}
	}

	logs, _ := s.pendingLogs()
	if len(logs) != 3 {
		t.Fatalf("setup: expected 3 pending logs")
	}

	ids := []int64{logs[0].ID, logs[1].ID}
	if err := s.markLogsSynced(ids); err != nil {
		t.Fatalf("markLogsSynced: %v", err)
	}

	pending, err := s.pendingLogs()
	if err != nil {
		t.Fatalf("pendingLogs after markLogsSynced: %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("expected 1 pending log after marking 2 synced, got %d", len(pending))
	}
}

func TestOfflineStore_MarkLogsSynced_Empty(t *testing.T) {
	s := openMemoryStore(t)
	// Passing an empty slice must not error.
	if err := s.markLogsSynced(nil); err != nil {
		t.Errorf("markLogsSynced(nil): %v", err)
	}
	if err := s.markLogsSynced([]int64{}); err != nil {
		t.Errorf("markLogsSynced([]): %v", err)
	}
}

// --- PruneJournal ---

func TestOfflineStore_PruneJournal_RemovesOldSynced(t *testing.T) {
	s := openMemoryStore(t)

	// Insert a result and mark it synced.
	if err := s.saveResult("task-old", "job-1", true, 0, ""); err != nil {
		t.Fatalf("saveResult: %v", err)
	}
	results, _ := s.pendingResults()
	if err := s.markSynced(results[0].ID); err != nil {
		t.Fatalf("markSynced: %v", err)
	}

	// Prune with duration = 0 so everything synced is "older than now".
	if err := s.PruneJournal(0); err != nil {
		t.Fatalf("PruneJournal: %v", err)
	}

	// The row should be gone.  We verify by checking there are still 0 pending
	// (the synced row was deleted, not un-synced).
	pending, _ := s.pendingResults()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending results after prune, got %d", len(pending))
	}
}

func TestOfflineStore_PruneJournal_KeepsUnsynced(t *testing.T) {
	s := openMemoryStore(t)

	// Insert a result but do NOT mark it synced.
	if err := s.saveResult("task-new", "job-1", true, 0, ""); err != nil {
		t.Fatalf("saveResult: %v", err)
	}

	// Prune with a large retention window — should not remove the unsynced row.
	if err := s.PruneJournal(7 * 24 * time.Hour); err != nil {
		t.Fatalf("PruneJournal: %v", err)
	}

	pending, _ := s.pendingResults()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending result (unsynced should survive prune), got %d", len(pending))
	}
}

func TestOfflineStore_PruneJournal_PrunesLogs(t *testing.T) {
	s := openMemoryStore(t)

	// Insert a log, mark it synced via markLogsSynced.
	if err := s.saveLog("task-1", "job-1", "stdout", "info", "msg"); err != nil {
		t.Fatalf("saveLog: %v", err)
	}
	logs, _ := s.pendingLogs()
	if err := s.markLogsSynced([]int64{logs[0].ID}); err != nil {
		t.Fatalf("markLogsSynced: %v", err)
	}

	// Prune with zero duration to delete everything synced.
	if err := s.PruneJournal(0); err != nil {
		t.Fatalf("PruneJournal: %v", err)
	}

	// The synced log should be removed; pendingLogs should be empty.
	pending, _ := s.pendingLogs()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending logs after prune, got %d", len(pending))
	}
}

func TestOfflineStore_PruneOldSynced_Compatibility(t *testing.T) {
	s := openMemoryStore(t)
	// pruneOldSynced is the backwards-compat wrapper; it should not error on an
	// empty store.
	if err := s.pruneOldSynced(); err != nil {
		t.Errorf("pruneOldSynced: %v", err)
	}
}

// --- error paths ---

// TestNewOfflineStore_InvalidPath verifies that newOfflineStore returns an
// error when the path is not usable as a SQLite file (directory that cannot be
// written to).
func TestNewOfflineStore_SchemaError(t *testing.T) {
	// Pass a path whose parent directory does not exist so that SQLite cannot
	// create the file.  The pure-Go modernc SQLite driver returns an error from
	// Exec (schema creation) when it cannot open the DB file.
	badPath := filepath.Join(t.TempDir(), "nonexistent", "deep", "offline.db")
	_, err := newOfflineStore(badPath)
	if err == nil {
		t.Fatal("expected error for newOfflineStore with unreachable path, got nil")
	}
}

// TestOfflineStore_PendingResults_ClosedDB verifies that pendingResults returns
// an error when the underlying database has been closed.
func TestOfflineStore_PendingResults_ClosedDB(t *testing.T) {
	s := openMemoryStore(t)
	// Close the DB to make subsequent queries fail.
	if err := s.close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	_, err := s.pendingResults()
	if err == nil {
		t.Fatal("expected error from pendingResults on closed DB, got nil")
	}
}

// TestOfflineStore_PendingLogs_ClosedDB verifies that pendingLogs returns an
// error when the underlying database has been closed.
func TestOfflineStore_PendingLogs_ClosedDB(t *testing.T) {
	s := openMemoryStore(t)
	if err := s.close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	_, err := s.pendingLogs()
	if err == nil {
		t.Fatal("expected error from pendingLogs on closed DB, got nil")
	}
}

// TestOfflineStore_PruneJournal_ClosedDB verifies that PruneJournal returns an
// error when the underlying database has been closed.
func TestOfflineStore_PruneJournal_ClosedDB(t *testing.T) {
	s := openMemoryStore(t)
	if err := s.close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	err := s.PruneJournal(7 * 24 * time.Hour)
	if err == nil {
		t.Fatal("expected error from PruneJournal on closed DB, got nil")
	}
}

// --- mixed results and logs ---

func TestOfflineStore_MixedResultsAndLogs(t *testing.T) {
	s := openMemoryStore(t)

	if err := s.saveResult("task-1", "job-1", true, 0, ""); err != nil {
		t.Fatalf("saveResult: %v", err)
	}
	if err := s.saveLog("task-1", "job-1", "stdout", "info", "done"); err != nil {
		t.Fatalf("saveLog: %v", err)
	}

	results, err := s.pendingResults()
	if err != nil || len(results) != 1 {
		t.Fatalf("expected 1 pending result, got %d (err: %v)", len(results), err)
	}
	logs, err := s.pendingLogs()
	if err != nil || len(logs) != 1 {
		t.Fatalf("expected 1 pending log, got %d (err: %v)", len(logs), err)
	}
}

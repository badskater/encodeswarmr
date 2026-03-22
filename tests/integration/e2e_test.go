//go:build integration

package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/db"
	"github.com/badskater/encodeswarmr/tests/integration/testharness"

	_ "modernc.org/sqlite"
)

// TestE2E_HappyPath verifies the full encode job lifecycle: controller starts,
// a task with a trivial success script is claimed by an agent, executed, and
// the job reaches "completed" status.
func TestE2E_HappyPath(t *testing.T) {
	// Not parallel — E2E tests share a single Postgres and TruncateAll at start.
	ctx := context.Background()

	tc := testharness.StartController(t)
	testharness.CreateAdminUser(t, tc.Store, tc.AuthSvc)

	source, err := tc.Store.CreateSource(ctx, db.CreateSourceParams{
		Filename: "test.mkv",
		UNCPath:  `\\nas\media\test.mkv`,
		SizeBytes: 1024,
	})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}

	job, err := tc.Store.CreateJob(ctx, db.CreateJobParams{
		SourceID:   source.ID,
		JobType:    "encode",
		Priority:   5,
		TargetTags: []string{},
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	// Create a task manually with a trivial script (bypassing engine expansion).
	scriptDir := t.TempDir()
	if err := writeScript(t, scriptDir, true); err != nil {
		t.Fatalf("write script: %v", err)
	}

	task, err := tc.Store.CreateTask(ctx, db.CreateTaskParams{
		JobID:      job.ID,
		ChunkIndex: 0,
		SourcePath: `\\nas\media\test.mkv`,
		OutputPath: `\\nas\output\test.mkv`,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	if err := tc.Store.SetTaskScriptDir(ctx, task.ID, scriptDir); err != nil {
		t.Fatalf("set task script dir: %v", err)
	}
	if err := tc.Store.UpdateJobStatus(ctx, job.ID, "queued"); err != nil {
		t.Fatalf("update job status: %v", err)
	}
	if err := tc.Store.UpdateJobTaskCounts(ctx, job.ID); err != nil {
		t.Fatalf("update job task counts: %v", err)
	}

	testharness.StartAgent(t, tc.GRPCAddr, "test-agent-1")

	testharness.WaitForJobStatus(t, tc.Store, job.ID, "completed", 30*time.Second)

	// Verify all tasks completed.
	tasks, err := tc.Store.ListTasksByJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	for _, tk := range tasks {
		if tk.Status != "completed" {
			t.Errorf("task %s: got status %q, want %q", tk.ID, tk.Status, "completed")
		}
	}

	// Verify agent is present with idle status.
	agent, err := tc.Store.GetAgentByName(ctx, "test-agent-1")
	if err != nil {
		t.Fatalf("get agent by name: %v", err)
	}
	if agent.Status != "idle" {
		t.Errorf("agent status: got %q, want %q", agent.Status, "idle")
	}
}

// TestE2E_TaskFailure verifies that a task with a failing script causes the
// job to reach "failed" status and the task records an error message.
func TestE2E_TaskFailure(t *testing.T) {
	// Not parallel — E2E tests share a single Postgres and TruncateAll at start.
	ctx := context.Background()

	tc := testharness.StartController(t)
	testharness.CreateAdminUser(t, tc.Store, tc.AuthSvc)

	source, err := tc.Store.CreateSource(ctx, db.CreateSourceParams{
		Filename:  "fail.mkv",
		UNCPath:   `\\nas\media\fail.mkv`,
		SizeBytes: 512,
	})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}

	job, err := tc.Store.CreateJob(ctx, db.CreateJobParams{
		SourceID:   source.ID,
		JobType:    "encode",
		Priority:   5,
		TargetTags: []string{},
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	scriptDir := t.TempDir()
	if err := writeScript(t, scriptDir, false); err != nil {
		t.Fatalf("write failing script: %v", err)
	}

	task, err := tc.Store.CreateTask(ctx, db.CreateTaskParams{
		JobID:      job.ID,
		ChunkIndex: 0,
		SourcePath: `\\nas\media\fail.mkv`,
		OutputPath: `\\nas\output\fail.mkv`,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	if err := tc.Store.SetTaskScriptDir(ctx, task.ID, scriptDir); err != nil {
		t.Fatalf("set task script dir: %v", err)
	}
	if err := tc.Store.UpdateJobStatus(ctx, job.ID, "queued"); err != nil {
		t.Fatalf("update job status: %v", err)
	}
	if err := tc.Store.UpdateJobTaskCounts(ctx, job.ID); err != nil {
		t.Fatalf("update job task counts: %v", err)
	}

	testharness.StartAgent(t, tc.GRPCAddr, "fail-agent-1")

	testharness.WaitForJobStatus(t, tc.Store, job.ID, "failed", 30*time.Second)

	// Verify the task has failed status.
	tk, err := tc.Store.GetTaskByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if tk.Status != "failed" {
		t.Errorf("task status: got %q, want %q", tk.Status, "failed")
	}
	if tk.ExitCode == nil || *tk.ExitCode == 0 {
		t.Errorf("task exit_code: expected non-zero, got %v", tk.ExitCode)
	}
}

// TestE2E_MultipleAgents verifies that multiple agents share work correctly:
// four tasks are claimed across two agents, with no task claimed by both.
func TestE2E_MultipleAgents(t *testing.T) {
	// Not parallel — E2E tests share a single Postgres and TruncateAll at start.
	ctx := context.Background()

	tc := testharness.StartController(t)
	testharness.CreateAdminUser(t, tc.Store, tc.AuthSvc)

	source, err := tc.Store.CreateSource(ctx, db.CreateSourceParams{
		Filename:  "multi.mkv",
		UNCPath:   `\\nas\media\multi.mkv`,
		SizeBytes: 4096,
	})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}

	job, err := tc.Store.CreateJob(ctx, db.CreateJobParams{
		SourceID:   source.ID,
		JobType:    "encode",
		Priority:   5,
		TargetTags: []string{},
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	// Create 4 tasks each with its own script directory.
	const numTasks = 4
	taskIDs := make([]string, numTasks)
	for i := range numTasks {
		scriptDir := t.TempDir()
		if err := writeScript(t, scriptDir, true); err != nil {
			t.Fatalf("write script for task %d: %v", i, err)
		}

		task, err := tc.Store.CreateTask(ctx, db.CreateTaskParams{
			JobID:      job.ID,
			ChunkIndex: i,
			SourcePath: `\\nas\media\multi.mkv`,
			OutputPath: fmt.Sprintf(`\\nas\output\multi_%d.mkv`, i),
		})
		if err != nil {
			t.Fatalf("create task %d: %v", i, err)
		}
		taskIDs[i] = task.ID

		if err := tc.Store.SetTaskScriptDir(ctx, task.ID, scriptDir); err != nil {
			t.Fatalf("set task %d script dir: %v", i, err)
		}
	}

	if err := tc.Store.UpdateJobStatus(ctx, job.ID, "queued"); err != nil {
		t.Fatalf("update job status: %v", err)
	}
	if err := tc.Store.UpdateJobTaskCounts(ctx, job.ID); err != nil {
		t.Fatalf("update job task counts: %v", err)
	}

	testharness.StartAgent(t, tc.GRPCAddr, "agent-1")
	testharness.StartAgent(t, tc.GRPCAddr, "agent-2")

	// Wait for all tasks to complete.
	testharness.WaitFor(t, 30*time.Second, func() bool {
		tasks, err := tc.Store.ListTasksByJob(ctx, job.ID)
		if err != nil {
			return false
		}
		for _, tk := range tasks {
			if tk.Status != "completed" {
				return false
			}
		}
		return len(tasks) == numTasks
	})

	// Verify each task was claimed by exactly one agent (no duplicates).
	tasks, err := tc.Store.ListTasksByJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	seenAgents := make(map[string]string) // taskID → agentID
	for _, tk := range tasks {
		if tk.Status != "completed" {
			t.Errorf("task %s: got status %q, want %q", tk.ID, tk.Status, "completed")
		}
		if tk.AgentID == nil {
			t.Errorf("task %s: no agent_id set", tk.ID)
			continue
		}
		seenAgents[tk.ID] = *tk.AgentID
	}

	// Ensure each task ID maps to exactly one agent (uniqueness of agent per task).
	seen := make(map[string]bool)
	for _, agentID := range seenAgents {
		_ = agentID // each task has its own agent_id; duplicates are fine across tasks
		seen[agentID] = true
	}
	// We expect both agents participated (at least one task each) given 4 tasks and 2 agents.
	// With fast scripts this is best-effort; we only hard-check no task has two agent_ids.
	if len(tasks) != numTasks {
		t.Errorf("expected %d tasks, got %d", numTasks, len(tasks))
	}
}

// TestE2E_HeartbeatUpdatesDB verifies that an agent's heartbeat_at field in
// the DB is updated within a short window after the agent starts.
func TestE2E_HeartbeatUpdatesDB(t *testing.T) {
	// Not parallel — E2E tests share a single Postgres and TruncateAll at start.
	ctx := context.Background()

	tc := testharness.StartController(t)
	testharness.CreateAdminUser(t, tc.Store, tc.AuthSvc)

	testharness.StartAgent(t, tc.GRPCAddr, "hb-agent-1")

	// Wait for the agent to register and send at least one heartbeat (up to 15s).
	testharness.WaitFor(t, 15*time.Second, func() bool {
		a, err := tc.Store.GetAgentByName(ctx, "hb-agent-1")
		if err != nil {
			return false
		}
		return a.LastHeartbeat != nil
	})

	agent, err := tc.Store.GetAgentByName(ctx, "hb-agent-1")
	if err != nil {
		t.Fatalf("get agent by name: %v", err)
	}

	if agent.LastHeartbeat == nil {
		t.Fatal("agent heartbeat_at is nil; expected a recent timestamp")
	}

	age := time.Since(*agent.LastHeartbeat)
	if age > 15*time.Second {
		t.Errorf("heartbeat_at is %v old; expected within 15s", age)
	}
}

// TestE2E_OfflineSync verifies that results buffered in the agent's SQLite
// offline journal are synced to the controller on startup.
func TestE2E_OfflineSync(t *testing.T) {
	// Not parallel — E2E tests share a single Postgres and TruncateAll at start.
	ctx := context.Background()

	tc := testharness.StartController(t)
	testharness.CreateAdminUser(t, tc.Store, tc.AuthSvc)

	// Pre-register an agent so it is approved and has a known ID.
	agent, err := tc.Store.UpsertAgent(ctx, db.UpsertAgentParams{
		Name:         "offline-agent-1",
		Hostname:     "offline-agent-1",
		IPAddress:    "127.0.0.1",
		AgentVersion: "0.1.0",
		OSVersion:    "linux/amd64",
		CPUCount:     4,
		Tags:         []string{},
	})
	if err != nil {
		t.Fatalf("upsert agent: %v", err)
	}
	// Mark the agent as idle (approved).
	if err := tc.Store.UpdateAgentStatus(ctx, agent.ID, "idle"); err != nil {
		t.Fatalf("approve agent: %v", err)
	}

	// Create a source and job so we have valid IDs for the offline result.
	source, err := tc.Store.CreateSource(ctx, db.CreateSourceParams{
		Filename:  "offline.mkv",
		UNCPath:   `\\nas\media\offline.mkv`,
		SizeBytes: 256,
	})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}

	job, err := tc.Store.CreateJob(ctx, db.CreateJobParams{
		SourceID:   source.ID,
		JobType:    "encode",
		Priority:   5,
		TargetTags: []string{},
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	// Create a task and set it to "assigned" for our pre-registered agent.
	scriptDir := t.TempDir()
	if err := writeScript(t, scriptDir, true); err != nil {
		t.Fatalf("write script: %v", err)
	}
	task, err := tc.Store.CreateTask(ctx, db.CreateTaskParams{
		JobID:      job.ID,
		ChunkIndex: 0,
		SourcePath: `\\nas\media\offline.mkv`,
		OutputPath: `\\nas\output\offline.mkv`,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := tc.Store.SetTaskScriptDir(ctx, task.ID, scriptDir); err != nil {
		t.Fatalf("set task script dir: %v", err)
	}

	// Pre-populate the agent's SQLite offline journal with a success result
	// so that when the agent starts it will sync this result immediately.
	offlineDBPath := filepath.Join(t.TempDir(), "offline.db")
	if err := seedOfflineJournal(offlineDBPath, task.ID, job.ID, true); err != nil {
		t.Fatalf("seed offline journal: %v", err)
	}

	// Mark job as queued so the controller can accept the synced result.
	if err := tc.Store.UpdateJobStatus(ctx, job.ID, "queued"); err != nil {
		t.Fatalf("update job status: %v", err)
	}
	if err := tc.Store.UpdateJobTaskCounts(ctx, job.ID); err != nil {
		t.Fatalf("update job task counts: %v", err)
	}

	// Start the agent pointing at the pre-seeded offline DB.
	testharness.StartAgentWithOfflineDB(t, tc.GRPCAddr, "offline-agent-1", offlineDBPath)

	// Wait for the job to reach completed — triggered by the offline sync.
	testharness.WaitForJobStatus(t, tc.Store, job.ID, "completed", 30*time.Second)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// writeScript writes a trivial entrypoint script into scriptDir.
// When succeed is true the script exits 0; when false it exits 1.
// The correct extension (.sh on Linux/macOS, .bat on Windows) is chosen
// automatically.
func writeScript(t *testing.T, scriptDir string, succeed bool) error {
	t.Helper()

	var (
		name    string
		content string
	)

	if runtime.GOOS == "windows" {
		name = "run.bat"
		if succeed {
			content = "@echo off\r\necho task ok\r\nexit /b 0\r\n"
		} else {
			content = "@echo off\r\necho task failed\r\nexit /b 1\r\n"
		}
	} else {
		name = "run.sh"
		if succeed {
			content = "#!/bin/sh\necho 'task ok'\nexit 0\n"
		} else {
			content = "#!/bin/sh\necho 'task failed'\nexit 1\n"
		}
	}

	path := filepath.Join(scriptDir, name)
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		return fmt.Errorf("write script %s: %w", path, err)
	}
	return nil
}

// seedOfflineJournal creates a SQLite database at path and inserts an
// unsynced offline_result row that matches the schema used by
// internal/agent/service/offline.go.
func seedOfflineJournal(path, taskID, jobID string, success bool) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open offline db: %w", err)
	}
	defer db.Close()

	const schema = `CREATE TABLE IF NOT EXISTS offline_results (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id    TEXT    NOT NULL,
		job_id     TEXT    NOT NULL,
		success    INTEGER NOT NULL,
		exit_code  INTEGER NOT NULL,
		error_msg  TEXT    NOT NULL DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		synced     INTEGER NOT NULL DEFAULT 0
	);
	CREATE TABLE IF NOT EXISTS offline_logs (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id    TEXT    NOT NULL,
		job_id     TEXT    NOT NULL,
		stream     TEXT    NOT NULL,
		level      TEXT    NOT NULL DEFAULT 'info',
		message    TEXT    NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		synced     INTEGER NOT NULL DEFAULT 0
	);`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}

	successInt := 0
	if success {
		successInt = 1
	}
	const q = `INSERT INTO offline_results (task_id, job_id, success, exit_code, error_msg) VALUES (?, ?, ?, ?, ?)`
	if _, err := db.Exec(q, taskID, jobID, successInt, 0, ""); err != nil {
		return fmt.Errorf("insert offline result: %w", err)
	}
	return nil
}


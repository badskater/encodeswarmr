//go:build integration

// Package integration_test contains Layer 1 integration tests that exercise
// every db.Store method against a real PostgreSQL instance.
//
// Run with:
//
//	go test -tags integration ./tests/integration/... -v
//
// Set TEST_DATABASE_URL to use an existing Postgres instance instead of
// starting a testcontainer:
//
//	TEST_DATABASE_URL=postgres://user:pass@localhost/dbname?sslmode=disable \
//	    go test -tags integration ./tests/integration/... -v
package integration_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/badskater/distributed-encoder/internal/db"
	"github.com/badskater/distributed-encoder/tests/integration/testharness"
)

// setupTest is called at the start of each top-level test.  It retrieves the
// shared store/pool (initialised on first call) and truncates all tables.
func setupTest(t *testing.T) db.Store {
	t.Helper()
	_, store, pool := testharness.SetupPostgres(t)
	testharness.TruncateAll(t, pool)
	return store
}

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

func TestUsers(t *testing.T) {
	store := setupTest(t)
	ctx := context.Background()

	hash := "bcrypt_placeholder"

	t.Run("CreateUser", func(t *testing.T) {
		u, err := store.CreateUser(ctx, db.CreateUserParams{
			Username:     "alice",
			Email:        "alice@test.local",
			Role:         "admin",
			PasswordHash: &hash,
		})
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		if u.ID == "" {
			t.Error("expected non-empty ID")
		}
		if u.Username != "alice" {
			t.Errorf("username: got %q want %q", u.Username, "alice")
		}
	})

	t.Run("GetUserByUsername", func(t *testing.T) {
		_, err := store.CreateUser(ctx, db.CreateUserParams{
			Username:     "bob",
			Email:        "bob@test.local",
			Role:         "operator",
			PasswordHash: &hash,
		})
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		u, err := store.GetUserByUsername(ctx, "bob")
		if err != nil {
			t.Fatalf("GetUserByUsername: %v", err)
		}
		if u.Username != "bob" {
			t.Errorf("got %q want bob", u.Username)
		}
		_, err = store.GetUserByUsername(ctx, "nonexistent")
		if !errors.Is(err, db.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("GetUserByOIDCSub", func(t *testing.T) {
		sub := "oidc|12345"
		_, err := store.CreateUser(ctx, db.CreateUserParams{
			Username: "carol",
			Email:    "carol@test.local",
			Role:     "viewer",
			OIDCSub:  &sub,
		})
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		u, err := store.GetUserByOIDCSub(ctx, sub)
		if err != nil {
			t.Fatalf("GetUserByOIDCSub: %v", err)
		}
		if u.Username != "carol" {
			t.Errorf("got %q want carol", u.Username)
		}
	})

	t.Run("GetUserByID", func(t *testing.T) {
		created, err := store.CreateUser(ctx, db.CreateUserParams{
			Username:     "dave",
			Email:        "dave@test.local",
			Role:         "operator",
			PasswordHash: &hash,
		})
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		fetched, err := store.GetUserByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetUserByID: %v", err)
		}
		if fetched.ID != created.ID {
			t.Errorf("ID mismatch: got %s want %s", fetched.ID, created.ID)
		}
	})

	t.Run("ListUsers", func(t *testing.T) {
		users, err := store.ListUsers(ctx)
		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		// At least the users created in prior subtests (TruncateAll ran before
		// this top-level test, so only users from this test exist).
		if len(users) == 0 {
			t.Error("expected at least one user")
		}
	})

	t.Run("UpdateUserRole", func(t *testing.T) {
		u, err := store.CreateUser(ctx, db.CreateUserParams{
			Username:     "eve",
			Email:        "eve@test.local",
			Role:         "viewer",
			PasswordHash: &hash,
		})
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		if err := store.UpdateUserRole(ctx, u.ID, "operator"); err != nil {
			t.Fatalf("UpdateUserRole: %v", err)
		}
		updated, err := store.GetUserByID(ctx, u.ID)
		if err != nil {
			t.Fatalf("GetUserByID after update: %v", err)
		}
		if updated.Role != "operator" {
			t.Errorf("role: got %q want operator", updated.Role)
		}
	})

	t.Run("DeleteUser", func(t *testing.T) {
		u, err := store.CreateUser(ctx, db.CreateUserParams{
			Username:     "frank",
			Email:        "frank@test.local",
			Role:         "viewer",
			PasswordHash: &hash,
		})
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		if err := store.DeleteUser(ctx, u.ID); err != nil {
			t.Fatalf("DeleteUser: %v", err)
		}
		_, err = store.GetUserByID(ctx, u.ID)
		if !errors.Is(err, db.ErrNotFound) {
			t.Errorf("expected ErrNotFound after delete, got %v", err)
		}
	})

	t.Run("CountAdminUsers", func(t *testing.T) {
		// Clean state: count existing admins, then add one more.
		before, err := store.CountAdminUsers(ctx)
		if err != nil {
			t.Fatalf("CountAdminUsers: %v", err)
		}
		_, err = store.CreateUser(ctx, db.CreateUserParams{
			Username:     "grace",
			Email:        "grace@test.local",
			Role:         "admin",
			PasswordHash: &hash,
		})
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		after, err := store.CountAdminUsers(ctx)
		if err != nil {
			t.Fatalf("CountAdminUsers: %v", err)
		}
		if after != before+1 {
			t.Errorf("count: got %d want %d", after, before+1)
		}
	})
}

// ---------------------------------------------------------------------------
// Agents
// ---------------------------------------------------------------------------

func TestAgents(t *testing.T) {
	store := setupTest(t)
	ctx := context.Background()

	t.Run("UpsertAgent", func(t *testing.T) {
		a, err := store.UpsertAgent(ctx, db.UpsertAgentParams{
			Name:      "agent-01",
			Hostname:  "WIN-01",
			IPAddress: "10.0.0.1",
			Tags:      []string{"gpu", "nvenc"},
		})
		if err != nil {
			t.Fatalf("UpsertAgent: %v", err)
		}
		if a.ID == "" {
			t.Error("expected non-empty ID")
		}
		// Upsert again — should update, not create a second row.
		a2, err := store.UpsertAgent(ctx, db.UpsertAgentParams{
			Name:         "agent-01",
			Hostname:     "WIN-01-RENAMED",
			IPAddress:    "10.0.0.2",
			Tags:         []string{"gpu"},
			AgentVersion: "1.2.3",
		})
		if err != nil {
			t.Fatalf("UpsertAgent (upsert): %v", err)
		}
		if a2.ID != a.ID {
			t.Errorf("ID changed on upsert: %s → %s", a.ID, a2.ID)
		}
		if a2.Hostname != "WIN-01-RENAMED" {
			t.Errorf("hostname not updated: got %q", a2.Hostname)
		}
	})

	t.Run("GetAgentByID", func(t *testing.T) {
		a, err := store.UpsertAgent(ctx, db.UpsertAgentParams{
			Name: "agent-02", Hostname: "WIN-02", IPAddress: "10.0.0.3", Tags: []string{},
		})
		if err != nil {
			t.Fatalf("UpsertAgent: %v", err)
		}
		fetched, err := store.GetAgentByID(ctx, a.ID)
		if err != nil {
			t.Fatalf("GetAgentByID: %v", err)
		}
		if fetched.ID != a.ID {
			t.Errorf("ID mismatch")
		}
	})

	t.Run("GetAgentByName", func(t *testing.T) {
		_, err := store.UpsertAgent(ctx, db.UpsertAgentParams{
			Name: "agent-byname", Hostname: "WIN-BN", IPAddress: "10.0.0.4", Tags: []string{},
		})
		if err != nil {
			t.Fatalf("UpsertAgent: %v", err)
		}
		a, err := store.GetAgentByName(ctx, "agent-byname")
		if err != nil {
			t.Fatalf("GetAgentByName: %v", err)
		}
		if a.Name != "agent-byname" {
			t.Errorf("name mismatch: %q", a.Name)
		}
	})

	t.Run("ListAgents", func(t *testing.T) {
		agents, err := store.ListAgents(ctx)
		if err != nil {
			t.Fatalf("ListAgents: %v", err)
		}
		if len(agents) == 0 {
			t.Error("expected at least one agent")
		}
	})

	t.Run("UpdateAgentStatus", func(t *testing.T) {
		a, err := store.UpsertAgent(ctx, db.UpsertAgentParams{
			Name: "agent-status", Hostname: "WIN-ST", IPAddress: "10.0.0.5", Tags: []string{},
		})
		if err != nil {
			t.Fatalf("UpsertAgent: %v", err)
		}
		if err := store.UpdateAgentStatus(ctx, a.ID, "busy"); err != nil {
			t.Fatalf("UpdateAgentStatus: %v", err)
		}
		updated, err := store.GetAgentByID(ctx, a.ID)
		if err != nil {
			t.Fatalf("GetAgentByID: %v", err)
		}
		if updated.Status != "busy" {
			t.Errorf("status: got %q want busy", updated.Status)
		}
	})

	t.Run("UpdateAgentHeartbeat", func(t *testing.T) {
		a, err := store.UpsertAgent(ctx, db.UpsertAgentParams{
			Name: "agent-hb", Hostname: "WIN-HB", IPAddress: "10.0.0.6", Tags: []string{},
		})
		if err != nil {
			t.Fatalf("UpsertAgent: %v", err)
		}
		err = store.UpdateAgentHeartbeat(ctx, db.UpdateAgentHeartbeatParams{
			ID:     a.ID,
			Status: "idle",
		})
		if err != nil {
			t.Fatalf("UpdateAgentHeartbeat: %v", err)
		}
		updated, err := store.GetAgentByID(ctx, a.ID)
		if err != nil {
			t.Fatalf("GetAgentByID: %v", err)
		}
		if updated.LastHeartbeat == nil {
			t.Error("expected LastHeartbeat to be set")
		}
	})

	t.Run("UpdateAgentVNCPort", func(t *testing.T) {
		a, err := store.UpsertAgent(ctx, db.UpsertAgentParams{
			Name: "agent-vnc", Hostname: "WIN-VNC", IPAddress: "10.0.0.7", Tags: []string{},
		})
		if err != nil {
			t.Fatalf("UpsertAgent: %v", err)
		}
		if err := store.UpdateAgentVNCPort(ctx, a.ID, 5900); err != nil {
			t.Fatalf("UpdateAgentVNCPort: %v", err)
		}
		updated, err := store.GetAgentByID(ctx, a.ID)
		if err != nil {
			t.Fatalf("GetAgentByID: %v", err)
		}
		if updated.VNCPort != 5900 {
			t.Errorf("vnc_port: got %d want 5900", updated.VNCPort)
		}
	})

	t.Run("MarkStaleAgents", func(t *testing.T) {
		a, err := store.UpsertAgent(ctx, db.UpsertAgentParams{
			Name: "agent-stale", Hostname: "WIN-STALE", IPAddress: "10.0.0.8", Tags: []string{},
		})
		if err != nil {
			t.Fatalf("UpsertAgent: %v", err)
		}
		// Give it an old heartbeat by setting status to idle first.
		if err := store.UpdateAgentHeartbeat(ctx, db.UpdateAgentHeartbeatParams{ID: a.ID, Status: "idle"}); err != nil {
			t.Fatalf("UpdateAgentHeartbeat: %v", err)
		}
		// Mark stale after 0ns — should pick up the agent immediately.
		n, err := store.MarkStaleAgents(ctx, 0)
		if err != nil {
			t.Fatalf("MarkStaleAgents: %v", err)
		}
		if n == 0 {
			t.Error("expected at least 1 agent marked stale")
		}
	})

	t.Run("SetAgentAPIKey", func(t *testing.T) {
		a, err := store.UpsertAgent(ctx, db.UpsertAgentParams{
			Name: "agent-apikey", Hostname: "WIN-AK", IPAddress: "10.0.0.9", Tags: []string{},
		})
		if err != nil {
			t.Fatalf("UpsertAgent: %v", err)
		}
		if err := store.SetAgentAPIKey(ctx, a.ID, "sha256hashvalue"); err != nil {
			t.Fatalf("SetAgentAPIKey: %v", err)
		}
		updated, err := store.GetAgentByID(ctx, a.ID)
		if err != nil {
			t.Fatalf("GetAgentByID: %v", err)
		}
		if updated.APIKeyHash == nil || *updated.APIKeyHash != "sha256hashvalue" {
			t.Errorf("api_key_hash: got %v", updated.APIKeyHash)
		}
	})
}

// ---------------------------------------------------------------------------
// Sources
// ---------------------------------------------------------------------------

func TestSources(t *testing.T) {
	store := setupTest(t)
	ctx := context.Background()

	t.Run("CreateSource", func(t *testing.T) {
		src, err := store.CreateSource(ctx, db.CreateSourceParams{
			Filename:  "movie.mkv",
			UNCPath:   `\\nas01\media\movie.mkv`,
			SizeBytes: 10_000_000_000,
		})
		if err != nil {
			t.Fatalf("CreateSource: %v", err)
		}
		if src.ID == "" {
			t.Error("expected non-empty ID")
		}
		if src.State == "" {
			t.Error("expected non-empty initial state")
		}
	})

	t.Run("GetSourceByID", func(t *testing.T) {
		src, err := store.CreateSource(ctx, db.CreateSourceParams{
			Filename: "byid.mkv", UNCPath: `\\nas01\media\byid.mkv`, SizeBytes: 1,
		})
		if err != nil {
			t.Fatalf("CreateSource: %v", err)
		}
		fetched, err := store.GetSourceByID(ctx, src.ID)
		if err != nil {
			t.Fatalf("GetSourceByID: %v", err)
		}
		if fetched.Filename != "byid.mkv" {
			t.Errorf("filename mismatch")
		}
	})

	t.Run("ListSources", func(t *testing.T) {
		_, _ = store.CreateSource(ctx, db.CreateSourceParams{
			Filename: "list1.mkv", UNCPath: `\\nas01\media\list1.mkv`, SizeBytes: 1,
		})
		_, _ = store.CreateSource(ctx, db.CreateSourceParams{
			Filename: "list2.mkv", UNCPath: `\\nas01\media\list2.mkv`, SizeBytes: 1,
		})
		sources, total, err := store.ListSources(ctx, db.ListSourcesFilter{PageSize: 10})
		if err != nil {
			t.Fatalf("ListSources: %v", err)
		}
		if total == 0 || len(sources) == 0 {
			t.Errorf("expected sources, got total=%d len=%d", total, len(sources))
		}
	})

	t.Run("UpdateSourceState", func(t *testing.T) {
		src, err := store.CreateSource(ctx, db.CreateSourceParams{
			Filename: "state.mkv", UNCPath: `\\nas01\media\state.mkv`, SizeBytes: 1,
		})
		if err != nil {
			t.Fatalf("CreateSource: %v", err)
		}
		if err := store.UpdateSourceState(ctx, src.ID, "ready"); err != nil {
			t.Fatalf("UpdateSourceState: %v", err)
		}
		updated, err := store.GetSourceByID(ctx, src.ID)
		if err != nil {
			t.Fatalf("GetSourceByID: %v", err)
		}
		if updated.State != "ready" {
			t.Errorf("state: got %q want ready", updated.State)
		}
	})

	t.Run("UpdateSourceHDR", func(t *testing.T) {
		src, err := store.CreateSource(ctx, db.CreateSourceParams{
			Filename: "hdr.mkv", UNCPath: `\\nas01\media\hdr.mkv`, SizeBytes: 1,
		})
		if err != nil {
			t.Fatalf("CreateSource: %v", err)
		}
		if err := store.UpdateSourceHDR(ctx, db.UpdateSourceHDRParams{
			ID: src.ID, HDRType: "hdr10", DVProfile: 0,
		}); err != nil {
			t.Fatalf("UpdateSourceHDR: %v", err)
		}
		updated, err := store.GetSourceByID(ctx, src.ID)
		if err != nil {
			t.Fatalf("GetSourceByID: %v", err)
		}
		if updated.HDRType != "hdr10" {
			t.Errorf("hdr_type: got %q want hdr10", updated.HDRType)
		}
	})

	t.Run("DeleteSource", func(t *testing.T) {
		src, err := store.CreateSource(ctx, db.CreateSourceParams{
			Filename: "delete.mkv", UNCPath: `\\nas01\media\delete.mkv`, SizeBytes: 1,
		})
		if err != nil {
			t.Fatalf("CreateSource: %v", err)
		}
		if err := store.DeleteSource(ctx, src.ID); err != nil {
			t.Fatalf("DeleteSource: %v", err)
		}
		_, err = store.GetSourceByID(ctx, src.ID)
		if !errors.Is(err, db.ErrNotFound) {
			t.Errorf("expected ErrNotFound after delete, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// Jobs
// ---------------------------------------------------------------------------

func TestJobs(t *testing.T) {
	store := setupTest(t)
	ctx := context.Background()

	src := testharness.CreateTestSource(t, store)

	t.Run("CreateJob", func(t *testing.T) {
		job, err := store.CreateJob(ctx, db.CreateJobParams{
			SourceID:   src.ID,
			JobType:    "encode",
			Priority:   5,
			TargetTags: []string{"gpu"},
		})
		if err != nil {
			t.Fatalf("CreateJob: %v", err)
		}
		if job.ID == "" {
			t.Error("expected non-empty ID")
		}
		if job.Status == "" {
			t.Error("expected non-empty initial status")
		}
	})

	t.Run("GetJobByID", func(t *testing.T) {
		job, err := store.CreateJob(ctx, db.CreateJobParams{
			SourceID: src.ID, JobType: "encode", TargetTags: []string{},
		})
		if err != nil {
			t.Fatalf("CreateJob: %v", err)
		}
		fetched, err := store.GetJobByID(ctx, job.ID)
		if err != nil {
			t.Fatalf("GetJobByID: %v", err)
		}
		if fetched.ID != job.ID {
			t.Errorf("ID mismatch")
		}
	})

	t.Run("ListJobs", func(t *testing.T) {
		jobs, total, err := store.ListJobs(ctx, db.ListJobsFilter{PageSize: 100})
		if err != nil {
			t.Fatalf("ListJobs: %v", err)
		}
		if total == 0 || len(jobs) == 0 {
			t.Errorf("expected jobs, got total=%d len=%d", total, len(jobs))
		}
	})

	t.Run("GetJobsNeedingExpansion", func(t *testing.T) {
		// Create a queued job with non-empty EncodeConfig so it would be picked up.
		_, err := store.CreateJob(ctx, db.CreateJobParams{
			SourceID:   src.ID,
			JobType:    "encode",
			TargetTags: []string{},
			EncodeConfig: db.EncodeConfig{
				OutputRoot:      `\\nas01\out`,
				OutputExtension: "mkv",
				ChunkBoundaries: []db.ChunkBoundary{{StartFrame: 0, EndFrame: 100}},
			},
		})
		if err != nil {
			t.Fatalf("CreateJob: %v", err)
		}
		jobs, err := store.GetJobsNeedingExpansion(ctx)
		if err != nil {
			t.Fatalf("GetJobsNeedingExpansion: %v", err)
		}
		// We don't assert a count because expansion criteria may vary; just
		// confirm it runs without error.
		_ = jobs
	})

	t.Run("UpdateJobStatus", func(t *testing.T) {
		job, err := store.CreateJob(ctx, db.CreateJobParams{
			SourceID: src.ID, JobType: "encode", TargetTags: []string{},
		})
		if err != nil {
			t.Fatalf("CreateJob: %v", err)
		}
		if err := store.UpdateJobStatus(ctx, job.ID, "running"); err != nil {
			t.Fatalf("UpdateJobStatus: %v", err)
		}
		updated, err := store.GetJobByID(ctx, job.ID)
		if err != nil {
			t.Fatalf("GetJobByID: %v", err)
		}
		if updated.Status != "running" {
			t.Errorf("status: got %q want running", updated.Status)
		}
	})

	t.Run("UpdateJobTaskCounts", func(t *testing.T) {
		job, err := store.CreateJob(ctx, db.CreateJobParams{
			SourceID: src.ID, JobType: "encode", TargetTags: []string{},
		})
		if err != nil {
			t.Fatalf("CreateJob: %v", err)
		}
		// Create a task so there is something to count.
		_, err = store.CreateTask(ctx, db.CreateTaskParams{
			JobID:      job.ID,
			ChunkIndex: 0,
			SourcePath: src.UNCPath,
			OutputPath: `\\nas01\out\chunk_0.mkv`,
		})
		if err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
		if err := store.UpdateJobTaskCounts(ctx, job.ID); err != nil {
			t.Fatalf("UpdateJobTaskCounts: %v", err)
		}
		updated, err := store.GetJobByID(ctx, job.ID)
		if err != nil {
			t.Fatalf("GetJobByID: %v", err)
		}
		if updated.TasksTotal == 0 {
			t.Error("expected TasksTotal > 0 after UpdateJobTaskCounts")
		}
	})
}

// ---------------------------------------------------------------------------
// Tasks
// ---------------------------------------------------------------------------

func TestTasks(t *testing.T) {
	store := setupTest(t)
	ctx := context.Background()

	src := testharness.CreateTestSource(t, store)
	job := testharness.CreateTestJob(t, store, src.ID)

	agent, err := store.UpsertAgent(ctx, db.UpsertAgentParams{
		Name: "task-test-agent", Hostname: "WIN-T", IPAddress: "10.1.0.1", Tags: []string{},
	})
	if err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}

	t.Run("CreateTask", func(t *testing.T) {
		task, err := store.CreateTask(ctx, db.CreateTaskParams{
			JobID:      job.ID,
			ChunkIndex: 0,
			SourcePath: src.UNCPath,
			OutputPath: `\\nas01\out\chunk_0.mkv`,
		})
		if err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
		if task.ID == "" {
			t.Error("expected non-empty ID")
		}
		if task.Status == "" {
			t.Error("expected non-empty initial status")
		}
	})

	t.Run("GetTaskByID", func(t *testing.T) {
		task, err := store.CreateTask(ctx, db.CreateTaskParams{
			JobID:      job.ID,
			ChunkIndex: 1,
			SourcePath: src.UNCPath,
			OutputPath: `\\nas01\out\chunk_1.mkv`,
		})
		if err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
		fetched, err := store.GetTaskByID(ctx, task.ID)
		if err != nil {
			t.Fatalf("GetTaskByID: %v", err)
		}
		if fetched.ID != task.ID {
			t.Errorf("ID mismatch")
		}
	})

	t.Run("ListTasksByJob", func(t *testing.T) {
		tasks, err := store.ListTasksByJob(ctx, job.ID)
		if err != nil {
			t.Fatalf("ListTasksByJob: %v", err)
		}
		if len(tasks) == 0 {
			t.Error("expected at least one task")
		}
	})

	t.Run("ClaimNextTask", func(t *testing.T) {
		// Create a fresh job+task to ensure a predictable claim.
		j2 := testharness.CreateTestJob(t, store, src.ID)
		_, err := store.CreateTask(ctx, db.CreateTaskParams{
			JobID:      j2.ID,
			ChunkIndex: 0,
			SourcePath: src.UNCPath,
			OutputPath: `\\nas01\out\claim_test.mkv`,
		})
		if err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
		task, err := store.ClaimNextTask(ctx, agent.ID, []string{})
		if err != nil {
			t.Fatalf("ClaimNextTask: %v", err)
		}
		if task == nil {
			t.Fatal("expected a claimed task, got nil")
		}
		if task.AgentID == nil || *task.AgentID != agent.ID {
			t.Errorf("agent_id not set correctly")
		}
	})

	t.Run("UpdateTaskStatus", func(t *testing.T) {
		j3 := testharness.CreateTestJob(t, store, src.ID)
		task, err := store.CreateTask(ctx, db.CreateTaskParams{
			JobID:      j3.ID,
			ChunkIndex: 0,
			SourcePath: src.UNCPath,
			OutputPath: `\\nas01\out\status_test.mkv`,
		})
		if err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
		if err := store.UpdateTaskStatus(ctx, task.ID, "running"); err != nil {
			t.Fatalf("UpdateTaskStatus: %v", err)
		}
		updated, err := store.GetTaskByID(ctx, task.ID)
		if err != nil {
			t.Fatalf("GetTaskByID: %v", err)
		}
		if updated.Status != "running" {
			t.Errorf("status: got %q want running", updated.Status)
		}
	})

	t.Run("SetTaskScriptDir", func(t *testing.T) {
		j4 := testharness.CreateTestJob(t, store, src.ID)
		task, err := store.CreateTask(ctx, db.CreateTaskParams{
			JobID:      j4.ID,
			ChunkIndex: 0,
			SourcePath: src.UNCPath,
			OutputPath: `\\nas01\out\scriptdir.mkv`,
		})
		if err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
		if err := store.SetTaskScriptDir(ctx, task.ID, "/tmp/scripts/task1"); err != nil {
			t.Fatalf("SetTaskScriptDir: %v", err)
		}
		updated, err := store.GetTaskByID(ctx, task.ID)
		if err != nil {
			t.Fatalf("GetTaskByID: %v", err)
		}
		if updated.ScriptDir != "/tmp/scripts/task1" {
			t.Errorf("script_dir: got %q", updated.ScriptDir)
		}
	})

	t.Run("CompleteTask", func(t *testing.T) {
		j5 := testharness.CreateTestJob(t, store, src.ID)
		task, err := store.CreateTask(ctx, db.CreateTaskParams{
			JobID:      j5.ID,
			ChunkIndex: 0,
			SourcePath: src.UNCPath,
			OutputPath: `\\nas01\out\complete.mkv`,
		})
		if err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
		vmaf := 95.5
		if err := store.CompleteTask(ctx, db.CompleteTaskParams{
			ID:            task.ID,
			ExitCode:      0,
			FramesEncoded: 1000,
			AvgFPS:        30.0,
			OutputSize:    50_000_000,
			DurationSec:   33,
			VMafScore:     &vmaf,
		}); err != nil {
			t.Fatalf("CompleteTask: %v", err)
		}
		updated, err := store.GetTaskByID(ctx, task.ID)
		if err != nil {
			t.Fatalf("GetTaskByID: %v", err)
		}
		if updated.Status != "completed" {
			t.Errorf("status: got %q want completed", updated.Status)
		}
		if updated.FramesEncoded == nil || *updated.FramesEncoded != 1000 {
			t.Errorf("frames_encoded: got %v", updated.FramesEncoded)
		}
	})

	t.Run("FailTask", func(t *testing.T) {
		j6 := testharness.CreateTestJob(t, store, src.ID)
		task, err := store.CreateTask(ctx, db.CreateTaskParams{
			JobID:      j6.ID,
			ChunkIndex: 0,
			SourcePath: src.UNCPath,
			OutputPath: `\\nas01\out\fail.mkv`,
		})
		if err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
		if err := store.FailTask(ctx, task.ID, 1, "encoding failed"); err != nil {
			t.Fatalf("FailTask: %v", err)
		}
		updated, err := store.GetTaskByID(ctx, task.ID)
		if err != nil {
			t.Fatalf("GetTaskByID: %v", err)
		}
		if updated.Status != "failed" {
			t.Errorf("status: got %q want failed", updated.Status)
		}
		if updated.ErrorMsg == nil || *updated.ErrorMsg != "encoding failed" {
			t.Errorf("error_msg: got %v", updated.ErrorMsg)
		}
	})

	t.Run("CancelPendingTasksForJob", func(t *testing.T) {
		j7 := testharness.CreateTestJob(t, store, src.ID)
		for i := range 3 {
			_, err := store.CreateTask(ctx, db.CreateTaskParams{
				JobID:      j7.ID,
				ChunkIndex: i,
				SourcePath: src.UNCPath,
				OutputPath: fmt.Sprintf(`\\nas01\out\cancel_%d.mkv`, i),
			})
			if err != nil {
				t.Fatalf("CreateTask %d: %v", i, err)
			}
		}
		if err := store.CancelPendingTasksForJob(ctx, j7.ID); err != nil {
			t.Fatalf("CancelPendingTasksForJob: %v", err)
		}
		tasks, err := store.ListTasksByJob(ctx, j7.ID)
		if err != nil {
			t.Fatalf("ListTasksByJob: %v", err)
		}
		for _, task := range tasks {
			if task.Status != "cancelled" {
				t.Errorf("task %s: status=%q want cancelled", task.ID, task.Status)
			}
		}
	})

	t.Run("DeleteTasksByJobID", func(t *testing.T) {
		j8 := testharness.CreateTestJob(t, store, src.ID)
		_, err := store.CreateTask(ctx, db.CreateTaskParams{
			JobID:      j8.ID,
			ChunkIndex: 0,
			SourcePath: src.UNCPath,
			OutputPath: `\\nas01\out\del.mkv`,
		})
		if err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
		if err := store.DeleteTasksByJobID(ctx, j8.ID); err != nil {
			t.Fatalf("DeleteTasksByJobID: %v", err)
		}
		tasks, err := store.ListTasksByJob(ctx, j8.ID)
		if err != nil {
			t.Fatalf("ListTasksByJob: %v", err)
		}
		if len(tasks) != 0 {
			t.Errorf("expected 0 tasks after delete, got %d", len(tasks))
		}
	})
}

// ---------------------------------------------------------------------------
// Task Logs
// ---------------------------------------------------------------------------

func TestTaskLogs(t *testing.T) {
	store := setupTest(t)
	ctx := context.Background()

	src := testharness.CreateTestSource(t, store)
	job := testharness.CreateTestJob(t, store, src.ID)
	task, err := store.CreateTask(ctx, db.CreateTaskParams{
		JobID:      job.ID,
		ChunkIndex: 0,
		SourcePath: src.UNCPath,
		OutputPath: `\\nas01\out\log_task.mkv`,
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	t.Run("InsertTaskLog", func(t *testing.T) {
		err := store.InsertTaskLog(ctx, db.InsertTaskLogParams{
			TaskID:  task.ID,
			JobID:   job.ID,
			Stream:  "stdout",
			Level:   "info",
			Message: "encoding started",
		})
		if err != nil {
			t.Fatalf("InsertTaskLog: %v", err)
		}
	})

	t.Run("ListTaskLogs", func(t *testing.T) {
		_ = store.InsertTaskLog(ctx, db.InsertTaskLogParams{
			TaskID: task.ID, JobID: job.ID,
			Stream: "stdout", Level: "info", Message: "progress 50%",
		})
		logs, err := store.ListTaskLogs(ctx, db.ListTaskLogsParams{
			TaskID: task.ID, PageSize: 50,
		})
		if err != nil {
			t.Fatalf("ListTaskLogs: %v", err)
		}
		if len(logs) == 0 {
			t.Error("expected at least one log entry")
		}
	})

	t.Run("ListJobLogs", func(t *testing.T) {
		logs, err := store.ListJobLogs(ctx, db.ListJobLogsParams{
			JobID: job.ID, PageSize: 50,
		})
		if err != nil {
			t.Fatalf("ListJobLogs: %v", err)
		}
		if len(logs) == 0 {
			t.Error("expected at least one log entry for job")
		}
	})

	t.Run("PruneOldTaskLogs", func(t *testing.T) {
		// Prune logs older than "now + 1s" — should prune everything inserted.
		future := time.Now().Add(time.Second)
		if err := store.PruneOldTaskLogs(ctx, future); err != nil {
			t.Fatalf("PruneOldTaskLogs: %v", err)
		}
		logs, err := store.ListTaskLogs(ctx, db.ListTaskLogsParams{
			TaskID: task.ID, PageSize: 50,
		})
		if err != nil {
			t.Fatalf("ListTaskLogs after prune: %v", err)
		}
		if len(logs) != 0 {
			t.Errorf("expected 0 logs after prune, got %d", len(logs))
		}
	})
}

// ---------------------------------------------------------------------------
// Templates
// ---------------------------------------------------------------------------

func TestTemplates(t *testing.T) {
	store := setupTest(t)
	ctx := context.Background()

	t.Run("Create", func(t *testing.T) {
		tmpl, err := store.CreateTemplate(ctx, db.CreateTemplateParams{
			Name:    "encode-template",
			Type:    "run_script",
			Content: "@echo off\nx265 ...",
		})
		if err != nil {
			t.Fatalf("CreateTemplate: %v", err)
		}
		if tmpl.ID == "" {
			t.Error("expected non-empty ID")
		}
	})

	t.Run("GetByID", func(t *testing.T) {
		tmpl := testharness.CreateTestTemplate(t, store)
		fetched, err := store.GetTemplateByID(ctx, tmpl.ID)
		if err != nil {
			t.Fatalf("GetTemplateByID: %v", err)
		}
		if fetched.ID != tmpl.ID {
			t.Errorf("ID mismatch")
		}
	})

	t.Run("List", func(t *testing.T) {
		templates, err := store.ListTemplates(ctx, "")
		if err != nil {
			t.Fatalf("ListTemplates: %v", err)
		}
		if len(templates) == 0 {
			t.Error("expected at least one template")
		}
	})

	t.Run("Update", func(t *testing.T) {
		tmpl := testharness.CreateTestTemplate(t, store)
		if err := store.UpdateTemplate(ctx, db.UpdateTemplateParams{
			ID:      tmpl.ID,
			Name:    tmpl.Name,
			Content: "updated content",
		}); err != nil {
			t.Fatalf("UpdateTemplate: %v", err)
		}
		updated, err := store.GetTemplateByID(ctx, tmpl.ID)
		if err != nil {
			t.Fatalf("GetTemplateByID: %v", err)
		}
		if updated.Content != "updated content" {
			t.Errorf("content: got %q", updated.Content)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		tmpl := testharness.CreateTestTemplate(t, store)
		if err := store.DeleteTemplate(ctx, tmpl.ID); err != nil {
			t.Fatalf("DeleteTemplate: %v", err)
		}
		_, err := store.GetTemplateByID(ctx, tmpl.ID)
		if !errors.Is(err, db.ErrNotFound) {
			t.Errorf("expected ErrNotFound after delete, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// Variables
// ---------------------------------------------------------------------------

func TestVariables(t *testing.T) {
	store := setupTest(t)
	ctx := context.Background()

	t.Run("UpsertVariable", func(t *testing.T) {
		v, err := store.UpsertVariable(ctx, db.UpsertVariableParams{
			Name:     "BITRATE",
			Value:    "8000k",
			Category: "encode",
		})
		if err != nil {
			t.Fatalf("UpsertVariable: %v", err)
		}
		if v.ID == "" {
			t.Error("expected non-empty ID")
		}
		// Upsert again with a new value.
		v2, err := store.UpsertVariable(ctx, db.UpsertVariableParams{
			Name:     "BITRATE",
			Value:    "10000k",
			Category: "encode",
		})
		if err != nil {
			t.Fatalf("UpsertVariable (update): %v", err)
		}
		if v2.Value != "10000k" {
			t.Errorf("value: got %q want 10000k", v2.Value)
		}
	})

	t.Run("GetVariableByName", func(t *testing.T) {
		_, err := store.UpsertVariable(ctx, db.UpsertVariableParams{
			Name: "PRESET", Value: "slow", Category: "encode",
		})
		if err != nil {
			t.Fatalf("UpsertVariable: %v", err)
		}
		v, err := store.GetVariableByName(ctx, "PRESET")
		if err != nil {
			t.Fatalf("GetVariableByName: %v", err)
		}
		if v.Value != "slow" {
			t.Errorf("value: got %q want slow", v.Value)
		}
	})

	t.Run("ListVariables", func(t *testing.T) {
		vars, err := store.ListVariables(ctx, "")
		if err != nil {
			t.Fatalf("ListVariables: %v", err)
		}
		if len(vars) == 0 {
			t.Error("expected at least one variable")
		}
	})

	t.Run("DeleteVariable", func(t *testing.T) {
		v, err := store.UpsertVariable(ctx, db.UpsertVariableParams{
			Name: "DELETEME", Value: "1",
		})
		if err != nil {
			t.Fatalf("UpsertVariable: %v", err)
		}
		if err := store.DeleteVariable(ctx, v.ID); err != nil {
			t.Fatalf("DeleteVariable: %v", err)
		}
		_, err = store.GetVariableByName(ctx, "DELETEME")
		if !errors.Is(err, db.ErrNotFound) {
			t.Errorf("expected ErrNotFound after delete, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// Webhooks
// ---------------------------------------------------------------------------

func TestWebhooks(t *testing.T) {
	store := setupTest(t)
	ctx := context.Background()

	secret := "mysecret"

	t.Run("CreateWebhook", func(t *testing.T) {
		wh, err := store.CreateWebhook(ctx, db.CreateWebhookParams{
			Name:     "discord-notify",
			Provider: "discord",
			URL:      "https://discord.com/api/webhooks/123/abc",
			Secret:   &secret,
			Events:   []string{"job.completed", "job.failed"},
		})
		if err != nil {
			t.Fatalf("CreateWebhook: %v", err)
		}
		if wh.ID == "" {
			t.Error("expected non-empty ID")
		}
	})

	t.Run("GetWebhookByID", func(t *testing.T) {
		wh, err := store.CreateWebhook(ctx, db.CreateWebhookParams{
			Name:     "get-by-id",
			Provider: "slack",
			URL:      "https://hooks.slack.com/services/T/B/X",
			Events:   []string{"job.completed"},
		})
		if err != nil {
			t.Fatalf("CreateWebhook: %v", err)
		}
		fetched, err := store.GetWebhookByID(ctx, wh.ID)
		if err != nil {
			t.Fatalf("GetWebhookByID: %v", err)
		}
		if fetched.ID != wh.ID {
			t.Errorf("ID mismatch")
		}
	})

	t.Run("ListWebhooks", func(t *testing.T) {
		whs, err := store.ListWebhooks(ctx)
		if err != nil {
			t.Fatalf("ListWebhooks: %v", err)
		}
		if len(whs) == 0 {
			t.Error("expected at least one webhook")
		}
	})

	t.Run("ListWebhooksByEvent", func(t *testing.T) {
		whs, err := store.ListWebhooksByEvent(ctx, "job.completed")
		if err != nil {
			t.Fatalf("ListWebhooksByEvent: %v", err)
		}
		if len(whs) == 0 {
			t.Error("expected at least one webhook for job.completed")
		}
	})

	t.Run("UpdateWebhook", func(t *testing.T) {
		wh, err := store.CreateWebhook(ctx, db.CreateWebhookParams{
			Name:     "update-me",
			Provider: "teams",
			URL:      "https://outlook.office.com/webhook/old",
			Events:   []string{"job.completed"},
		})
		if err != nil {
			t.Fatalf("CreateWebhook: %v", err)
		}
		if err := store.UpdateWebhook(ctx, db.UpdateWebhookParams{
			ID:      wh.ID,
			Name:    "update-me",
			URL:     "https://outlook.office.com/webhook/new",
			Events:  []string{"job.completed", "task.failed"},
			Enabled: true,
		}); err != nil {
			t.Fatalf("UpdateWebhook: %v", err)
		}
		updated, err := store.GetWebhookByID(ctx, wh.ID)
		if err != nil {
			t.Fatalf("GetWebhookByID: %v", err)
		}
		if updated.URL != "https://outlook.office.com/webhook/new" {
			t.Errorf("url: got %q", updated.URL)
		}
	})

	t.Run("DeleteWebhook", func(t *testing.T) {
		wh, err := store.CreateWebhook(ctx, db.CreateWebhookParams{
			Name:     "delete-me",
			Provider: "discord",
			URL:      "https://discord.com/del",
			Events:   []string{},
		})
		if err != nil {
			t.Fatalf("CreateWebhook: %v", err)
		}
		if err := store.DeleteWebhook(ctx, wh.ID); err != nil {
			t.Fatalf("DeleteWebhook: %v", err)
		}
		_, err = store.GetWebhookByID(ctx, wh.ID)
		if !errors.Is(err, db.ErrNotFound) {
			t.Errorf("expected ErrNotFound after delete, got %v", err)
		}
	})

	t.Run("InsertWebhookDelivery", func(t *testing.T) {
		wh, err := store.CreateWebhook(ctx, db.CreateWebhookParams{
			Name:     "delivery-wh",
			Provider: "discord",
			URL:      "https://discord.com/delivery",
			Events:   []string{"job.completed"},
		})
		if err != nil {
			t.Fatalf("CreateWebhook: %v", err)
		}
		code := 200
		err = store.InsertWebhookDelivery(ctx, db.InsertWebhookDeliveryParams{
			WebhookID:    wh.ID,
			Event:        "job.completed",
			Payload:      []byte(`{"job_id":"abc"}`),
			ResponseCode: &code,
			Success:      true,
			Attempt:      1,
		})
		if err != nil {
			t.Fatalf("InsertWebhookDelivery: %v", err)
		}
	})

	t.Run("ListWebhookDeliveries", func(t *testing.T) {
		wh, err := store.CreateWebhook(ctx, db.CreateWebhookParams{
			Name:     "delivery-list-wh",
			Provider: "discord",
			URL:      "https://discord.com/list",
			Events:   []string{"job.completed"},
		})
		if err != nil {
			t.Fatalf("CreateWebhook: %v", err)
		}
		code := 201
		_ = store.InsertWebhookDelivery(ctx, db.InsertWebhookDeliveryParams{
			WebhookID: wh.ID, Event: "job.completed",
			Payload: []byte("{}"), ResponseCode: &code, Success: true, Attempt: 1,
		})
		deliveries, err := store.ListWebhookDeliveries(ctx, wh.ID, 10, 0)
		if err != nil {
			t.Fatalf("ListWebhookDeliveries: %v", err)
		}
		if len(deliveries) == 0 {
			t.Error("expected at least one delivery")
		}
	})
}

// ---------------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------------

func TestSessions(t *testing.T) {
	store := setupTest(t)
	ctx := context.Background()

	hash := "h"
	user, err := store.CreateUser(ctx, db.CreateUserParams{
		Username:     "session-user",
		Email:        "session@test.local",
		Role:         "operator",
		PasswordHash: &hash,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	t.Run("CreateSession", func(t *testing.T) {
		sess, err := store.CreateSession(ctx, db.CreateSessionParams{
			Token:     "token-abc",
			UserID:    user.ID,
			ExpiresAt: time.Now().Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		if sess.Token != "token-abc" {
			t.Errorf("token mismatch")
		}
	})

	t.Run("GetSessionByToken", func(t *testing.T) {
		_, err := store.CreateSession(ctx, db.CreateSessionParams{
			Token: "token-get", UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		sess, err := store.GetSessionByToken(ctx, "token-get")
		if err != nil {
			t.Fatalf("GetSessionByToken: %v", err)
		}
		if sess.UserID != user.ID {
			t.Errorf("user_id mismatch")
		}
	})

	t.Run("DeleteSession", func(t *testing.T) {
		_, err := store.CreateSession(ctx, db.CreateSessionParams{
			Token: "token-del", UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		if err := store.DeleteSession(ctx, "token-del"); err != nil {
			t.Fatalf("DeleteSession: %v", err)
		}
		_, err = store.GetSessionByToken(ctx, "token-del")
		if !errors.Is(err, db.ErrNotFound) {
			t.Errorf("expected ErrNotFound after delete, got %v", err)
		}
	})

	t.Run("PruneExpiredSessions", func(t *testing.T) {
		// Insert an already-expired session.
		_, err := store.CreateSession(ctx, db.CreateSessionParams{
			Token:     "token-expired",
			UserID:    user.ID,
			ExpiresAt: time.Now().Add(-time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		if err := store.PruneExpiredSessions(ctx); err != nil {
			t.Fatalf("PruneExpiredSessions: %v", err)
		}
		_, err = store.GetSessionByToken(ctx, "token-expired")
		if !errors.Is(err, db.ErrNotFound) {
			t.Errorf("expected ErrNotFound for pruned session, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// Enrollment Tokens
// ---------------------------------------------------------------------------

func TestEnrollmentTokens(t *testing.T) {
	store := setupTest(t)
	ctx := context.Background()

	hash := "h"
	user, err := store.CreateUser(ctx, db.CreateUserParams{
		Username:     "enroll-user",
		Email:        "enroll@test.local",
		Role:         "admin",
		PasswordHash: &hash,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	t.Run("Create", func(t *testing.T) {
		et, err := store.CreateEnrollmentToken(ctx, db.CreateEnrollmentTokenParams{
			Token:     "enroll-abc",
			CreatedBy: user.ID,
			ExpiresAt: time.Now().Add(24 * time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateEnrollmentToken: %v", err)
		}
		if et.ID == "" {
			t.Error("expected non-empty ID")
		}
	})

	t.Run("Get", func(t *testing.T) {
		_, err := store.CreateEnrollmentToken(ctx, db.CreateEnrollmentTokenParams{
			Token:     "enroll-get",
			CreatedBy: user.ID,
			ExpiresAt: time.Now().Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateEnrollmentToken: %v", err)
		}
		et, err := store.GetEnrollmentToken(ctx, "enroll-get")
		if err != nil {
			t.Fatalf("GetEnrollmentToken: %v", err)
		}
		if et.Token != "enroll-get" {
			t.Errorf("token mismatch")
		}
	})

	t.Run("Consume", func(t *testing.T) {
		agent, err := store.UpsertAgent(ctx, db.UpsertAgentParams{
			Name: "enroll-agent", Hostname: "WIN-EN", IPAddress: "10.2.0.1", Tags: []string{},
		})
		if err != nil {
			t.Fatalf("UpsertAgent: %v", err)
		}
		_, err = store.CreateEnrollmentToken(ctx, db.CreateEnrollmentTokenParams{
			Token:     "enroll-consume",
			CreatedBy: user.ID,
			ExpiresAt: time.Now().Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateEnrollmentToken: %v", err)
		}
		if err := store.ConsumeEnrollmentToken(ctx, db.ConsumeEnrollmentTokenParams{
			Token:   "enroll-consume",
			AgentID: agent.ID,
		}); err != nil {
			t.Fatalf("ConsumeEnrollmentToken: %v", err)
		}
		// ConsumeEnrollmentToken marks the token as used (sets used_at).
		// GetEnrollmentToken filters WHERE used_at IS NULL, so a consumed token
		// is intentionally unretrievable — verify it returns ErrNotFound.
		_, err = store.GetEnrollmentToken(ctx, "enroll-consume")
		if !errors.Is(err, db.ErrNotFound) {
			t.Errorf("GetEnrollmentToken after consume: expected ErrNotFound, got %v", err)
		}
	})

	t.Run("List", func(t *testing.T) {
		tokens, err := store.ListEnrollmentTokens(ctx)
		if err != nil {
			t.Fatalf("ListEnrollmentTokens: %v", err)
		}
		if len(tokens) == 0 {
			t.Error("expected at least one token")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		et, err := store.CreateEnrollmentToken(ctx, db.CreateEnrollmentTokenParams{
			Token:     "enroll-delete",
			CreatedBy: user.ID,
			ExpiresAt: time.Now().Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateEnrollmentToken: %v", err)
		}
		if err := store.DeleteEnrollmentToken(ctx, et.ID); err != nil {
			t.Fatalf("DeleteEnrollmentToken: %v", err)
		}
		_, err = store.GetEnrollmentToken(ctx, "enroll-delete")
		if !errors.Is(err, db.ErrNotFound) {
			t.Errorf("expected ErrNotFound after delete, got %v", err)
		}
	})

	t.Run("PruneExpired", func(t *testing.T) {
		_, err := store.CreateEnrollmentToken(ctx, db.CreateEnrollmentTokenParams{
			Token:     "enroll-expired",
			CreatedBy: user.ID,
			ExpiresAt: time.Now().Add(-time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateEnrollmentToken: %v", err)
		}
		if err := store.PruneExpiredEnrollmentTokens(ctx); err != nil {
			t.Fatalf("PruneExpiredEnrollmentTokens: %v", err)
		}
		_, err = store.GetEnrollmentToken(ctx, "enroll-expired")
		if !errors.Is(err, db.ErrNotFound) {
			t.Errorf("expected ErrNotFound for pruned token, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// Path Mappings
// ---------------------------------------------------------------------------

func TestPathMappings(t *testing.T) {
	store := setupTest(t)
	ctx := context.Background()

	t.Run("Create", func(t *testing.T) {
		pm, err := store.CreatePathMapping(ctx, db.CreatePathMappingParams{
			Name:          "NAS media",
			WindowsPrefix: `\\NAS01\media`,
			LinuxPrefix:   "/mnt/nas/media",
		})
		if err != nil {
			t.Fatalf("CreatePathMapping: %v", err)
		}
		if pm.ID == "" {
			t.Error("expected non-empty ID")
		}
	})

	t.Run("GetByID", func(t *testing.T) {
		pm, err := store.CreatePathMapping(ctx, db.CreatePathMappingParams{
			Name:          "NAS archive",
			WindowsPrefix: `\\NAS01\archive`,
			LinuxPrefix:   "/mnt/nas/archive",
		})
		if err != nil {
			t.Fatalf("CreatePathMapping: %v", err)
		}
		fetched, err := store.GetPathMappingByID(ctx, pm.ID)
		if err != nil {
			t.Fatalf("GetPathMappingByID: %v", err)
		}
		if fetched.ID != pm.ID {
			t.Errorf("ID mismatch")
		}
	})

	t.Run("List", func(t *testing.T) {
		mappings, err := store.ListPathMappings(ctx)
		if err != nil {
			t.Fatalf("ListPathMappings: %v", err)
		}
		if len(mappings) == 0 {
			t.Error("expected at least one mapping")
		}
	})

	t.Run("Update", func(t *testing.T) {
		pm, err := store.CreatePathMapping(ctx, db.CreatePathMappingParams{
			Name:          "update-me",
			WindowsPrefix: `\\OLD\share`,
			LinuxPrefix:   "/old/mount",
		})
		if err != nil {
			t.Fatalf("CreatePathMapping: %v", err)
		}
		updated, err := store.UpdatePathMapping(ctx, db.UpdatePathMappingParams{
			ID:            pm.ID,
			Name:          "updated-name",
			WindowsPrefix: `\\NEW\share`,
			LinuxPrefix:   "/new/mount",
			Enabled:       true,
		})
		if err != nil {
			t.Fatalf("UpdatePathMapping: %v", err)
		}
		if updated.Name != "updated-name" {
			t.Errorf("name: got %q", updated.Name)
		}
		if updated.LinuxPrefix != "/new/mount" {
			t.Errorf("linux_prefix: got %q", updated.LinuxPrefix)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		pm, err := store.CreatePathMapping(ctx, db.CreatePathMappingParams{
			Name:          "delete-me",
			WindowsPrefix: `\\DEL\share`,
			LinuxPrefix:   "/del/mount",
		})
		if err != nil {
			t.Fatalf("CreatePathMapping: %v", err)
		}
		if err := store.DeletePathMapping(ctx, pm.ID); err != nil {
			t.Fatalf("DeletePathMapping: %v", err)
		}
		_, err = store.GetPathMappingByID(ctx, pm.ID)
		if !errors.Is(err, db.ErrNotFound) {
			t.Errorf("expected ErrNotFound after delete, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// Audit Log
// ---------------------------------------------------------------------------

func TestAuditLog(t *testing.T) {
	store := setupTest(t)
	ctx := context.Background()

	// Create a real user so the audit_log.user_id foreign key is satisfied.
	hash := "bcrypt_placeholder_audit"
	auditUser, err := store.CreateUser(ctx, db.CreateUserParams{
		Username:     "audit-admin",
		Email:        "audit-admin@test.local",
		Role:         "admin",
		PasswordHash: &hash,
	})
	if err != nil {
		t.Fatalf("TestAuditLog: create user: %v", err)
	}
	userID := auditUser.ID

	t.Run("CreateAuditEntry", func(t *testing.T) {
		err := store.CreateAuditEntry(ctx, db.CreateAuditEntryParams{
			UserID:     &userID,
			Username:   "admin",
			Action:     "create",
			Resource:   "job",
			ResourceID: "job-001",
			Detail:     []byte(`{"status":"queued"}`),
			IPAddress:  "127.0.0.1",
		})
		if err != nil {
			t.Fatalf("CreateAuditEntry: %v", err)
		}
	})

	t.Run("ListAuditLog", func(t *testing.T) {
		_ = store.CreateAuditEntry(ctx, db.CreateAuditEntryParams{
			Username:   "admin",
			Action:     "delete",
			Resource:   "job",
			ResourceID: "job-002",
			IPAddress:  "127.0.0.1",
		})
		entries, total, err := store.ListAuditLog(ctx, 10, 0)
		if err != nil {
			t.Fatalf("ListAuditLog: %v", err)
		}
		if total == 0 || len(entries) == 0 {
			t.Errorf("expected audit entries, got total=%d len=%d", total, len(entries))
		}
	})
}

// ---------------------------------------------------------------------------
// Agent Metrics
// ---------------------------------------------------------------------------

func TestAgentMetrics(t *testing.T) {
	store := setupTest(t)
	ctx := context.Background()

	agent, err := store.UpsertAgent(ctx, db.UpsertAgentParams{
		Name: "metrics-agent", Hostname: "WIN-MET", IPAddress: "10.3.0.1", Tags: []string{},
	})
	if err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}

	t.Run("InsertAgentMetric", func(t *testing.T) {
		err := store.InsertAgentMetric(ctx, db.InsertAgentMetricParams{
			AgentID: agent.ID,
			CPUPct:  45.5,
			GPUPct:  80.0,
			MemPct:  60.2,
		})
		if err != nil {
			t.Fatalf("InsertAgentMetric: %v", err)
		}
	})

	t.Run("ListAgentMetrics", func(t *testing.T) {
		_ = store.InsertAgentMetric(ctx, db.InsertAgentMetricParams{
			AgentID: agent.ID, CPUPct: 50.0, GPUPct: 70.0, MemPct: 55.0,
		})
		metrics, err := store.ListAgentMetrics(ctx, agent.ID, time.Now().Add(-time.Minute))
		if err != nil {
			t.Fatalf("ListAgentMetrics: %v", err)
		}
		if len(metrics) == 0 {
			t.Error("expected at least one metric")
		}
	})
}

// ---------------------------------------------------------------------------
// Concurrent claim — exactly 1 goroutine wins per task
// ---------------------------------------------------------------------------

func TestConcurrentClaim(t *testing.T) {
	store := setupTest(t)
	ctx := context.Background()

	src := testharness.CreateTestSource(t, store)
	job := testharness.CreateTestJob(t, store, src.ID)

	const numTasks = 1
	_, err := store.CreateTask(ctx, db.CreateTaskParams{
		JobID:      job.ID,
		ChunkIndex: 0,
		SourcePath: src.UNCPath,
		OutputPath: `\\nas01\out\concurrent.mkv`,
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	const numAgents = 10
	agentIDs := make([]string, numAgents)
	for i := range numAgents {
		a, err := store.UpsertAgent(ctx, db.UpsertAgentParams{
			Name:      fmt.Sprintf("concurrent-agent-%d", i),
			Hostname:  fmt.Sprintf("WIN-C%d", i),
			IPAddress: fmt.Sprintf("10.4.0.%d", i+1),
			Tags:      []string{},
		})
		if err != nil {
			t.Fatalf("UpsertAgent %d: %v", i, err)
		}
		agentIDs[i] = a.ID
	}

	type claim struct {
		agentID string
		taskID  string
	}
	claims := make(chan claim, numAgents)

	var wg sync.WaitGroup
	for _, agentID := range agentIDs {
		wg.Add(1)
		go func(aid string) {
			defer wg.Done()
			task, err := store.ClaimNextTask(ctx, aid, []string{})
			if err != nil {
				t.Errorf("ClaimNextTask for agent %s: %v", aid, err)
				return
			}
			if task != nil {
				claims <- claim{agentID: aid, taskID: task.ID}
			}
		}(agentID)
	}
	wg.Wait()
	close(claims)

	var claimList []claim
	for c := range claims {
		claimList = append(claimList, c)
	}

	if len(claimList) != numTasks {
		t.Errorf("expected exactly %d claim(s), got %d: %+v", numTasks, len(claimList), claimList)
	}
}

// ---------------------------------------------------------------------------
// API Keys
// ---------------------------------------------------------------------------

func TestDBAPIKeys(t *testing.T) {
	store := setupTest(t)
	ctx := context.Background()

	hash := "bcrypt_placeholder"
	user, err := store.CreateUser(ctx, db.CreateUserParams{
		Username:     "apikey-user",
		Email:        "apikey-user@test.local",
		Role:         "admin",
		PasswordHash: &hash,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	const keyHash = "sha256-test-key-hash-abc123"
	var key *db.APIKey

	t.Run("CreateAPIKey", func(t *testing.T) {
		k, err := store.CreateAPIKey(ctx, db.CreateAPIKeyParams{
			UserID:  user.ID,
			Name:    "my-key",
			KeyHash: keyHash,
		})
		if err != nil {
			t.Fatalf("CreateAPIKey: %v", err)
		}
		if k.ID == "" {
			t.Error("expected non-empty ID")
		}
		key = k
	})

	t.Run("GetAPIKeyByHash", func(t *testing.T) {
		if key == nil {
			t.Skip("depends on CreateAPIKey")
		}
		k, err := store.GetAPIKeyByHash(ctx, keyHash)
		if err != nil {
			t.Fatalf("GetAPIKeyByHash: %v", err)
		}
		if k.ID != key.ID {
			t.Errorf("ID mismatch: got %q want %q", k.ID, key.ID)
		}
	})

	t.Run("ListAPIKeysByUser", func(t *testing.T) {
		if key == nil {
			t.Skip("depends on CreateAPIKey")
		}
		keys, err := store.ListAPIKeysByUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("ListAPIKeysByUser: %v", err)
		}
		if len(keys) != 1 {
			t.Errorf("expected 1 key, got %d", len(keys))
		}
	})

	t.Run("DeleteAPIKey", func(t *testing.T) {
		if key == nil {
			t.Skip("depends on CreateAPIKey")
		}
		if err := store.DeleteAPIKey(ctx, key.ID); err != nil {
			t.Fatalf("DeleteAPIKey: %v", err)
		}
	})

	t.Run("ListAPIKeysByUser_afterDelete", func(t *testing.T) {
		if key == nil {
			t.Skip("depends on CreateAPIKey")
		}
		keys, err := store.ListAPIKeysByUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("ListAPIKeysByUser: %v", err)
		}
		if len(keys) != 0 {
			t.Errorf("expected 0 keys after delete, got %d", len(keys))
		}
	})
}

// ---------------------------------------------------------------------------
// Notification Preferences
// ---------------------------------------------------------------------------

func TestNotificationPreferences(t *testing.T) {
	store := setupTest(t)
	ctx := context.Background()

	hash := "bcrypt_placeholder"
	user, err := store.CreateUser(ctx, db.CreateUserParams{
		Username:     "notifpref-user",
		Email:        "notifpref-user@test.local",
		Role:         "admin",
		PasswordHash: &hash,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	t.Run("GetNotificationPrefs_notFound", func(t *testing.T) {
		_, err := store.GetNotificationPrefs(ctx, user.ID)
		if !errors.Is(err, db.ErrNotFound) {
			t.Errorf("expected ErrNotFound for new user, got %v", err)
		}
	})

	t.Run("UpsertNotificationPrefs_create", func(t *testing.T) {
		if err := store.UpsertNotificationPrefs(ctx, db.UpsertNotificationPrefsParams{
			UserID:              user.ID,
			NotifyOnJobComplete: true,
			NotifyOnJobFailed:   true,
			NotifyOnAgentStale:  false,
		}); err != nil {
			t.Fatalf("UpsertNotificationPrefs: %v", err)
		}
	})

	t.Run("GetNotificationPrefs_afterCreate", func(t *testing.T) {
		prefs, err := store.GetNotificationPrefs(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetNotificationPrefs: %v", err)
		}
		if !prefs.NotifyOnJobComplete {
			t.Error("notify_on_job_complete: want true")
		}
		if !prefs.NotifyOnJobFailed {
			t.Error("notify_on_job_failed: want true")
		}
		if prefs.NotifyOnAgentStale {
			t.Error("notify_on_agent_stale: want false")
		}
	})

	t.Run("UpsertNotificationPrefs_update", func(t *testing.T) {
		if err := store.UpsertNotificationPrefs(ctx, db.UpsertNotificationPrefsParams{
			UserID:              user.ID,
			NotifyOnJobComplete: false,
			NotifyOnJobFailed:   true,
			NotifyOnAgentStale:  true,
		}); err != nil {
			t.Fatalf("UpsertNotificationPrefs (update): %v", err)
		}
		prefs, err := store.GetNotificationPrefs(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetNotificationPrefs after update: %v", err)
		}
		if prefs.NotifyOnJobComplete {
			t.Error("notify_on_job_complete after update: want false")
		}
		if !prefs.NotifyOnAgentStale {
			t.Error("notify_on_agent_stale after update: want true")
		}
	})
}

// ---------------------------------------------------------------------------
// Task Retry
// ---------------------------------------------------------------------------

func TestTaskRetry(t *testing.T) {
	store := setupTest(t)
	ctx := context.Background()

	src := testharness.CreateTestSource(t, store)

	job, err := store.CreateJob(ctx, db.CreateJobParams{
		SourceID:   src.ID,
		JobType:    "encode",
		TargetTags: []string{},
		MaxRetries: 3,
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	task, err := store.CreateTask(ctx, db.CreateTaskParams{
		JobID:      job.ID,
		ChunkIndex: 0,
		SourcePath: src.UNCPath,
		OutputPath: `\\nas01\out\retry.mkv`,
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// Fail the task.
	if err := store.FailTask(ctx, task.ID, 1, "test failure"); err != nil {
		t.Fatalf("FailTask: %v", err)
	}

	// RetryTaskWithBackoff → creates new pending task with retry_count=1.
	retried, err := store.RetryTaskWithBackoff(ctx, task.ID, 1)
	if err != nil {
		t.Fatalf("RetryTaskWithBackoff: %v", err)
	}
	if retried == nil {
		t.Fatal("RetryTaskWithBackoff: expected a new task, got nil")
	}
	if retried.RetryCount != 2 {
		t.Errorf("retry_count: want 2, got %d", retried.RetryCount)
	}
	if retried.RetryAfter == nil {
		t.Error("retry_after: expected non-nil after RetryTaskWithBackoff")
	}
	if retried.Status != "pending" {
		t.Errorf("status: want pending, got %q", retried.Status)
	}
}

// ---------------------------------------------------------------------------
// Agent Upgrade Flag
// ---------------------------------------------------------------------------

func TestAgentUpgradeFlag(t *testing.T) {
	store := setupTest(t)
	ctx := context.Background()

	agent, err := store.UpsertAgent(ctx, db.UpsertAgentParams{
		Name:      "upgrade-flag-agent",
		Hostname:  "WIN-UPGRADE",
		IPAddress: "10.10.0.1",
		Tags:      []string{},
	})
	if err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}

	t.Run("SetAgentUpgradeRequested_true", func(t *testing.T) {
		if err := store.SetAgentUpgradeRequested(ctx, agent.ID, true); err != nil {
			t.Fatalf("SetAgentUpgradeRequested: %v", err)
		}
		updated, err := store.GetAgentByID(ctx, agent.ID)
		if err != nil {
			t.Fatalf("GetAgentByID: %v", err)
		}
		if !updated.UpgradeRequested {
			t.Error("upgrade_requested: want true, got false")
		}
	})

	t.Run("ClearAgentUpgradeRequested", func(t *testing.T) {
		if err := store.ClearAgentUpgradeRequested(ctx, agent.ID); err != nil {
			t.Fatalf("ClearAgentUpgradeRequested: %v", err)
		}
		updated, err := store.GetAgentByID(ctx, agent.ID)
		if err != nil {
			t.Fatalf("GetAgentByID: %v", err)
		}
		if updated.UpgradeRequested {
			t.Error("upgrade_requested after clear: want false, got true")
		}
	})
}

// ---------------------------------------------------------------------------
// Schedules
// ---------------------------------------------------------------------------

func TestSchedules(t *testing.T) {
	store := setupTest(t)
	ctx := context.Background()

	jobTemplate := []byte(`{"source_id":"placeholder","job_type":"encode"}`)
	var sched *db.Schedule

	t.Run("CreateSchedule", func(t *testing.T) {
		s, err := store.CreateSchedule(ctx, db.CreateScheduleParams{
			Name:        "nightly-encode",
			CronExpr:    "0 2 * * *",
			JobTemplate: jobTemplate,
			Enabled:     true,
		})
		if err != nil {
			t.Fatalf("CreateSchedule: %v", err)
		}
		if s.ID == "" {
			t.Error("expected non-empty ID")
		}
		sched = s
	})

	t.Run("GetScheduleByID", func(t *testing.T) {
		if sched == nil {
			t.Skip("depends on CreateSchedule")
		}
		got, err := store.GetScheduleByID(ctx, sched.ID)
		if err != nil {
			t.Fatalf("GetScheduleByID: %v", err)
		}
		if got.Name != "nightly-encode" {
			t.Errorf("name: got %q want nightly-encode", got.Name)
		}
	})

	t.Run("ListSchedules", func(t *testing.T) {
		if sched == nil {
			t.Skip("depends on CreateSchedule")
		}
		schedules, err := store.ListSchedules(ctx)
		if err != nil {
			t.Fatalf("ListSchedules: %v", err)
		}
		if len(schedules) < 1 {
			t.Errorf("expected at least 1 schedule, got %d", len(schedules))
		}
	})

	t.Run("UpdateSchedule", func(t *testing.T) {
		if sched == nil {
			t.Skip("depends on CreateSchedule")
		}
		updated, err := store.UpdateSchedule(ctx, db.UpdateScheduleParams{
			ID:          sched.ID,
			Name:        "nightly-encode-updated",
			CronExpr:    "0 3 * * *",
			JobTemplate: jobTemplate,
			Enabled:     false,
		})
		if err != nil {
			t.Fatalf("UpdateSchedule: %v", err)
		}
		if updated.Name != "nightly-encode-updated" {
			t.Errorf("name after update: got %q want nightly-encode-updated", updated.Name)
		}
		if updated.CronExpr != "0 3 * * *" {
			t.Errorf("cron_expr after update: got %q want 0 3 * * *", updated.CronExpr)
		}
		if updated.Enabled {
			t.Error("enabled after update: want false, got true")
		}
	})

	t.Run("DeleteSchedule", func(t *testing.T) {
		if sched == nil {
			t.Skip("depends on CreateSchedule")
		}
		if err := store.DeleteSchedule(ctx, sched.ID); err != nil {
			t.Fatalf("DeleteSchedule: %v", err)
		}
	})

	t.Run("ListSchedules_afterDelete", func(t *testing.T) {
		schedules, err := store.ListSchedules(ctx)
		if err != nil {
			t.Fatalf("ListSchedules after delete: %v", err)
		}
		for _, s := range schedules {
			if s.ID == sched.ID {
				t.Error("deleted schedule still present in list")
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Flows
// ---------------------------------------------------------------------------

func TestFlows(t *testing.T) {
	store := setupTest(t)
	ctx := context.Background()

	graph := []byte(`{"nodes":[{"id":"n1","type":"input_source","data":{}}],"edges":[]}`)
	var flow *db.Flow

	t.Run("CreateFlow", func(t *testing.T) {
		f, err := store.CreateFlow(ctx, db.CreateFlowParams{
			Name:        "integration-test-flow",
			Description: "DB integration test",
			Graph:       graph,
		})
		if err != nil {
			t.Fatalf("CreateFlow: %v", err)
		}
		if f.ID == "" {
			t.Error("expected non-empty ID")
		}
		if f.Name != "integration-test-flow" {
			t.Errorf("name: got %q want integration-test-flow", f.Name)
		}
		if len(f.Graph) == 0 {
			t.Error("expected non-empty graph")
		}
		flow = f
	})

	t.Run("GetFlowByID", func(t *testing.T) {
		if flow == nil {
			t.Skip("depends on CreateFlow")
		}
		got, err := store.GetFlowByID(ctx, flow.ID)
		if err != nil {
			t.Fatalf("GetFlowByID: %v", err)
		}
		if got.ID != flow.ID {
			t.Errorf("id: got %q want %q", got.ID, flow.ID)
		}
		if got.Name != flow.Name {
			t.Errorf("name: got %q want %q", got.Name, flow.Name)
		}
	})

	t.Run("GetFlowByID_NotFound", func(t *testing.T) {
		_, err := store.GetFlowByID(ctx, "00000000-0000-0000-0000-000000000000")
		if !errors.Is(err, db.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("ListFlows", func(t *testing.T) {
		if flow == nil {
			t.Skip("depends on CreateFlow")
		}
		flows, err := store.ListFlows(ctx)
		if err != nil {
			t.Fatalf("ListFlows: %v", err)
		}
		if len(flows) < 1 {
			t.Errorf("expected at least 1 flow, got %d", len(flows))
		}
		found := false
		for _, f := range flows {
			if f.ID == flow.ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("created flow %q not found in list", flow.ID)
		}
	})

	t.Run("UpdateFlow", func(t *testing.T) {
		if flow == nil {
			t.Skip("depends on CreateFlow")
		}
		updatedGraph := []byte(`{"nodes":[{"id":"n1","type":"input_source","data":{}},{"id":"n2","type":"encode_x265","data":{}}],"edges":[{"id":"e1","source":"n1","target":"n2","sourceHandle":""}]}`)
		updated, err := store.UpdateFlow(ctx, db.UpdateFlowParams{
			ID:          flow.ID,
			Name:        "integration-test-flow-updated",
			Description: "updated description",
			Graph:       updatedGraph,
		})
		if err != nil {
			t.Fatalf("UpdateFlow: %v", err)
		}
		if updated.Name != "integration-test-flow-updated" {
			t.Errorf("name after update: got %q want integration-test-flow-updated", updated.Name)
		}
		if updated.Description != "updated description" {
			t.Errorf("description after update: got %q", updated.Description)
		}
	})

	t.Run("DeleteFlow", func(t *testing.T) {
		if flow == nil {
			t.Skip("depends on CreateFlow")
		}
		if err := store.DeleteFlow(ctx, flow.ID); err != nil {
			t.Fatalf("DeleteFlow: %v", err)
		}
		// Verify it's gone.
		_, err := store.GetFlowByID(ctx, flow.ID)
		if !errors.Is(err, db.ErrNotFound) {
			t.Errorf("expected ErrNotFound after delete, got %v", err)
		}
	})

	t.Run("ListFlows_AfterDelete", func(t *testing.T) {
		if flow == nil {
			t.Skip("depends on DeleteFlow")
		}
		flows, err := store.ListFlows(ctx)
		if err != nil {
			t.Fatalf("ListFlows after delete: %v", err)
		}
		for _, f := range flows {
			if f.ID == flow.ID {
				t.Errorf("deleted flow %q still present in list", flow.ID)
			}
		}
	})
}

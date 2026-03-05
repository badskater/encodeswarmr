package db_test

// Integration tests for ClaimNextTask.
//
// Requires a live PostgreSQL instance.  Set TEST_DATABASE_URL to run:
//
//	TEST_DATABASE_URL=postgres://distencoder:test@localhost:5432/distencoder_test \
//	    go test ./internal/db/... -run TestClaimNextTask -v

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/badskater/distributed-encoder/internal/db"
)

func testDB(t *testing.T) db.Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://distencoder:test@localhost:5432/distencoder_test?sslmode=disable"
	}
	ctx := context.Background()
	store, pool, err := db.New(ctx, dsn)
	if err != nil {
		t.Skipf("skip: cannot connect to test db (%v)", err)
	}
	t.Cleanup(pool.Close)

	// Migrate up (idempotent).
	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store
}

// TestClaimNextTask_NoDuplicates spins up N goroutines all claiming from the
// same pending-task queue and verifies that each task is claimed at most once.
func TestClaimNextTask_NoDuplicates(t *testing.T) {
	store := testDB(t)
	ctx := context.Background()

	// Create a source and a job with several pending tasks.
	const numTasks = 8
	const numAgents = 4

	src, err := store.CreateSource(ctx, db.CreateSourceParams{
		Filename:  "concurrency_test.mkv",
		UNCPath:   `\\testserver\share\concurrency_test.mkv`,
		SizeBytes: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	t.Cleanup(func() { _ = store.DeleteSource(ctx, src.ID) })

	job, err := store.CreateJob(ctx, db.CreateJobParams{
		SourceID:   src.ID,
		JobType:    "encode",
		Priority:   0,
		TargetTags: []string{},
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	for i := range numTasks {
		_, err := store.CreateTask(ctx, db.CreateTaskParams{
			JobID:      job.ID,
			ChunkIndex: i,
			SourcePath: src.UNCPath,
			OutputPath: fmt.Sprintf(`\\out\chunk_%d.mkv`, i),
		})
		if err != nil {
			t.Fatalf("create task %d: %v", i, err)
		}
	}

	// All agents use the empty tag set (matches any job).
	tags := []string{}

	type result struct {
		agentID string
		taskID  string
	}
	results := make(chan result, numTasks*numAgents)

	var wg sync.WaitGroup
	for a := range numAgents {
		wg.Add(1)
		go func(agentID string) {
			defer wg.Done()
			// Each goroutine claims as many tasks as it can until the queue is empty.
			for {
				task, err := store.ClaimNextTask(ctx, agentID, tags)
				if err != nil {
					t.Errorf("agent %s: claim error: %v", agentID, err)
					return
				}
				if task == nil {
					return // no more tasks
				}
				results <- result{agentID: agentID, taskID: task.ID}
			}
		}(fmt.Sprintf("agent-%d", a))
	}
	wg.Wait()
	close(results)

	// Verify: every task assigned at most once.
	seen := make(map[string]string) // taskID → agentID
	for r := range results {
		if prev, dup := seen[r.taskID]; dup {
			t.Errorf("task %s claimed by both %s and %s", r.taskID, prev, r.agentID)
		}
		seen[r.taskID] = r.agentID
	}

	if got := len(seen); got != numTasks {
		t.Errorf("expected %d tasks claimed, got %d", numTasks, got)
	}
}

// TestClaimNextTask_TagFiltering verifies that agents without matching tags
// do not receive tasks whose job requires specific tags.
func TestClaimNextTask_TagFiltering(t *testing.T) {
	store := testDB(t)
	ctx := context.Background()

	src, err := store.CreateSource(ctx, db.CreateSourceParams{
		Filename:  "tag_filter_test.mkv",
		UNCPath:   `\\testserver\share\tag_filter_test.mkv`,
		SizeBytes: 1024,
	})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	t.Cleanup(func() { _ = store.DeleteSource(ctx, src.ID) })

	// Job requires "gpu" tag.
	job, err := store.CreateJob(ctx, db.CreateJobParams{
		SourceID:   src.ID,
		JobType:    "encode",
		Priority:   10,
		TargetTags: []string{"gpu"},
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	_, err = store.CreateTask(ctx, db.CreateTaskParams{
		JobID:      job.ID,
		ChunkIndex: 0,
		SourcePath: src.UNCPath,
		OutputPath: `\\out\chunk_0.mkv`,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Agent without "gpu" tag must not claim the task.
	task, err := store.ClaimNextTask(ctx, "agent-no-gpu", []string{"cpu"})
	if err != nil {
		t.Fatalf("claim (no-gpu): %v", err)
	}
	if task != nil {
		t.Errorf("expected nil task for agent without gpu tag, got task %s", task.ID)
	}

	// Agent with "gpu" tag must claim it.
	task, err = store.ClaimNextTask(ctx, "agent-gpu", []string{"gpu"})
	if err != nil {
		t.Fatalf("claim (gpu): %v", err)
	}
	if task == nil {
		t.Error("expected task for agent with gpu tag, got nil")
	}
}

//go:build integration && stress

package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/badskater/encodeswarmr/internal/db"
	pb "github.com/badskater/encodeswarmr/internal/proto/encoderv1"
	"github.com/badskater/encodeswarmr/tests/integration/testharness"
)

// dialGRPC opens a plaintext gRPC connection to addr and returns a client +
// closer. Callers must call closer() when done.
func dialGRPC(t *testing.T, addr string) (pb.AgentServiceClient, func()) {
	t.Helper()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("stress: grpc dial %s: %v", addr, err)
	}
	return pb.NewAgentServiceClient(conn), func() { _ = conn.Close() }
}

// --------------------------------------------------------------------------
// TestStress_ConcurrentAgentRegistration
// --------------------------------------------------------------------------

// TestStress_ConcurrentAgentRegistration spawns 20 goroutines each
// registering a unique agent via gRPC.  All 20 must succeed without data
// races or panics and ListAgents must return exactly 20 rows.
func TestStress_ConcurrentAgentRegistration(t *testing.T) {
	tc := testharness.StartController(t)

	const numAgents = 20

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		errs    []error
	)

	wg.Add(numAgents)
	for i := range numAgents {
		i := i
		go func() {
			defer wg.Done()

			client, cleanup := dialGRPC(t, tc.GRPCAddr)
			defer cleanup()

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			hostname := fmt.Sprintf("stress-agent-%02d", i)
			resp, err := client.Register(ctx, &pb.AgentInfo{
				Hostname:     hostname,
				IpAddress:    "127.0.0.1",
				AgentVersion: "0.0.1-stress",
				OsVersion:    "Windows Server 2022",
				CpuCount:     4,
				RamMib:       8192,
			})
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("agent %d Register: %w", i, err))
				mu.Unlock()
				return
			}
			if !resp.GetApproved() {
				mu.Lock()
				errs = append(errs, fmt.Errorf("agent %d: expected approved=true", i))
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if len(errs) > 0 {
		for _, e := range errs {
			t.Error(e)
		}
		t.FailNow()
	}

	// ListAgents must return exactly 20 agents.
	agents, err := tc.Store.ListAgents(context.Background())
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != numAgents {
		t.Errorf("ListAgents: want %d, got %d", numAgents, len(agents))
	}
}

// --------------------------------------------------------------------------
// TestStress_MassJobCreation
// --------------------------------------------------------------------------

// TestStress_MassJobCreation creates 100 jobs via the HTTP API in rapid
// succession using 10 concurrent goroutines.  All must return 201 and
// ListJobs must return 100 rows.
func TestStress_MassJobCreation(t *testing.T) {
	tc := testharness.StartController(t)
	_, token := testharness.CreateAdminUser(t, tc.Store, tc.AuthSvc)

	// Create a single source to attach all jobs to.
	src := testharness.CreateTestSource(t, tc.Store)

	const (
		numJobs    = 100
		concurrent = 10
	)

	start := time.Now()

	g, _ := errgroup.WithContext(context.Background())
	sem := make(chan struct{}, concurrent)

	for i := range numJobs {
		i := i
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			client := testharness.AuthenticatedClient(t, tc.HTTPBaseURL, token)
			req, err := http.NewRequest(http.MethodPost, tc.HTTPBaseURL+"/api/v1/jobs",
				jsonBody(t, map[string]any{
					"source_id":    src.ID,
					"job_type":     "analysis",
					"priority":     5,
					"target_tags":  []string{},
				}),
			)
			if err != nil {
				return fmt.Errorf("job %d: build request: %w", i, err)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("job %d: do request: %w", i, err)
			}
			drainClose(resp)
			if resp.StatusCode != http.StatusCreated {
				return fmt.Errorf("job %d: expected 201, got %d", i, resp.StatusCode)
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}

	elapsed := time.Since(start)
	t.Logf("created %d jobs in %s", numJobs, elapsed)

	// Verify count via DB query with large page size (API defaults to 50).
	jobs, total, err := tc.Store.ListJobs(context.Background(), db.ListJobsFilter{PageSize: 200})
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	_ = jobs
	if total < int64(numJobs) {
		t.Errorf("ListJobs total: want >= %d, got %d", numJobs, total)
	}
}

// --------------------------------------------------------------------------
// TestStress_ConcurrentClaimTask
// --------------------------------------------------------------------------

// TestStress_ConcurrentClaimTask registers 10 agents and has them all call
// PollTask simultaneously.  Exactly 1 must receive the task; the other 9
// must get "no task available".  No deadlocks or duplicate claims.
func TestStress_ConcurrentClaimTask(t *testing.T) {
	ctx := context.Background()
	tc := testharness.StartController(t)

	// Create a single source + job with exactly one task.
	src, err := tc.Store.CreateSource(ctx, db.CreateSourceParams{
		Filename:  "stress-claim.mkv",
		UNCPath:   `\\nas\media\stress-claim.mkv`,
		SizeBytes: 1024,
	})
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}

	job, err := tc.Store.CreateJob(ctx, db.CreateJobParams{
		SourceID:   src.ID,
		JobType:    "encode",
		Priority:   5,
		TargetTags: []string{},
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	scriptDir := t.TempDir()
	testharness.WriteTaskScript(t, scriptDir, 0)

	task, err := tc.Store.CreateTask(ctx, db.CreateTaskParams{
		JobID:      job.ID,
		ChunkIndex: 0,
		SourcePath: `\\nas\media\stress-claim.mkv`,
		OutputPath: `\\nas\output\stress-claim.mkv`,
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := tc.Store.SetTaskScriptDir(ctx, task.ID, scriptDir); err != nil {
		t.Fatalf("SetTaskScriptDir: %v", err)
	}
	if err := tc.Store.UpdateJobStatus(ctx, job.ID, "queued"); err != nil {
		t.Fatalf("UpdateJobStatus: %v", err)
	}
	if err := tc.Store.UpdateJobTaskCounts(ctx, job.ID); err != nil {
		t.Fatalf("UpdateJobTaskCounts: %v", err)
	}

	const numAgents = 10

	// Register all agents upfront.
	agentIDs := make([]string, numAgents)
	for i := range numAgents {
		client, cleanup := dialGRPC(t, tc.GRPCAddr)
		regCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		resp, err := client.Register(regCtx, &pb.AgentInfo{
			Hostname:     fmt.Sprintf("claim-agent-%02d", i),
			IpAddress:    "127.0.0.1",
			AgentVersion: "0.0.1-stress",
			OsVersion:    "Windows Server 2022",
			CpuCount:     4,
			RamMib:       8192,
		})
		cancel()
		cleanup()
		if err != nil {
			t.Fatalf("agent %d Register: %v", i, err)
		}
		agentIDs[i] = resp.GetAgentId()
	}

	// All 10 agents poll simultaneously.
	var (
		claimed  int32
		noClaims int32
		wg       sync.WaitGroup
	)

	wg.Add(numAgents)
	for i := range numAgents {
		i := i
		go func() {
			defer wg.Done()

			client, cleanup := dialGRPC(t, tc.GRPCAddr)
			defer cleanup()

			pollCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			assignment, err := client.PollTask(pollCtx, &pb.PollTaskReq{
				AgentId: agentIDs[i],
				Tags:    []string{},
			})
			if err != nil {
				t.Errorf("agent %d PollTask: %v", i, err)
				return
			}
			if assignment.GetHasTask() {
				atomic.AddInt32(&claimed, 1)
			} else {
				atomic.AddInt32(&noClaims, 1)
			}
		}()
	}

	wg.Wait()

	if claimed != 1 {
		t.Errorf("exactly 1 agent should claim the task; got %d", claimed)
	}
	if noClaims != int32(numAgents-1) {
		t.Errorf("expected %d no-task responses; got %d", numAgents-1, noClaims)
	}
}

// --------------------------------------------------------------------------
// TestStress_BulkTaskCompletion
// --------------------------------------------------------------------------

// TestStress_BulkTaskCompletion creates 50 single-task jobs and uses 5
// in-process agents to run them all to completion within 120 s.
func TestStress_BulkTaskCompletion(t *testing.T) {
	ctx := context.Background()
	tc := testharness.StartController(t)

	const numJobs = 50

	jobIDs := make([]string, numJobs)
	for i := range numJobs {
		src, err := tc.Store.CreateSource(ctx, db.CreateSourceParams{
			Filename:  fmt.Sprintf("bulk-%03d.mkv", i),
			UNCPath:   fmt.Sprintf(`\\nas\media\bulk-%03d.mkv`, i),
			SizeBytes: 1024,
		})
		if err != nil {
			t.Fatalf("bulk job %d CreateSource: %v", i, err)
		}

		job, err := tc.Store.CreateJob(ctx, db.CreateJobParams{
			SourceID:   src.ID,
			JobType:    "encode",
			Priority:   5,
			TargetTags: []string{},
		})
		if err != nil {
			t.Fatalf("bulk job %d CreateJob: %v", i, err)
		}
		jobIDs[i] = job.ID

		scriptDir := t.TempDir()
		testharness.WriteTaskScript(t, scriptDir, 0)

		task, err := tc.Store.CreateTask(ctx, db.CreateTaskParams{
			JobID:      job.ID,
			ChunkIndex: 0,
			SourcePath: fmt.Sprintf(`\\nas\media\bulk-%03d.mkv`, i),
			OutputPath: fmt.Sprintf(`\\nas\output\bulk-%03d.mkv`, i),
		})
		if err != nil {
			t.Fatalf("bulk job %d CreateTask: %v", i, err)
		}
		if err := tc.Store.SetTaskScriptDir(ctx, task.ID, scriptDir); err != nil {
			t.Fatalf("bulk job %d SetTaskScriptDir: %v", i, err)
		}
		if err := tc.Store.UpdateJobStatus(ctx, job.ID, "queued"); err != nil {
			t.Fatalf("bulk job %d UpdateJobStatus: %v", i, err)
		}
		if err := tc.Store.UpdateJobTaskCounts(ctx, job.ID); err != nil {
			t.Fatalf("bulk job %d UpdateJobTaskCounts: %v", i, err)
		}
	}

	// Start 5 agents to work through the queue.
	for i := range 5 {
		testharness.StartAgent(t, tc.GRPCAddr, fmt.Sprintf("bulk-agent-%02d", i))
	}

	// Wait for all 50 jobs to reach completed status (120 s timeout).
	testharness.WaitFor(t, 120*time.Second, func() bool {
		for _, jid := range jobIDs {
			job, err := tc.Store.GetJobByID(ctx, jid)
			if err != nil || job.Status != "completed" {
				return false
			}
		}
		return true
	})

	// Verify final state.
	for _, jid := range jobIDs {
		job, err := tc.Store.GetJobByID(ctx, jid)
		if err != nil {
			t.Errorf("GetJobByID %s: %v", jid, err)
			continue
		}
		if job.Status != "completed" {
			t.Errorf("job %s: want completed, got %q", jid, job.Status)
		}
	}
}

// --------------------------------------------------------------------------
// TestStress_RapidHeartbeats
// --------------------------------------------------------------------------

// TestStress_RapidHeartbeats sends 100 heartbeats in rapid succession from a
// single agent.  All must succeed and the agent's last_heartbeat must be
// recent afterwards.
func TestStress_RapidHeartbeats(t *testing.T) {
	ctx := context.Background()
	tc := testharness.StartController(t)

	client, cleanup := dialGRPC(t, tc.GRPCAddr)
	defer cleanup()

	regCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	regResp, err := client.Register(regCtx, &pb.AgentInfo{
		Hostname:     "hb-stress-agent",
		IpAddress:    "127.0.0.1",
		AgentVersion: "0.0.1-stress",
		OsVersion:    "Windows Server 2022",
		CpuCount:     4,
		RamMib:       8192,
	})
	cancel()
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	agentID := regResp.GetAgentId()

	const numHeartbeats = 100

	for i := range numHeartbeats {
		hbCtx, hbCancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := client.Heartbeat(hbCtx, &pb.HeartbeatReq{
			AgentId: agentID,
			State:   pb.AgentState_AGENT_STATE_IDLE,
			Metrics: &pb.AgentMetrics{
				CpuPercent:  float32(i % 100),
				RamPercent:  45.0,
				DiskFreeMib: 204800,
			},
		})
		hbCancel()
		if err != nil {
			t.Fatalf("heartbeat %d: %v", i, err)
		}
	}

	// Verify agent's last_heartbeat is recent.
	agent, err := tc.Store.GetAgentByID(ctx, agentID)
	if err != nil {
		t.Fatalf("GetAgentByID: %v", err)
	}
	if agent.LastHeartbeat == nil {
		t.Fatal("agent last_heartbeat is nil after 100 heartbeats")
	}
	age := time.Since(*agent.LastHeartbeat)
	if age > 30*time.Second {
		t.Errorf("last_heartbeat is %v old; expected < 30s", age)
	}
}

// --------------------------------------------------------------------------
// TestStress_ConcurrentAPIRequests
// --------------------------------------------------------------------------

// TestStress_ConcurrentAPIRequests spawns 50 goroutines each making GET
// /api/v1/agents.  All must return 200 and the server must not panic or
// return 5xx.
func TestStress_ConcurrentAPIRequests(t *testing.T) {
	tc := testharness.StartController(t)
	_, token := testharness.CreateAdminUser(t, tc.Store, tc.AuthSvc)

	const numRequests = 50

	g, _ := errgroup.WithContext(context.Background())

	for i := range numRequests {
		i := i
		g.Go(func() error {
			client := testharness.AuthenticatedClient(t, tc.HTTPBaseURL, token)
			req, err := http.NewRequest(http.MethodGet, tc.HTTPBaseURL+"/api/v1/agents", nil)
			if err != nil {
				return fmt.Errorf("request %d: build: %w", i, err)
			}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("request %d: do: %w", i, err)
			}
			drainClose(resp)
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("request %d: expected 200, got %d", i, resp.StatusCode)
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
}

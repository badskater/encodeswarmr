//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/badskater/distributed-encoder/internal/db"
	pb "github.com/badskater/distributed-encoder/internal/proto/encoderv1"
	"github.com/badskater/distributed-encoder/tests/integration/testharness"
)

// grpcClient dials the controller gRPC address with insecure (plaintext)
// credentials and returns the client and a cleanup func.
func grpcClient(t *testing.T, addr string) (pb.AgentServiceClient, func()) {
	t.Helper()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc dial %s: %v", addr, err)
	}
	return pb.NewAgentServiceClient(conn), func() { _ = conn.Close() }
}

// testAgentInfo returns a minimal AgentInfo for use in test registrations.
func testAgentInfo(hostname string) *pb.AgentInfo {
	return &pb.AgentInfo{
		Hostname:     hostname,
		IpAddress:    "127.0.0.1",
		AgentVersion: "0.0.1-test",
		OsVersion:    "Windows Server 2022",
		CpuCount:     4,
		RamMib:       8192,
	}
}

// --------------------------------------------------------------------------
// TestGRPCRegister
// --------------------------------------------------------------------------

// TestGRPCRegister verifies that a new agent registers successfully and is
// immediately approved (AutoApprove=true).
func TestGRPCRegister(t *testing.T) {
	tc := testharness.StartController(t)

	client, cleanup := grpcClient(t, tc.GRPCAddr)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Register(ctx, testAgentInfo("test-agent-register"))
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !resp.GetApproved() {
		t.Errorf("Register: expected approved=true (auto-approve), got false; message=%q", resp.GetMessage())
	}
	if resp.GetAgentId() == "" {
		t.Error("Register: expected non-empty agent_id")
	}

	// Verify agent exists in DB.
	agent, err := tc.Store.GetAgentByName(context.Background(), "test-agent-register")
	if err != nil {
		t.Fatalf("GetAgentByName: %v", err)
	}
	if agent.Status != "idle" && agent.Status != "approved" {
		t.Errorf("agent status: unexpected %q", agent.Status)
	}
}

// --------------------------------------------------------------------------
// TestGRPCHeartbeat
// --------------------------------------------------------------------------

// TestGRPCHeartbeat registers an agent and then sends a heartbeat with
// metrics, verifying that the heartbeat_at timestamp is updated in the DB.
func TestGRPCHeartbeat(t *testing.T) {
	tc := testharness.StartController(t)

	client, cleanup := grpcClient(t, tc.GRPCAddr)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Register first.
	regResp, err := client.Register(ctx, testAgentInfo("test-agent-heartbeat"))
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	agentID := regResp.GetAgentId()

	// Record current heartbeat time (may be nil for a brand-new agent).
	agentBefore, err := tc.Store.GetAgentByID(context.Background(), agentID)
	if err != nil {
		t.Fatalf("GetAgentByID before heartbeat: %v", err)
	}

	// Small sleep so the updated timestamp is strictly greater.
	time.Sleep(50 * time.Millisecond)

	// Send heartbeat with metrics.
	hbResp, err := client.Heartbeat(ctx, &pb.HeartbeatReq{
		AgentId: agentID,
		State:   pb.AgentState_AGENT_STATE_IDLE,
		Metrics: &pb.AgentMetrics{
			CpuPercent:  12.5,
			RamPercent:  45.0,
			DiskFreeMib: 204800,
		},
	})
	if err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}
	_ = hbResp // response fields are informational

	// Verify heartbeat_at updated in DB.
	agentAfter, err := tc.Store.GetAgentByID(context.Background(), agentID)
	if err != nil {
		t.Fatalf("GetAgentByID after heartbeat: %v", err)
	}

	if agentAfter.LastHeartbeat == nil {
		t.Fatal("heartbeat: last_heartbeat is still nil after Heartbeat RPC")
	}
	if agentBefore.LastHeartbeat != nil && !agentAfter.LastHeartbeat.After(*agentBefore.LastHeartbeat) {
		t.Errorf("heartbeat: last_heartbeat did not advance (before=%v, after=%v)",
			agentBefore.LastHeartbeat, agentAfter.LastHeartbeat)
	}
}

// --------------------------------------------------------------------------
// TestGRPCPollTask
// --------------------------------------------------------------------------

// TestGRPCPollTask creates a source and job directly in the DB, waits for the
// engine to expand the job into tasks, then exercises PollTask to verify a
// TaskAssignment is returned.
func TestGRPCPollTask(t *testing.T) {
	tc := testharness.StartController(t)

	client, cleanup := grpcClient(t, tc.GRPCAddr)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Register an agent so it can claim tasks.
	regResp, err := client.Register(ctx, testAgentInfo("test-agent-polltask"))
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	agentID := regResp.GetAgentId()

	// Create source and job directly in the DB.
	src, err := tc.Store.CreateSource(context.Background(), db.CreateSourceParams{
		Filename:  "polltask_test.mkv",
		UNCPath:   `\\nas01\media\polltask_test.mkv`,
		SizeBytes: 1024 * 1024 * 100,
	})
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}

	// Create a template required by the engine's script generator.
	tmpl := testharness.CreateTestTemplate(t, tc.Store)

	job, err := tc.Store.CreateJob(context.Background(), db.CreateJobParams{
		SourceID:   src.ID,
		JobType:    "encode",
		Priority:   0,
		TargetTags: []string{},
		EncodeConfig: db.EncodeConfig{
			RunScriptTemplateID: tmpl.ID,
			OutputRoot:          `\\nas01\output`,
			OutputExtension:     "mkv",
			ChunkBoundaries: []db.ChunkBoundary{
				{StartFrame: 0, EndFrame: 999},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	// Wait for the engine to expand the job and create tasks (up to 15 s).
	testharness.WaitFor(t, 15*time.Second, func() bool {
		tasks, _ := tc.Store.ListTasksByJob(context.Background(), job.ID)
		return len(tasks) > 0
	})

	// PollTask → expect a TaskAssignment.
	assignment, err := client.PollTask(ctx, &pb.PollTaskReq{
		AgentId: agentID,
		Tags:    []string{},
	})
	if err != nil {
		t.Fatalf("PollTask: %v", err)
	}
	if !assignment.GetHasTask() {
		t.Error("PollTask: expected has_task=true, got false")
	}
	if assignment.GetTaskId() == "" {
		t.Error("PollTask: expected non-empty task_id")
	}
}

// --------------------------------------------------------------------------
// TestGRPCReportResult
// --------------------------------------------------------------------------

// TestGRPCReportResult verifies the full task claim → ReportResult(success)
// lifecycle: after reporting success the task and job should be completed.
func TestGRPCReportResult(t *testing.T) {
	tc := testharness.StartController(t)

	client, cleanup := grpcClient(t, tc.GRPCAddr)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Register agent.
	regResp, err := client.Register(ctx, testAgentInfo("test-agent-result"))
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	agentID := regResp.GetAgentId()

	// Create a source + single-chunk job.
	src, err := tc.Store.CreateSource(context.Background(), db.CreateSourceParams{
		Filename:  "result_test.mkv",
		UNCPath:   `\\nas01\media\result_test.mkv`,
		SizeBytes: 1024 * 1024 * 50,
	})
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}

	// Create a template required by the engine's script generator.
	tmpl := testharness.CreateTestTemplate(t, tc.Store)

	job, err := tc.Store.CreateJob(context.Background(), db.CreateJobParams{
		SourceID:   src.ID,
		JobType:    "encode",
		Priority:   0,
		TargetTags: []string{},
		EncodeConfig: db.EncodeConfig{
			RunScriptTemplateID: tmpl.ID,
			OutputRoot:          `\\nas01\output`,
			OutputExtension:     "mkv",
			ChunkBoundaries: []db.ChunkBoundary{
				{StartFrame: 0, EndFrame: 499},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	// Wait for expansion.
	testharness.WaitFor(t, 15*time.Second, func() bool {
		tasks, _ := tc.Store.ListTasksByJob(context.Background(), job.ID)
		return len(tasks) > 0
	})

	// Poll for the task.
	assignment, err := client.PollTask(ctx, &pb.PollTaskReq{
		AgentId: agentID,
		Tags:    []string{},
	})
	if err != nil {
		t.Fatalf("PollTask: %v", err)
	}
	if !assignment.GetHasTask() {
		t.Fatal("PollTask: expected has_task=true")
	}

	// ReportResult(success).
	ack, err := client.ReportResult(ctx, &pb.TaskResult{
		TaskId:  assignment.GetTaskId(),
		JobId:   assignment.GetJobId(),
		Success: true,
		Metrics: &pb.EncodeMetrics{
			FramesEncoded: 500,
			AvgFps:        24.0,
			OutputSize:    10 * 1024 * 1024,
			DurationSec:   21,
		},
	})
	if err != nil {
		t.Fatalf("ReportResult: %v", err)
	}
	if !ack.GetOk() {
		t.Error("ReportResult: ack.ok=false")
	}

	// Verify task is completed in DB.
	task, err := tc.Store.GetTaskByID(context.Background(), assignment.GetTaskId())
	if err != nil {
		t.Fatalf("GetTaskByID: %v", err)
	}
	if task.Status != "completed" {
		t.Errorf("task status: want completed, got %q", task.Status)
	}

	// Wait for job to reach completed state (engine needs a cycle to finalize).
	testharness.WaitForJobStatus(t, tc.Store, job.ID, "completed", 20*time.Second)
}

// --------------------------------------------------------------------------
// TestGRPCStreamLogs
// --------------------------------------------------------------------------

// TestGRPCStreamLogs registers an agent, creates a task, then streams log
// entries and verifies they appear in the task_logs table.
func TestGRPCStreamLogs(t *testing.T) {
	tc := testharness.StartController(t)

	client, cleanup := grpcClient(t, tc.GRPCAddr)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Register agent.
	regResp, err := client.Register(ctx, testAgentInfo("test-agent-logs"))
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	agentID := regResp.GetAgentId()

	// Create source + job + wait for task creation.
	src, err := tc.Store.CreateSource(context.Background(), db.CreateSourceParams{
		Filename:  "logs_test.mkv",
		UNCPath:   `\\nas01\media\logs_test.mkv`,
		SizeBytes: 1024 * 1024 * 20,
	})
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}

	// Create a template required by the engine's script generator.
	tmpl := testharness.CreateTestTemplate(t, tc.Store)

	job, err := tc.Store.CreateJob(context.Background(), db.CreateJobParams{
		SourceID:   src.ID,
		JobType:    "encode",
		Priority:   0,
		TargetTags: []string{},
		EncodeConfig: db.EncodeConfig{
			RunScriptTemplateID: tmpl.ID,
			OutputRoot:          `\\nas01\output`,
			OutputExtension:     "mkv",
			ChunkBoundaries: []db.ChunkBoundary{
				{StartFrame: 0, EndFrame: 99},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	// Wait for engine to expand the job.
	testharness.WaitFor(t, 15*time.Second, func() bool {
		tasks, _ := tc.Store.ListTasksByJob(context.Background(), job.ID)
		return len(tasks) > 0
	})

	// Poll and claim the task.
	assignment, err := client.PollTask(ctx, &pb.PollTaskReq{AgentId: agentID})
	if err != nil {
		t.Fatalf("PollTask: %v", err)
	}
	if !assignment.GetHasTask() {
		t.Fatal("PollTask: expected has_task=true for log test")
	}
	taskID := assignment.GetTaskId()

	// StreamLogs: open the streaming call and send a few entries.
	stream, err := client.StreamLogs(ctx)
	if err != nil {
		t.Fatalf("StreamLogs open: %v", err)
	}

	logMessages := []string{
		"starting encode",
		"frame 50/100",
		"encode complete",
	}
	for _, msg := range logMessages {
		if sendErr := stream.Send(&pb.LogEntry{
			TaskId:  taskID,
			JobId:   job.ID,
			Stream:  "stdout",
			Level:   "info",
			Message: msg,
		}); sendErr != nil {
			t.Fatalf("StreamLogs send: %v", sendErr)
		}
	}

	// Close the send side and wait for the server ack.
	ack, closeErr := stream.CloseAndRecv()
	if closeErr != nil {
		t.Fatalf("StreamLogs CloseAndRecv: %v", closeErr)
	}
	if !ack.GetOk() {
		t.Error("StreamLogs: ack.ok=false")
	}

	// Verify logs appear in the DB.
	testharness.WaitFor(t, 5*time.Second, func() bool {
		logs, _ := tc.Store.ListTaskLogs(context.Background(), db.ListTaskLogsParams{
			TaskID:   taskID,
			PageSize: 10,
		})
		return len(logs) >= len(logMessages)
	})

	logs, err := tc.Store.ListTaskLogs(context.Background(), db.ListTaskLogsParams{
		TaskID:   taskID,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("ListTaskLogs: %v", err)
	}
	if len(logs) < len(logMessages) {
		t.Errorf("expected at least %d log rows, got %d", len(logMessages), len(logs))
	}
}

// --------------------------------------------------------------------------
// TestGRPCSyncOffline
// --------------------------------------------------------------------------

// TestGRPCSyncOffline simulates an agent that was offline and replays a
// buffered task result via SyncOfflineResults, verifying the task is marked
// completed.
func TestGRPCSyncOffline(t *testing.T) {
	tc := testharness.StartController(t)

	client, cleanup := grpcClient(t, tc.GRPCAddr)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Register agent.
	regResp, err := client.Register(ctx, testAgentInfo("test-agent-offline"))
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	agentID := regResp.GetAgentId()

	// Create source + single-chunk job.
	src, err := tc.Store.CreateSource(context.Background(), db.CreateSourceParams{
		Filename:  "offline_test.mkv",
		UNCPath:   `\\nas01\media\offline_test.mkv`,
		SizeBytes: 1024 * 1024 * 30,
	})
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}

	// Create a template required by the engine's script generator.
	tmpl := testharness.CreateTestTemplate(t, tc.Store)

	job, err := tc.Store.CreateJob(context.Background(), db.CreateJobParams{
		SourceID:   src.ID,
		JobType:    "encode",
		Priority:   0,
		TargetTags: []string{},
		EncodeConfig: db.EncodeConfig{
			RunScriptTemplateID: tmpl.ID,
			OutputRoot:          `\\nas01\output`,
			OutputExtension:     "mkv",
			ChunkBoundaries: []db.ChunkBoundary{
				{StartFrame: 0, EndFrame: 299},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	// Wait for expansion.
	testharness.WaitFor(t, 15*time.Second, func() bool {
		tasks, _ := tc.Store.ListTasksByJob(context.Background(), job.ID)
		return len(tasks) > 0
	})

	// Claim the task via PollTask so it is in "assigned" / "running" state.
	assignment, err := client.PollTask(ctx, &pb.PollTaskReq{AgentId: agentID})
	if err != nil {
		t.Fatalf("PollTask: %v", err)
	}
	if !assignment.GetHasTask() {
		t.Fatal("PollTask: expected has_task=true for offline test")
	}
	taskID := assignment.GetTaskId()

	// SyncOfflineResults — replay the buffered result.
	syncStream, err := client.SyncOfflineResults(ctx)
	if err != nil {
		t.Fatalf("SyncOfflineResults open: %v", err)
	}

	if sendErr := syncStream.Send(&pb.TaskResult{
		TaskId:        taskID,
		JobId:         assignment.GetJobId(),
		Success:       true,
		OfflineResult: true,
		Metrics: &pb.EncodeMetrics{
			FramesEncoded: 300,
			AvgFps:        24.0,
			OutputSize:    5 * 1024 * 1024,
			DurationSec:   12,
		},
	}); sendErr != nil {
		t.Fatalf("SyncOfflineResults send: %v", sendErr)
	}

	syncResp, closeErr := syncStream.CloseAndRecv()
	if closeErr != nil {
		t.Fatalf("SyncOfflineResults CloseAndRecv: %v", closeErr)
	}
	if syncResp.GetAccepted() < 1 {
		t.Errorf("SyncOfflineResults: expected accepted>=1, got %d", syncResp.GetAccepted())
	}

	// Verify task is completed.
	task, err := tc.Store.GetTaskByID(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetTaskByID: %v", err)
	}
	if task.Status != "completed" {
		t.Errorf("task status after SyncOfflineResults: want completed, got %q", task.Status)
	}
}

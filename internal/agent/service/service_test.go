package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	pb "github.com/badskater/distributed-encoder/internal/proto/encoderv1"
)

// ---------------------------------------------------------------------------
// defaultConfigPath
// ---------------------------------------------------------------------------

func TestDefaultConfigPath_NotEmpty(t *testing.T) {
	p := defaultConfigPath()
	if p == "" {
		t.Fatal("defaultConfigPath() returned empty string")
	}
}

func TestDefaultConfigPath_PlatformSpecific(t *testing.T) {
	p := defaultConfigPath()
	if runtime.GOOS == "windows" {
		if !strings.HasPrefix(p, `C:\`) && !strings.HasPrefix(p, `c:\`) {
			t.Errorf("defaultConfigPath() on Windows = %q, expected C:\\ prefix", p)
		}
	} else {
		if !strings.HasPrefix(p, "/") {
			t.Errorf("defaultConfigPath() on %s = %q, expected absolute path", runtime.GOOS, p)
		}
	}
}

// ---------------------------------------------------------------------------
// usageText
// ---------------------------------------------------------------------------

func TestUsageText_NotEmpty(t *testing.T) {
	txt := usageText()
	if txt == "" {
		t.Fatal("usageText() returned empty string")
	}
}

func TestUsageText_ContainsSubcommands(t *testing.T) {
	txt := usageText()
	// Core subcommands that must appear regardless of platform.
	for _, sub := range []string{"run", "install", "setup-vnc"} {
		if !strings.Contains(txt, sub) {
			t.Errorf("usageText() missing subcommand %q", sub)
		}
	}
}

func TestUsageText_ContainsFlagsSection(t *testing.T) {
	txt := usageText()
	if !strings.Contains(txt, "--config") {
		t.Error("usageText() missing --config flag")
	}
	if !strings.Contains(txt, "--debug") {
		t.Error("usageText() missing --debug flag")
	}
}

// ---------------------------------------------------------------------------
// parseArgs
// ---------------------------------------------------------------------------

func TestParseArgs_Defaults(t *testing.T) {
	p := parseArgs([]string{})
	if p.subcommand != "" {
		t.Errorf("subcommand = %q, want empty", p.subcommand)
	}
	if p.configPath == "" {
		t.Error("configPath should have a default, got empty")
	}
	if p.debug {
		t.Error("debug should default to false")
	}
	if p.httpDebug {
		t.Error("httpDebug should default to false")
	}
}

func TestParseArgs_Subcommand(t *testing.T) {
	p := parseArgs([]string{"run"})
	if p.subcommand != "run" {
		t.Errorf("subcommand = %q, want run", p.subcommand)
	}
}

func TestParseArgs_ConfigFlag_Equals(t *testing.T) {
	p := parseArgs([]string{"--config=/custom/agent.yaml"})
	if p.configPath != "/custom/agent.yaml" {
		t.Errorf("configPath = %q, want /custom/agent.yaml", p.configPath)
	}
}

func TestParseArgs_ConfigFlag_Space(t *testing.T) {
	p := parseArgs([]string{"--config", "/custom/agent.yaml"})
	if p.configPath != "/custom/agent.yaml" {
		t.Errorf("configPath = %q, want /custom/agent.yaml", p.configPath)
	}
}

func TestParseArgs_DebugFlag(t *testing.T) {
	p := parseArgs([]string{"--debug"})
	if !p.debug {
		t.Error("debug should be true when --debug is passed")
	}
}

func TestParseArgs_HTTPDebugFlag(t *testing.T) {
	p := parseArgs([]string{"--http-debug"})
	if !p.httpDebug {
		t.Error("httpDebug should be true when --http-debug is passed")
	}
}

func TestParseArgs_AllFlags(t *testing.T) {
	p := parseArgs([]string{"run", "--config=/etc/agent.yaml", "--debug", "--http-debug"})
	if p.subcommand != "run" {
		t.Errorf("subcommand = %q, want run", p.subcommand)
	}
	if p.configPath != "/etc/agent.yaml" {
		t.Errorf("configPath = %q, want /etc/agent.yaml", p.configPath)
	}
	if !p.debug {
		t.Error("debug should be true")
	}
	if !p.httpDebug {
		t.Error("httpDebug should be true")
	}
}

func TestParseArgs_SetupVNCSubcommand(t *testing.T) {
	p := parseArgs([]string{"setup-vnc"})
	if p.subcommand != "setup-vnc" {
		t.Errorf("subcommand = %q, want setup-vnc", p.subcommand)
	}
}

func TestParseArgs_FirstNonFlagIsSubcommand(t *testing.T) {
	p := parseArgs([]string{"--debug", "install"})
	if p.subcommand != "install" {
		t.Errorf("subcommand = %q, want install", p.subcommand)
	}
	if !p.debug {
		t.Error("debug should be true")
	}
}

func TestParseArgs_ConfigMissingValue_NoIndex(t *testing.T) {
	// --config at end of args with no following value: should not panic.
	p := parseArgs([]string{"--config"})
	// configPath falls back to default — or is empty. Either is acceptable.
	_ = p.configPath
}

// ---------------------------------------------------------------------------
// startDebugServer + health/metrics endpoints
// ---------------------------------------------------------------------------

func TestStartDebugServer_HealthEndpoint(t *testing.T) {
	r := &runner{
		cfg:   nil, // health handler does not dereference cfg
		log:   slog.Default(),
		state: pb.AgentState_AGENT_STATE_IDLE,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start on a different port to avoid collisions with other tests.
	// We cannot easily change the port since startDebugServer hard-codes
	// "localhost:9080", so we start the server in a goroutine and give it
	// a moment to come up, then query it.
	started := make(chan struct{})
	go func() {
		close(started)
		startDebugServer(ctx, r, slog.Default())
	}()

	<-started
	// Give the server a moment to bind.
	time.Sleep(80 * time.Millisecond)

	resp, err := http.Get("http://localhost:9080/health")
	if err != nil {
		t.Skipf("debug server not reachable (may be occupied): %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /health status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var h healthResponse
	if err := json.Unmarshal(body, &h); err != nil {
		t.Fatalf("unmarshal health response: %v", err)
	}
	if h.Status != "running" {
		t.Errorf("status = %q, want running", h.Status)
	}
	if h.State != "IDLE" {
		t.Errorf("state = %q, want IDLE", h.State)
	}
}

func TestStartDebugServer_MetricsEndpoint(t *testing.T) {
	r := &runner{
		cfg:   nil,
		log:   slog.Default(),
		state: pb.AgentState_AGENT_STATE_IDLE,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := make(chan struct{})
	go func() {
		close(started)
		startDebugServer(ctx, r, slog.Default())
	}()

	<-started
	time.Sleep(80 * time.Millisecond)

	resp, err := http.Get("http://localhost:9080/metrics")
	if err != nil {
		t.Skipf("debug server not reachable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /metrics status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	if !strings.Contains(text, "agent_state") {
		t.Errorf("metrics missing agent_state gauge; body:\n%s", text)
	}
	if !strings.Contains(text, "agent_uptime_seconds") {
		t.Errorf("metrics missing agent_uptime_seconds gauge; body:\n%s", text)
	}
}

func TestStartDebugServer_ShutdownOnCancel(t *testing.T) {
	r := &runner{
		cfg:   nil,
		log:   slog.Default(),
		state: pb.AgentState_AGENT_STATE_IDLE,
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		startDebugServer(ctx, r, slog.Default())
		close(done)
	}()

	// Give the server a moment to start, then cancel.
	time.Sleep(80 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Server exited cleanly.
	case <-time.After(3 * time.Second):
		t.Error("startDebugServer did not stop within 3s after context cancel")
	}
}

// ---------------------------------------------------------------------------
// progressStreamer.start / run — with mock client
// ---------------------------------------------------------------------------

func TestProgressStreamer_Start_CancelStops(t *testing.T) {
	mockStream := &mockProgressStream{mockClientStream: &mockClientStream{}}
	mock := &mockAgentClient{progressStream: mockStream}

	ps := newProgressStreamer(mock, "t1", "j1", slog.Default(), nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancelPS := ps.start(ctx)

	// Send a progress metric so there is data to flush.
	ps.ch <- &progressMetric{Frame: 100, TotalFrames: 1000, FPS: 25.0, Percent: 10.0}

	// Let the ticker fire at least once (ticker is 5s; we cancel immediately
	// to avoid waiting, but the goroutine should still stop cleanly).
	cancel()
	cancelPS() // cancel the streamer's context and wait for it to stop.
}

func TestProgressStreamer_Start_StreamOpenError(t *testing.T) {
	mock := &mockAgentClient{progressStreamErr: errTestStreamFail}

	ps := newProgressStreamer(mock, "t1", "j1", slog.Default(), nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelPS := ps.start(ctx)

	// Give the goroutine a moment to try opening the stream and exit.
	time.Sleep(50 * time.Millisecond)
	cancelPS()
}

var errTestStreamFail = errors.New("stream open failed")

// ---------------------------------------------------------------------------
// progressStreamer channel behaviour
// ---------------------------------------------------------------------------

func TestProgressStreamer_ChannelBuffered(t *testing.T) {
	mockStream := &mockProgressStream{mockClientStream: &mockClientStream{}}
	mock := &mockAgentClient{progressStream: mockStream}

	ps := newProgressStreamer(mock, "t1", "j1", slog.Default(), nil)

	// The channel has capacity 64; filling it should not block.
	for i := 0; i < 64; i++ {
		select {
		case ps.ch <- &progressMetric{Frame: int64(i)}:
		default:
			t.Fatalf("channel full at i=%d, expected capacity 64", i)
		}
	}
}

func TestNewProgressStreamer_Fields(t *testing.T) {
	mock := &mockAgentClient{}
	ps := newProgressStreamer(mock, "task-1", "job-1", slog.Default(), nil)

	if ps.taskID != "task-1" {
		t.Errorf("taskID = %q, want task-1", ps.taskID)
	}
	if ps.jobID != "job-1" {
		t.Errorf("jobID = %q, want job-1", ps.jobID)
	}
	if ps.ch == nil {
		t.Error("ch should not be nil")
	}
	if cap(ps.ch) != 64 {
		t.Errorf("ch capacity = %d, want 64", cap(ps.ch))
	}
}

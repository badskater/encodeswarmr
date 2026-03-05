package service

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	agentcfg "github.com/badskater/distributed-encoder/internal/agent/config"
	pb "github.com/badskater/distributed-encoder/internal/proto/encoderv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const agentVersion = "0.1.0"

// runner holds the runtime state of the agent service.
type runner struct {
	cfg     *agentcfg.Config
	conn    *grpc.ClientConn
	client  pb.AgentServiceClient
	agentID string
	log     *slog.Logger
	offline *offlineStore

	// primaryGPUVendor is set after detectGPUs() at startup (empty if none found).
	primaryGPUVendor string

	mu            sync.Mutex
	state         pb.AgentState
	currentTaskID string
}

// run is the main lifecycle of the agent. It blocks until ctx is cancelled.
func (r *runner) run(ctx context.Context) error {
	r.log.Info("agent starting", "version", agentVersion)

	// Detect GPUs at startup (best-effort).
	if r.cfg.GPU.Enabled {
		gpus := detectGPUs()
		if len(gpus) > 0 {
			r.primaryGPUVendor = gpus[0].Vendor
			r.log.Info("GPU detected", "vendor", gpus[0].Vendor, "model", gpus[0].Model)
		}
	}

	// Open offline journal.
	offDB, err := newOfflineStore(r.cfg.Agent.OfflineDB)
	if err != nil {
		return fmt.Errorf("offline store: %w", err)
	}
	r.offline = offDB
	defer offDB.close()

	// Establish gRPC connection with reconnect loop.
	if err := r.connect(ctx); err != nil {
		return fmt.Errorf("initial connect: %w", err)
	}
	defer r.conn.Close()

	// Register with controller (blocks until approved).
	if err := r.register(ctx); err != nil {
		return fmt.Errorf("register: %w", err)
	}

	// Sync any offline results and logs from previous runs.
	r.syncOfflineResults(ctx)

	r.setState(pb.AgentState_AGENT_STATE_IDLE, "")

	// Start upgrade checker goroutine.
	upgrader := newUpgradeChecker(r.cfg, r.log)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		upgrader.checkLoop(ctx, func() bool {
			r.mu.Lock()
			busy := r.state == pb.AgentState_AGENT_STATE_BUSY
			r.mu.Unlock()
			return busy
		})
	}()

	// Start heartbeat goroutine.
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.heartbeatLoop(ctx)
	}()

	// Task poll loop (runs in current goroutine).
	r.pollLoop(ctx)

	wg.Wait()
	r.log.Info("agent stopped")
	return nil
}

// connect establishes a gRPC connection to the controller using exponential
// backoff.
func (r *runner) connect(ctx context.Context) error {
	delay := r.cfg.Controller.Reconnect.InitialDelay
	if delay == 0 {
		delay = 5 * time.Second
	}
	maxDelay := r.cfg.Controller.Reconnect.MaxDelay
	if maxDelay == 0 {
		maxDelay = 5 * time.Minute
	}
	multiplier := r.cfg.Controller.Reconnect.Multiplier
	if multiplier < 1 {
		multiplier = 2.0
	}

	var creds grpc.DialOption
	if r.cfg.Controller.TLS.Cert != "" && r.cfg.Controller.TLS.Key != "" && r.cfg.Controller.TLS.CA != "" {
		tlsCfg, err := buildTLSConfig(r.cfg.Controller.TLS)
		if err != nil {
			return fmt.Errorf("tls config: %w", err)
		}
		creds = grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg))
	} else {
		r.log.Warn("TLS not configured, using insecure connection (dev mode)")
		creds = grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	currentDelay := delay
	for {
		r.log.Info("connecting to controller", "address", r.cfg.Controller.Address)
		conn, err := grpc.NewClient(r.cfg.Controller.Address, creds)
		if err == nil {
			r.conn = conn
			r.client = pb.NewAgentServiceClient(conn)
			r.log.Info("connected to controller")
			return nil
		}
		r.log.Error("connection failed, retrying", "error", err, "delay", currentDelay)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(currentDelay):
		}

		currentDelay = time.Duration(float64(currentDelay) * multiplier)
		if currentDelay > maxDelay {
			currentDelay = maxDelay
		}
	}
}

// buildTLSConfig creates a mutual TLS configuration from the agent's cert,
// key, and CA files.
func buildTLSConfig(cfg agentcfg.TLSConfig) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.Cert, cfg.Key)
	if err != nil {
		return nil, fmt.Errorf("loading client cert/key: %w", err)
	}
	caCert, err := os.ReadFile(cfg.CA)
	if err != nil {
		return nil, fmt.Errorf("reading CA cert: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// register calls the controller Register RPC with exponential backoff until
// the agent is registered AND approved. Implements the PENDING_APPROVAL state
// from the agent state machine (AGENTS.md §3).
func (r *runner) register(ctx context.Context) error {
	hostname := r.cfg.Agent.Hostname
	if hostname == "" {
		hostname, _ = os.Hostname()
	}
	ip := localIP()

	info := &pb.AgentInfo{
		Hostname:     hostname,
		IpAddress:    ip,
		AgentVersion: agentVersion,
		OsVersion:    runtime.GOOS + "/" + runtime.GOARCH,
		CpuCount:     int32(runtime.NumCPU()),
	}
	if r.cfg.GPU.Enabled && r.primaryGPUVendor != "" {
		info.Gpu = &pb.GPUCapabilities{
			Vendor: r.primaryGPUVendor,
		}
	}

	delay := r.cfg.Controller.Reconnect.InitialDelay
	if delay == 0 {
		delay = 5 * time.Second
	}
	maxDelay := r.cfg.Controller.Reconnect.MaxDelay
	if maxDelay == 0 {
		maxDelay = 5 * time.Minute
	}
	multiplier := r.cfg.Controller.Reconnect.Multiplier
	if multiplier < 1 {
		multiplier = 2.0
	}

	currentDelay := delay
	pendingLogged := false

	for {
		resp, err := r.client.Register(ctx, info)
		if err != nil {
			r.log.Error("registration failed, retrying", "error", err, "delay", currentDelay)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(currentDelay):
			}
			currentDelay = time.Duration(float64(currentDelay) * multiplier)
			if currentDelay > maxDelay {
				currentDelay = maxDelay
			}
			continue
		}
		if !resp.GetOk() {
			r.log.Warn("registration rejected by controller", "message", resp.GetMessage())
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(currentDelay):
			}
			continue
		}

		r.agentID = resp.GetAgentId()

		// If not yet approved, enter PENDING_APPROVAL state and poll until approved.
		if !resp.GetApproved() {
			if !pendingLogged {
				r.log.Info("agent pending admin approval", "agent_id", r.agentID)
				pendingLogged = true
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(30 * time.Second):
			}
			// Re-register to check approval status.
			continue
		}

		r.log.Info("registered with controller", "agent_id", r.agentID)
		return nil
	}
}

// heartbeatLoop sends periodic heartbeats to the controller.
func (r *runner) heartbeatLoop(ctx context.Context) {
	interval := r.cfg.Agent.HeartbeatInterval
	if interval == 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.sendHeartbeat(ctx)
		}
	}
}

// sendHeartbeat sends a single heartbeat RPC.
func (r *runner) sendHeartbeat(ctx context.Context) {
	r.mu.Lock()
	state := r.state
	taskID := r.currentTaskID
	r.mu.Unlock()

	resp, err := r.client.Heartbeat(ctx, &pb.HeartbeatReq{
		AgentId:       r.agentID,
		State:         state,
		CurrentTaskId: taskID,
	})
	if err != nil {
		r.log.Error("heartbeat failed", "error", err)
		return
	}
	if resp.GetDrain() {
		r.log.Warn("controller requested drain")
	}
	if resp.GetDisabled() {
		r.log.Warn("controller disabled this agent")
	}
}

// pollLoop polls the controller for task assignments.
func (r *runner) pollLoop(ctx context.Context) {
	interval := r.cfg.Agent.PollInterval
	if interval == 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.mu.Lock()
			busy := r.state == pb.AgentState_AGENT_STATE_BUSY
			r.mu.Unlock()
			if busy {
				continue
			}
			r.pollAndExecute(ctx)
		}
	}
}

// pollAndExecute polls for a task and, if one is assigned, executes it.
func (r *runner) pollAndExecute(ctx context.Context) {
	resp, err := r.client.PollTask(ctx, &pb.PollTaskReq{
		AgentId: r.agentID,
	})
	if err != nil {
		r.log.Error("poll task failed", "error", err)
		return
	}
	if !resp.GetHasTask() {
		return
	}

	r.log.Info("task received", "task_id", resp.GetTaskId(), "job_id", resp.GetJobId())
	r.setState(pb.AgentState_AGENT_STATE_BUSY, resp.GetTaskId())

	startedAt := time.Now()
	exitCode, execErr := r.executeTask(ctx, resp)
	completedAt := time.Now()

	success := execErr == nil && exitCode == 0
	errMsg := ""
	if execErr != nil {
		errMsg = execErr.Error()
	}

	result := &pb.TaskResult{
		TaskId:      resp.GetTaskId(),
		JobId:       resp.GetJobId(),
		Success:     success,
		ExitCode:    int32(exitCode),
		ErrorMsg:    errMsg,
		StartedAt:   timestamppb.New(startedAt),
		CompletedAt: timestamppb.New(completedAt),
	}

	if _, err := r.client.ReportResult(ctx, result); err != nil {
		r.log.Error("report result failed, saving offline", "error", err)
		if saveErr := r.offline.saveResult(resp.GetTaskId(), resp.GetJobId(), success, int32(exitCode), errMsg); saveErr != nil {
			r.log.Error("failed to save offline result", "error", saveErr)
		}
	} else {
		r.log.Info("task result reported", "task_id", resp.GetTaskId(), "success", success)
	}

	// Clean up work directory on success if configured.
	if success && r.cfg.Agent.CleanupOnSuccess {
		workDir := filepath.Join(r.cfg.Agent.WorkDir, resp.GetTaskId())
		if err := os.RemoveAll(workDir); err != nil {
			r.log.Warn("cleanup work dir failed", "path", workDir, "error", err)
		}
	}

	r.setState(pb.AgentState_AGENT_STATE_IDLE, "")
}

// validateTask performs pre-execution checks per AGENTS.md §5.1.
// Returns a descriptive error if any check fails.
func (r *runner) validateTask(task *pb.TaskAssignment) error {
	// Timeout sanity.
	if task.GetTimeoutSec() < 0 {
		return fmt.Errorf("validation: invalid timeout %d", task.GetTimeoutSec())
	}

	// DE_PARAM_* variable completeness — all variables must be non-empty.
	for k, v := range task.GetVariables() {
		if strings.HasPrefix(k, "DE_PARAM_") && v == "" {
			return fmt.Errorf("validation: required param %s is empty", k)
		}
	}

	// UNC path validation.
	if path := task.GetSourcePath(); path != "" {
		if err := r.validateUNCPath(path); err != nil {
			return fmt.Errorf("validation: source_path: %w", err)
		}
	}
	if path := task.GetOutputPath(); path != "" {
		if err := r.validateUNCPath(path); err != nil {
			return fmt.Errorf("validation: output_path: %w", err)
		}
	}

	// Script content must be present.
	if len(task.GetScripts()) == 0 {
		return fmt.Errorf("validation: task contains no scripts")
	}

	// Windows requires a .bat entrypoint; all other platforms require a .sh entrypoint.
	requiredExt := ".sh"
	if runtime.GOOS == "windows" {
		requiredExt = ".bat"
	}
	hasEntrypoint := false
	for name, content := range task.GetScripts() {
		if strings.HasSuffix(strings.ToLower(name), requiredExt) {
			hasEntrypoint = true
			if strings.TrimSpace(content) == "" {
				return fmt.Errorf("validation: script %q is empty", name)
			}
		}
	}
	if !hasEntrypoint {
		return fmt.Errorf("validation: no %s script found in task", requiredExt)
	}

	return nil
}

// validateUNCPath checks that path is within one of the configured allowed
// shares per AGENTS.md §7.2. If allowed_shares is empty, all paths pass.
func (r *runner) validateUNCPath(path string) error {
	if len(r.cfg.AllowedShares) == 0 {
		return nil
	}
	pathLower := strings.ToLower(path)
	for _, prefix := range r.cfg.AllowedShares {
		if strings.HasPrefix(pathLower, strings.ToLower(prefix)) {
			return nil
		}
	}
	return fmt.Errorf("path %q not in allowed shares", path)
}

// executeTask validates, writes script files to disk, and runs the .bat
// entrypoint, streaming stdout/stderr to the controller with offline fallback.
func (r *runner) executeTask(ctx context.Context, task *pb.TaskAssignment) (int, error) {
	// §5.1 Pre-execution validation.
	if err := r.validateTask(task); err != nil {
		r.log.Error("task validation failed", "task_id", task.GetTaskId(), "error", err)
		// Stream validation failure as an agent-level log entry.
		r.streamAgentLog(ctx, task, "error", err.Error())
		return -1, err
	}

	workDir := filepath.Join(r.cfg.Agent.WorkDir, task.GetTaskId())
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return -1, fmt.Errorf("creating work dir: %w", err)
	}

	// Select the platform-appropriate script entrypoint extension.
	entryExt := ".sh"
	if runtime.GOOS == "windows" {
		entryExt = ".bat"
	}

	// Write script files.
	var entryPath string
	for name, content := range task.GetScripts() {
		p := filepath.Join(workDir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return -1, fmt.Errorf("creating script dir for %s: %w", name, err)
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(strings.ToLower(name), ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(p, []byte(content), mode); err != nil {
			return -1, fmt.Errorf("writing script %s: %w", name, err)
		}
		if strings.HasSuffix(strings.ToLower(name), entryExt) {
			entryPath = p
		}
	}
	if entryPath == "" {
		return -1, fmt.Errorf("no %s script found in task scripts", entryExt)
	}

	// Apply task timeout if specified.
	execCtx := ctx
	if task.GetTimeoutSec() > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, time.Duration(task.GetTimeoutSec())*time.Second)
		defer cancel()
	}

	r.log.Info("executing task", "script", entryPath, "task_id", task.GetTaskId())

	// §5.3 Build the entrypoint command with DE_PARAM_* environment variables.
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(execCtx, "cmd.exe", "/c", entryPath)
	} else {
		cmd = exec.CommandContext(execCtx, "/bin/sh", entryPath)
	}
	cmd.Dir = workDir

	env := os.Environ()
	for k, v := range task.GetVariables() {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return -1, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return -1, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return -1, fmt.Errorf("starting command: %w", err)
	}

	// Open a log stream to the controller (may be nil if unavailable).
	logStream, streamErr := r.client.StreamLogs(ctx)
	if streamErr != nil {
		r.log.Warn("could not open log stream, falling back to offline journal", "error", streamErr)
	}

	// §5.4 Start progress streamer.
	var gpuFn func() *gpuSample
	var gpuCh <-chan gpuSample
	if r.cfg.GPU.Enabled && r.primaryGPUVendor != "" {
		monitorInterval := r.cfg.GPU.MonitorInterval
		if monitorInterval == 0 {
			monitorInterval = 5 * time.Second
		}
		gpuCh = monitorGPU(execCtx, r.primaryGPUVendor, monitorInterval)
		var lastSample gpuSample
		var sampleMu sync.Mutex
		// Forward GPU samples to a local variable for the progress streamer.
		go func() {
			for s := range gpuCh {
				sampleMu.Lock()
				lastSample = s
				sampleMu.Unlock()
			}
		}()
		gpuFn = func() *gpuSample {
			sampleMu.Lock()
			s := lastSample
			sampleMu.Unlock()
			return &s
		}
	}

	ps := newProgressStreamer(r.client, task.GetTaskId(), task.GetJobId(), r.log, gpuFn)
	cancelProgress := ps.start(execCtx)
	defer cancelProgress()

	// Stream stdout and stderr in goroutines with offline fallback.
	var streamWg sync.WaitGroup

	streamLine := func(stream string, scanner *bufio.Scanner) {
		defer streamWg.Done()
		for scanner.Scan() {
			line := scanner.Text()

			// §5.4 Parse progress from stdout.
			if stream == "stdout" {
				if pm := parseProgress(line); pm != nil {
					select {
					case ps.ch <- pm:
					default:
					}
				}
			}

			level := "info"
			if stream == "stderr" {
				level = "warn"
			}

			entry := &pb.LogEntry{
				TaskId:    task.GetTaskId(),
				JobId:     task.GetJobId(),
				Stream:    stream,
				Level:     level,
				Message:   line,
				Timestamp: timestamppb.Now(),
			}

			if logStream != nil && streamErr == nil {
				if sendErr := logStream.Send(entry); sendErr != nil {
					// §8.1 Fall back to offline journal on send error.
					r.log.Warn("log stream send failed, buffering offline", "error", sendErr)
					_ = r.offline.saveLog(task.GetTaskId(), task.GetJobId(), stream, level, line)
				}
			} else {
				// §8.1 No stream available — journal directly.
				_ = r.offline.saveLog(task.GetTaskId(), task.GetJobId(), stream, level, line)
			}
		}
	}

	streamWg.Add(2)
	go streamLine("stdout", bufio.NewScanner(stdout))
	go streamLine("stderr", bufio.NewScanner(stderr))
	streamWg.Wait()

	// Close the log stream.
	if logStream != nil && streamErr == nil {
		if _, err := logStream.CloseAndRecv(); err != nil {
			r.log.Warn("closing log stream", "error", err)
		}
	}

	waitErr := cmd.Wait()
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return -1, waitErr
	}
	return 0, nil
}

// streamAgentLog sends a single agent-level log entry to the controller,
// falling back to the offline journal if the stream is unavailable.
func (r *runner) streamAgentLog(ctx context.Context, task *pb.TaskAssignment, level, message string) {
	logStream, err := r.client.StreamLogs(ctx)
	if err != nil {
		_ = r.offline.saveLog(task.GetTaskId(), task.GetJobId(), "agent", level, message)
		return
	}
	_ = logStream.Send(&pb.LogEntry{
		TaskId:    task.GetTaskId(),
		JobId:     task.GetJobId(),
		Stream:    "agent",
		Level:     level,
		Message:   message,
		Timestamp: timestamppb.Now(),
	})
	_, _ = logStream.CloseAndRecv()
}

// syncOfflineResults replays buffered results and logs to the controller
// per AGENTS.md §4.3.
func (r *runner) syncOfflineResults(ctx context.Context) {
	r.syncResults(ctx)
	r.syncLogs(ctx)
}

// syncResults syncs buffered task results via the SyncOfflineResults RPC.
func (r *runner) syncResults(ctx context.Context) {
	results, err := r.offline.pendingResults()
	if err != nil {
		r.log.Error("reading offline results", "error", err)
		return
	}
	if len(results) == 0 {
		return
	}

	r.log.Info("syncing offline results", "count", len(results))

	stream, err := r.client.SyncOfflineResults(ctx)
	if err != nil {
		r.log.Error("opening sync stream", "error", err)
		return
	}

	for _, res := range results {
		if err := stream.Send(&pb.TaskResult{
			TaskId:        res.TaskID,
			JobId:         res.JobID,
			Success:       res.Success,
			ExitCode:      res.ExitCode,
			ErrorMsg:      res.ErrorMsg,
			OfflineResult: true,
		}); err != nil {
			r.log.Error("sending offline result", "error", err, "task_id", res.TaskID)
			break
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		r.log.Error("closing sync stream", "error", err)
		return
	}

	rejected := make(map[string]bool, len(resp.GetRejectedTaskIds()))
	for _, tid := range resp.GetRejectedTaskIds() {
		rejected[tid] = true
	}
	for _, res := range results {
		if err := r.offline.markSynced(res.ID); err != nil {
			r.log.Error("marking result synced", "error", err, "id", res.ID)
		}
	}

	r.log.Info("offline results sync complete", "accepted", resp.GetAccepted(), "rejected", len(rejected))
}

// syncLogs syncs buffered log lines via the StreamLogs RPC.
func (r *runner) syncLogs(ctx context.Context) {
	logs, err := r.offline.pendingLogs()
	if err != nil {
		r.log.Error("reading offline logs", "error", err)
		return
	}
	if len(logs) == 0 {
		return
	}

	r.log.Info("syncing offline logs", "count", len(logs))

	stream, err := r.client.StreamLogs(ctx)
	if err != nil {
		r.log.Error("opening log sync stream", "error", err)
		return
	}

	var synced []int64
	for _, l := range logs {
		if err := stream.Send(&pb.LogEntry{
			TaskId:    l.TaskID,
			JobId:     l.JobID,
			Stream:    l.Stream,
			Level:     l.Level,
			Message:   l.Message,
			Timestamp: timestamppb.New(l.CreatedAt),
		}); err != nil {
			r.log.Error("sending offline log", "error", err)
			break
		}
		synced = append(synced, l.ID)
	}

	if _, err := stream.CloseAndRecv(); err != nil {
		r.log.Warn("closing log sync stream", "error", err)
	}

	if err := r.offline.markLogsSynced(synced); err != nil {
		r.log.Error("marking logs synced", "error", err)
	}

	r.log.Info("offline logs sync complete", "synced", len(synced))
}

// setState updates the agent's current state atomically.
func (r *runner) setState(state pb.AgentState, taskID string) {
	r.mu.Lock()
	r.state = state
	r.currentTaskID = taskID
	r.mu.Unlock()
}

// localIP returns the first non-loopback IPv4 address found on the host.
func localIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "unknown"
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			return ipNet.IP.String()
		}
	}
	return "unknown"
}

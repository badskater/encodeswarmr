package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"log/slog"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	agentcfg "github.com/badskater/distributed-encoder/internal/agent/config"
	pb "github.com/badskater/distributed-encoder/internal/proto/encoderv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// buildToolEnv returns the environment slice the runner would build for a task,
// using the same logic as executeTask but without the full process setup.
// It exists to let us test the tool-path injection rules in isolation.
func buildToolEnv(tools agentcfg.ToolsConfig, taskVars map[string]string) []string {
	env := os.Environ()
	if tools.FFmpeg != "" {
		env = append(env, "FFMPEG_BIN="+tools.FFmpeg)
	}
	if tools.FFprobe != "" {
		env = append(env, "FFPROBE_BIN="+tools.FFprobe)
	}
	if tools.DoviTool != "" {
		env = append(env, "DOVI_TOOL_BIN="+tools.DoviTool)
	}
	for k, v := range taskVars {
		env = append(env, k+"="+v)
	}
	return env
}

// lastValue returns the last value assigned to key in a KEY=VALUE slice,
// mimicking the "last assignment wins" behaviour of most process environments.
func lastValue(env []string, key string) string {
	val := ""
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			val = strings.TrimPrefix(entry, prefix)
		}
	}
	return val
}

func TestBuildToolEnv_ToolsInjected(t *testing.T) {
	tools := agentcfg.ToolsConfig{
		FFmpeg:   "/usr/bin/ffmpeg",
		FFprobe:  "/usr/bin/ffprobe",
		DoviTool: "/usr/local/bin/dovi_tool",
	}
	env := buildToolEnv(tools, nil)

	checks := map[string]string{
		"FFMPEG_BIN":    "/usr/bin/ffmpeg",
		"FFPROBE_BIN":   "/usr/bin/ffprobe",
		"DOVI_TOOL_BIN": "/usr/local/bin/dovi_tool",
	}
	for key, want := range checks {
		if got := lastValue(env, key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestBuildToolEnv_EmptyToolsNotInjected(t *testing.T) {
	// When a tool path is empty, the env var should not appear at all.
	tools := agentcfg.ToolsConfig{
		FFmpeg:   "/usr/bin/ffmpeg",
		FFprobe:  "",
		DoviTool: "",
	}
	env := buildToolEnv(tools, nil)

	for _, key := range []string{"FFPROBE_BIN", "DOVI_TOOL_BIN"} {
		if got := lastValue(env, key); got != "" {
			t.Errorf("%s should not be set, but got %q", key, got)
		}
	}
	if got := lastValue(env, "FFMPEG_BIN"); got != "/usr/bin/ffmpeg" {
		t.Errorf("FFMPEG_BIN = %q, want /usr/bin/ffmpeg", got)
	}
}

func TestBuildToolEnv_TaskVarsOverrideTools(t *testing.T) {
	// Task variables must override the agent config tool paths when the same
	// key is present, since they are appended last.
	tools := agentcfg.ToolsConfig{
		FFprobe: "/usr/bin/ffprobe",
	}
	taskVars := map[string]string{
		"FFPROBE_BIN": "/custom/ffprobe",
	}
	env := buildToolEnv(tools, taskVars)

	if got := lastValue(env, "FFPROBE_BIN"); got != "/custom/ffprobe" {
		t.Errorf("FFPROBE_BIN = %q, want /custom/ffprobe (task var should override)", got)
	}
}

func TestBuildToolEnv_DoviToolInjected(t *testing.T) {
	tools := agentcfg.ToolsConfig{DoviTool: "/opt/dovi_tool/dovi_tool"}
	env := buildToolEnv(tools, nil)

	if got := lastValue(env, "DOVI_TOOL_BIN"); got != "/opt/dovi_tool/dovi_tool" {
		t.Errorf("DOVI_TOOL_BIN = %q, want /opt/dovi_tool/dovi_tool", got)
	}
}

// TestToolEnvVisibleToChild verifies end-to-end that a child process can read
// FFPROBE_BIN and DOVI_TOOL_BIN when they are set via buildToolEnv.
// The test is skipped on platforms where "sh" is unavailable (e.g. pure Windows).
func TestToolEnvVisibleToChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh not available on Windows; tool env injection is covered by unit tests above")
	}

	tools := agentcfg.ToolsConfig{
		FFprobe:  "/test/ffprobe",
		DoviTool: "/test/dovi_tool",
	}
	env := buildToolEnv(tools, nil)

	// Write a tiny shell script that prints the env vars.
	dir := t.TempDir()
	script := filepath.Join(dir, "check.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho FFPROBE_BIN=$FFPROBE_BIN\necho DOVI_TOOL_BIN=$DOVI_TOOL_BIN\n"), 0o755); err != nil {
		t.Fatalf("writing script: %v", err)
	}

	cmd := exec.Command("/bin/sh", script)
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running script: %v", err)
	}

	got := string(out)
	if !strings.Contains(got, "FFPROBE_BIN=/test/ffprobe") {
		t.Errorf("child did not see FFPROBE_BIN; output:\n%s", got)
	}
	if !strings.Contains(got, "DOVI_TOOL_BIN=/test/dovi_tool") {
		t.Errorf("child did not see DOVI_TOOL_BIN; output:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// isCloudURI — pure function
// ---------------------------------------------------------------------------

func TestIsCloudURI_S3(t *testing.T) {
	cases := []struct {
		uri  string
		want bool
	}{
		{"s3://my-bucket/video.mkv", true},
		{"S3://MY-BUCKET/video.mkv", true}, // case-insensitive
		{"gs://my-bucket/video.mkv", true},
		{"GS://bucket/file", true},
		{"az://container/blob", true},
		{"AZ://container/blob", true},
		{`\\server\share\video.mkv`, false},
		{"/mnt/nas/video.mkv", false},
		{"C:\\video\\file.mkv", false},
		{"file:///local/path", false},
		{"http://example.com/file", false},
		{"", false},
	}
	for _, tc := range cases {
		got := isCloudURI(tc.uri)
		if got != tc.want {
			t.Errorf("isCloudURI(%q) = %v, want %v", tc.uri, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// validateTask — pure method (no gRPC needed)
// ---------------------------------------------------------------------------

// newRunnerForValidation builds a minimal runner suitable for validateTask tests.
func newRunnerForValidation(allowedShares []string) *runner {
	return &runner{
		cfg: &agentcfg.Config{
			AllowedShares: allowedShares,
		},
		log: slog.Default(),
	}
}

// entrypointScript returns the correct script map for the current platform.
func entrypointScript() map[string]string {
	if runtime.GOOS == "windows" {
		return map[string]string{"encode.bat": "@echo off\necho running\n"}
	}
	return map[string]string{"encode.sh": "#!/bin/sh\necho running\n"}
}

func TestValidateTask_ValidTask(t *testing.T) {
	r := newRunnerForValidation(nil)
	task := &pb.TaskAssignment{
		TaskId:  "task-1",
		Scripts: entrypointScript(),
	}
	if err := r.validateTask(task); err != nil {
		t.Errorf("validateTask on valid task: %v", err)
	}
}

func TestValidateTask_NoScripts(t *testing.T) {
	r := newRunnerForValidation(nil)
	task := &pb.TaskAssignment{
		TaskId:  "task-2",
		Scripts: map[string]string{},
	}
	err := r.validateTask(task)
	if err == nil {
		t.Fatal("expected error for task with no scripts")
	}
	if !strings.Contains(err.Error(), "no scripts") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateTask_MissingEntrypoint(t *testing.T) {
	r := newRunnerForValidation(nil)
	// Provide only a script of the wrong extension.
	var scripts map[string]string
	if runtime.GOOS == "windows" {
		scripts = map[string]string{"encode.sh": "#!/bin/sh\necho hi\n"}
	} else {
		scripts = map[string]string{"encode.bat": "@echo off\n"}
	}
	task := &pb.TaskAssignment{
		TaskId:  "task-3",
		Scripts: scripts,
	}
	err := r.validateTask(task)
	if err == nil {
		t.Fatal("expected error for missing entrypoint")
	}
}

func TestValidateTask_EmptyEntrypointScript(t *testing.T) {
	r := newRunnerForValidation(nil)
	var scripts map[string]string
	if runtime.GOOS == "windows" {
		scripts = map[string]string{"encode.bat": "   "}
	} else {
		scripts = map[string]string{"encode.sh": "   "}
	}
	task := &pb.TaskAssignment{
		TaskId:  "task-4",
		Scripts: scripts,
	}
	err := r.validateTask(task)
	if err == nil {
		t.Fatal("expected error for empty entrypoint script body")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateTask_NegativeTimeout(t *testing.T) {
	r := newRunnerForValidation(nil)
	task := &pb.TaskAssignment{
		TaskId:     "task-5",
		TimeoutSec: -1,
		Scripts:    entrypointScript(),
	}
	err := r.validateTask(task)
	if err == nil {
		t.Fatal("expected error for negative timeout")
	}
	if !strings.Contains(err.Error(), "invalid timeout") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateTask_EmptyDEParamVariable(t *testing.T) {
	r := newRunnerForValidation(nil)
	task := &pb.TaskAssignment{
		TaskId:    "task-6",
		Scripts:   entrypointScript(),
		Variables: map[string]string{"DE_PARAM_INPUT": ""},
	}
	err := r.validateTask(task)
	if err == nil {
		t.Fatal("expected error for empty DE_PARAM_ variable")
	}
	if !strings.Contains(err.Error(), "DE_PARAM_INPUT") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateTask_NonEmptyDEParamOK(t *testing.T) {
	r := newRunnerForValidation(nil)
	task := &pb.TaskAssignment{
		TaskId:    "task-7",
		Scripts:   entrypointScript(),
		Variables: map[string]string{"DE_PARAM_INPUT": "/mnt/nas/video.mkv"},
	}
	if err := r.validateTask(task); err != nil {
		t.Errorf("validateTask with valid DE_PARAM_: %v", err)
	}
}

// ---------------------------------------------------------------------------
// validateSharePath — pure method
// ---------------------------------------------------------------------------

func TestValidateSharePath_EmptyAllowedShares_AllowsAll(t *testing.T) {
	r := newRunnerForValidation(nil)
	paths := []string{
		`\\server\share\video.mkv`,
		"/mnt/nas/video.mkv",
		"C:\\video\\file.mkv",
	}
	for _, p := range paths {
		if err := r.validateSharePath(p); err != nil {
			t.Errorf("validateSharePath(%q) with no allowed_shares: %v", p, err)
		}
	}
}

func TestValidateSharePath_AllowedPrefixMatches(t *testing.T) {
	r := newRunnerForValidation([]string{`\\nas01\media`, "/mnt/nas"})
	allowed := []string{
		`\\nas01\media\movie.mkv`,
		`\\NAS01\MEDIA\movie.mkv`, // case-insensitive
		"/mnt/nas/video.mkv",
		"/MNT/NAS/video.mkv",
	}
	for _, p := range allowed {
		if err := r.validateSharePath(p); err != nil {
			t.Errorf("validateSharePath(%q): unexpected error: %v", p, err)
		}
	}
}

func TestValidateSharePath_DisallowedPath(t *testing.T) {
	r := newRunnerForValidation([]string{`\\nas01\media`})
	err := r.validateSharePath(`\\other-server\share\file.mkv`)
	if err == nil {
		t.Fatal("expected error for path not in allowed shares")
	}
	if !strings.Contains(err.Error(), "not in allowed shares") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// setState — race-safe state transitions
// ---------------------------------------------------------------------------

func TestSetState_Transitions(t *testing.T) {
	r := &runner{
		cfg: &agentcfg.Config{},
		log: slog.Default(),
	}

	r.setState(pb.AgentState_AGENT_STATE_IDLE, "")
	r.mu.Lock()
	if r.state != pb.AgentState_AGENT_STATE_IDLE {
		t.Errorf("state = %v, want IDLE", r.state)
	}
	if r.currentTaskID != "" {
		t.Errorf("currentTaskID = %q, want empty", r.currentTaskID)
	}
	r.mu.Unlock()

	r.setState(pb.AgentState_AGENT_STATE_BUSY, "task-abc")
	r.mu.Lock()
	if r.state != pb.AgentState_AGENT_STATE_BUSY {
		t.Errorf("state = %v, want BUSY", r.state)
	}
	if r.currentTaskID != "task-abc" {
		t.Errorf("currentTaskID = %q, want task-abc", r.currentTaskID)
	}
	r.mu.Unlock()
}

func TestSetState_Concurrent(t *testing.T) {
	r := &runner{
		cfg: &agentcfg.Config{},
		log: slog.Default(),
	}
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(i int) {
			r.setState(pb.AgentState_AGENT_STATE_BUSY, "task")
			r.setState(pb.AgentState_AGENT_STATE_IDLE, "")
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

// ---------------------------------------------------------------------------
// localIP — returns non-empty string
// ---------------------------------------------------------------------------

func TestLocalIP_ReturnsValue(t *testing.T) {
	ip := localIP()
	if ip == "" {
		t.Error("localIP() returned empty string")
	}
	// Valid results are either a dotted-decimal IPv4 address or "unknown".
	if ip == "unknown" {
		t.Log("localIP() returned 'unknown' (no non-loopback interface found)")
		return
	}
	if net.ParseIP(ip) == nil {
		t.Errorf("localIP() = %q is not a valid IP address", ip)
	}
}

// ---------------------------------------------------------------------------
// buildTLSConfig — requires real cert/key/CA files
// ---------------------------------------------------------------------------

// generateSelfSignedCert creates a minimal self-signed ECDSA certificate and
// writes cert.pem, key.pem, and ca.pem to dir (ca == cert for self-signed).
func generateSelfSignedCert(t *testing.T, dir string) (certPath, keyPath, caPath string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-agent"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		IsCA:         true,
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	caPath = filepath.Join(dir, "ca.pem")

	// Write cert PEM.
	cf, err := os.Create(certPath)
	if err != nil {
		t.Fatalf("create cert file: %v", err)
	}
	_ = pem.Encode(cf, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	cf.Close()

	// Write key PEM.
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	kf, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("create key file: %v", err)
	}
	_ = pem.Encode(kf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	kf.Close()

	// CA == cert for self-signed.
	caData, _ := os.ReadFile(certPath)
	_ = os.WriteFile(caPath, caData, 0o644)

	return certPath, keyPath, caPath
}

func TestBuildTLSConfig_ValidCerts(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath, caPath := generateSelfSignedCert(t, dir)

	cfg := agentcfg.TLSConfig{
		Cert: certPath,
		Key:  keyPath,
		CA:   caPath,
	}
	tlsCfg, err := buildTLSConfig(cfg)
	if err != nil {
		t.Fatalf("buildTLSConfig: %v", err)
	}
	if tlsCfg == nil {
		t.Fatal("buildTLSConfig returned nil config")
	}
	if tlsCfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %d, want TLS 1.2", tlsCfg.MinVersion)
	}
	if len(tlsCfg.Certificates) != 1 {
		t.Errorf("Certificates len = %d, want 1", len(tlsCfg.Certificates))
	}
}

func TestBuildTLSConfig_MissingCert(t *testing.T) {
	cfg := agentcfg.TLSConfig{
		Cert: "/nonexistent/cert.pem",
		Key:  "/nonexistent/key.pem",
		CA:   "/nonexistent/ca.pem",
	}
	_, err := buildTLSConfig(cfg)
	if err == nil {
		t.Fatal("expected error for missing cert files")
	}
}

func TestBuildTLSConfig_BadCA(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath, _ := generateSelfSignedCert(t, dir)

	// Write an invalid CA file.
	badCA := filepath.Join(dir, "bad-ca.pem")
	_ = os.WriteFile(badCA, []byte("not a pem"), 0o644)

	cfg := agentcfg.TLSConfig{
		Cert: certPath,
		Key:  keyPath,
		CA:   badCA,
	}
	_, err := buildTLSConfig(cfg)
	if err == nil {
		t.Fatal("expected error for invalid CA PEM")
	}
}

// ---------------------------------------------------------------------------
// connect — insecure path (no TLS configured)
// ---------------------------------------------------------------------------

func TestConnect_Insecure(t *testing.T) {
	r := &runner{
		cfg: &agentcfg.Config{
			Controller: agentcfg.ControllerConfig{
				Address: "localhost:50051",
			},
		},
		log: slog.Default(),
	}
	ctx := context.Background()
	if err := r.connect(ctx); err != nil {
		t.Fatalf("connect (insecure): %v", err)
	}
	defer r.conn.Close()

	if r.conn == nil {
		t.Error("expected non-nil conn after connect")
	}
	if r.client == nil {
		t.Error("expected non-nil client after connect")
	}
}

// ---------------------------------------------------------------------------
// newUpgradeChecker — constructor logic
// ---------------------------------------------------------------------------

func TestNewUpgradeChecker_HTTPBase(t *testing.T) {
	cases := []struct {
		name        string
		address     string
		hasTLS      bool
		wantScheme  string
		wantContain string
	}{
		{
			name:        "insecure with port",
			address:     "controller.example.com:50051",
			hasTLS:      false,
			wantScheme:  "http",
			wantContain: "controller.example.com:8080",
		},
		{
			name:        "tls with port",
			address:     "secure.example.com:50051",
			hasTLS:      true,
			wantScheme:  "https",
			wantContain: "secure.example.com:8080",
		},
		{
			name:        "no port in address",
			address:     "bare-host",
			hasTLS:      false,
			wantScheme:  "http",
			wantContain: "bare-host:8080",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &agentcfg.Config{
				Controller: agentcfg.ControllerConfig{
					Address: tc.address,
				},
			}
			if tc.hasTLS {
				cfg.Controller.TLS.Cert = "/some/cert.pem"
			}
			u := newUpgradeChecker(cfg, slog.Default())
			if !strings.Contains(u.controllerHTTPBase, tc.wantContain) {
				t.Errorf("controllerHTTPBase = %q, want to contain %q", u.controllerHTTPBase, tc.wantContain)
			}
			if !strings.HasPrefix(u.controllerHTTPBase, tc.wantScheme+"://") {
				t.Errorf("controllerHTTPBase = %q, want scheme %q", u.controllerHTTPBase, tc.wantScheme)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Mock gRPC client infrastructure
// ---------------------------------------------------------------------------

// mockClientStream is a minimal implementation of grpc.ClientStream used
// to embed in typed stream mocks.
type mockClientStream struct {
	sendErr  error
	closeErr error
}

func (m *mockClientStream) Header() (metadata.MD, error)  { return nil, nil }
func (m *mockClientStream) Trailer() metadata.MD           { return nil }
func (m *mockClientStream) CloseSend() error               { return nil }
func (m *mockClientStream) Context() context.Context       { return context.Background() }
func (m *mockClientStream) SendMsg(v interface{}) error    { return m.sendErr }
func (m *mockClientStream) RecvMsg(v interface{}) error    { return nil }

// mockStreamLogs is a mock for the StreamLogs client stream.
type mockStreamLogs struct {
	*mockClientStream
	entries []*pb.LogEntry
}

func (m *mockStreamLogs) Send(e *pb.LogEntry) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.entries = append(m.entries, e)
	return nil
}

func (m *mockStreamLogs) CloseAndRecv() (*pb.Ack, error) {
	return &pb.Ack{}, m.closeErr
}

// mockSyncOfflineResults mock for the SyncOfflineResults stream.
type mockSyncResultsStream struct {
	*mockClientStream
	sent     []*pb.TaskResult
	syncResp *pb.SyncResponse
}

func (m *mockSyncResultsStream) Send(r *pb.TaskResult) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sent = append(m.sent, r)
	return nil
}

func (m *mockSyncResultsStream) CloseAndRecv() (*pb.SyncResponse, error) {
	if m.closeErr != nil {
		return nil, m.closeErr
	}
	if m.syncResp != nil {
		return m.syncResp, nil
	}
	return &pb.SyncResponse{Accepted: int32(len(m.sent))}, nil
}

// mockProgressStream mock for the ReportProgress stream.
type mockProgressStream struct {
	*mockClientStream
	updates []*pb.ProgressUpdate
}

func (m *mockProgressStream) Send(u *pb.ProgressUpdate) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.updates = append(m.updates, u)
	return nil
}

func (m *mockProgressStream) CloseAndRecv() (*pb.Ack, error) {
	return &pb.Ack{}, m.closeErr
}

// mockAgentClient is a configurable mock of pb.AgentServiceClient.
type mockAgentClient struct {
	pb.AgentServiceClient

	registerResp  *pb.RegisterResponse
	registerErr   error
	heartbeatResp *pb.HeartbeatResp
	heartbeatErr  error
	pollTaskResp  *pb.TaskAssignment
	pollTaskErr   error
	reportResultResp *pb.Ack
	reportResultErr  error

	streamLogsStream *mockStreamLogs
	streamLogsErr    error

	syncStream    *mockSyncResultsStream
	syncStreamErr error

	progressStream    *mockProgressStream
	progressStreamErr error
}

func (m *mockAgentClient) Register(ctx context.Context, in *pb.AgentInfo, opts ...grpc.CallOption) (*pb.RegisterResponse, error) {
	return m.registerResp, m.registerErr
}

func (m *mockAgentClient) Heartbeat(ctx context.Context, in *pb.HeartbeatReq, opts ...grpc.CallOption) (*pb.HeartbeatResp, error) {
	return m.heartbeatResp, m.heartbeatErr
}

func (m *mockAgentClient) PollTask(ctx context.Context, in *pb.PollTaskReq, opts ...grpc.CallOption) (*pb.TaskAssignment, error) {
	return m.pollTaskResp, m.pollTaskErr
}

func (m *mockAgentClient) ReportResult(ctx context.Context, in *pb.TaskResult, opts ...grpc.CallOption) (*pb.Ack, error) {
	return m.reportResultResp, m.reportResultErr
}

func (m *mockAgentClient) StreamLogs(ctx context.Context, opts ...grpc.CallOption) (pb.AgentService_StreamLogsClient, error) {
	if m.streamLogsErr != nil {
		return nil, m.streamLogsErr
	}
	if m.streamLogsStream == nil {
		m.streamLogsStream = &mockStreamLogs{mockClientStream: &mockClientStream{}}
	}
	return m.streamLogsStream, nil
}

func (m *mockAgentClient) SyncOfflineResults(ctx context.Context, opts ...grpc.CallOption) (pb.AgentService_SyncOfflineResultsClient, error) {
	if m.syncStreamErr != nil {
		return nil, m.syncStreamErr
	}
	if m.syncStream == nil {
		m.syncStream = &mockSyncResultsStream{mockClientStream: &mockClientStream{}}
	}
	return m.syncStream, nil
}

func (m *mockAgentClient) ReportProgress(ctx context.Context, opts ...grpc.CallOption) (pb.AgentService_ReportProgressClient, error) {
	if m.progressStreamErr != nil {
		return nil, m.progressStreamErr
	}
	if m.progressStream == nil {
		m.progressStream = &mockProgressStream{mockClientStream: &mockClientStream{}}
	}
	return m.progressStream, nil
}

// newRunnerWithMock builds a runner wired to the given mock client and an
// in-memory offline store.
func newRunnerWithMock(t *testing.T, mock *mockAgentClient) *runner {
	t.Helper()
	store, err := newOfflineStore(":memory:")
	if err != nil {
		t.Fatalf("newOfflineStore: %v", err)
	}
	t.Cleanup(func() { store.close() })

	return &runner{
		cfg: &agentcfg.Config{
			Agent: agentcfg.AgentConfig{
				WorkDir: t.TempDir(),
			},
		},
		client:  mock,
		agentID: "agent-test-1",
		log:     slog.Default(),
		offline: store,
		state:   pb.AgentState_AGENT_STATE_IDLE,
	}
}

// ---------------------------------------------------------------------------
// sendHeartbeat
// ---------------------------------------------------------------------------

func TestSendHeartbeat_Success(t *testing.T) {
	mock := &mockAgentClient{
		heartbeatResp: &pb.HeartbeatResp{Drain: false, Disabled: false},
	}
	r := newRunnerWithMock(t, mock)
	r.setState(pb.AgentState_AGENT_STATE_IDLE, "")

	// Should not panic or error — result is logged only.
	r.sendHeartbeat(context.Background())
}

func TestSendHeartbeat_DrainFlag(t *testing.T) {
	mock := &mockAgentClient{
		heartbeatResp: &pb.HeartbeatResp{Drain: true},
	}
	r := newRunnerWithMock(t, mock)
	r.sendHeartbeat(context.Background())
	// Drain is logged; no state change expected here.
}

func TestSendHeartbeat_Error(t *testing.T) {
	mock := &mockAgentClient{
		heartbeatErr: errors.New("connection refused"),
	}
	r := newRunnerWithMock(t, mock)
	// Must not panic.
	r.sendHeartbeat(context.Background())
}

// ---------------------------------------------------------------------------
// pollAndExecute
// ---------------------------------------------------------------------------

func TestPollAndExecute_NoTask(t *testing.T) {
	mock := &mockAgentClient{
		pollTaskResp: &pb.TaskAssignment{HasTask: false},
	}
	r := newRunnerWithMock(t, mock)
	r.pollAndExecute(context.Background())
	// State stays IDLE.
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state != pb.AgentState_AGENT_STATE_IDLE {
		t.Errorf("state = %v after no-task poll, want IDLE", r.state)
	}
}

func TestPollAndExecute_PollError(t *testing.T) {
	mock := &mockAgentClient{
		pollTaskErr: errors.New("grpc timeout"),
	}
	r := newRunnerWithMock(t, mock)
	r.pollAndExecute(context.Background())
	// State stays IDLE.
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state != pb.AgentState_AGENT_STATE_IDLE {
		t.Errorf("state = %v after poll error, want IDLE", r.state)
	}
}

func TestPollAndExecute_InvalidTask_OfflineFallback(t *testing.T) {
	// Provide a task that will fail validation (no scripts).
	mock := &mockAgentClient{
		pollTaskResp: &pb.TaskAssignment{
			HasTask: true,
			TaskId:  "task-bad",
			JobId:   "job-1",
			Scripts: map[string]string{},
		},
		reportResultErr:  errors.New("controller unavailable"),
		streamLogsErr:    errors.New("stream unavailable"),
	}
	r := newRunnerWithMock(t, mock)
	r.pollAndExecute(context.Background())

	// The failed result should have been saved to the offline store.
	results, err := r.offline.pendingResults()
	if err != nil {
		t.Fatalf("pendingResults: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected offline result to be saved after failed task + report error")
	}
}

// ---------------------------------------------------------------------------
// syncResults
// ---------------------------------------------------------------------------

func TestSyncResults_Empty(t *testing.T) {
	mock := &mockAgentClient{}
	r := newRunnerWithMock(t, mock)
	// No pending results — should return without calling SyncOfflineResults.
	r.syncResults(context.Background())
}

func TestSyncResults_WithData(t *testing.T) {
	syncStream := &mockSyncResultsStream{
		mockClientStream: &mockClientStream{},
		syncResp:         &pb.SyncResponse{Accepted: 1},
	}
	mock := &mockAgentClient{syncStream: syncStream}
	r := newRunnerWithMock(t, mock)

	// Insert an offline result.
	if err := r.offline.saveResult("task-1", "job-1", true, 0, ""); err != nil {
		t.Fatalf("saveResult: %v", err)
	}

	r.syncResults(context.Background())

	if len(syncStream.sent) != 1 {
		t.Errorf("sent %d results, want 1", len(syncStream.sent))
	}
	if syncStream.sent[0].TaskId != "task-1" {
		t.Errorf("sent task_id = %q, want task-1", syncStream.sent[0].TaskId)
	}
}

func TestSyncResults_StreamOpenError(t *testing.T) {
	mock := &mockAgentClient{syncStreamErr: errors.New("rpc unavailable")}
	r := newRunnerWithMock(t, mock)

	_ = r.offline.saveResult("task-x", "job-x", false, 1, "fail")
	// Must not panic.
	r.syncResults(context.Background())
}

// ---------------------------------------------------------------------------
// syncLogs
// ---------------------------------------------------------------------------

func TestSyncLogs_Empty(t *testing.T) {
	mock := &mockAgentClient{}
	r := newRunnerWithMock(t, mock)
	r.syncLogs(context.Background())
}

func TestSyncLogs_WithData(t *testing.T) {
	logStream := &mockStreamLogs{mockClientStream: &mockClientStream{}}
	mock := &mockAgentClient{streamLogsStream: logStream}
	r := newRunnerWithMock(t, mock)

	if err := r.offline.saveLog("task-1", "job-1", "stdout", "info", "hello"); err != nil {
		t.Fatalf("saveLog: %v", err)
	}

	r.syncLogs(context.Background())

	if len(logStream.entries) != 1 {
		t.Errorf("sent %d log entries, want 1", len(logStream.entries))
	}
}

func TestSyncLogs_StreamOpenError(t *testing.T) {
	mock := &mockAgentClient{streamLogsErr: errors.New("no stream")}
	r := newRunnerWithMock(t, mock)
	_ = r.offline.saveLog("task-1", "job-1", "stdout", "info", "msg")
	// Must not panic.
	r.syncLogs(context.Background())
}

// ---------------------------------------------------------------------------
// streamAgentLog
// ---------------------------------------------------------------------------

func TestStreamAgentLog_Success(t *testing.T) {
	logStream := &mockStreamLogs{mockClientStream: &mockClientStream{}}
	mock := &mockAgentClient{streamLogsStream: logStream}
	r := newRunnerWithMock(t, mock)

	task := &pb.TaskAssignment{TaskId: "t1", JobId: "j1"}
	r.streamAgentLog(context.Background(), task, "error", "validation failed")

	if len(logStream.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(logStream.entries))
	}
	entry := logStream.entries[0]
	if entry.Level != "error" {
		t.Errorf("Level = %q, want error", entry.Level)
	}
	if entry.Message != "validation failed" {
		t.Errorf("Message = %q, want 'validation failed'", entry.Message)
	}
	if entry.Stream != "agent" {
		t.Errorf("Stream = %q, want agent", entry.Stream)
	}
}

func TestStreamAgentLog_StreamError_FallsBackToOffline(t *testing.T) {
	mock := &mockAgentClient{streamLogsErr: errors.New("unavailable")}
	r := newRunnerWithMock(t, mock)

	task := &pb.TaskAssignment{TaskId: "t2", JobId: "j2"}
	r.streamAgentLog(context.Background(), task, "warn", "offline msg")

	// Message should be in offline store.
	logs, err := r.offline.pendingLogs()
	if err != nil {
		t.Fatalf("pendingLogs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 offline log, got %d", len(logs))
	}
	if logs[0].Message != "offline msg" {
		t.Errorf("offline message = %q, want 'offline msg'", logs[0].Message)
	}
}

// ---------------------------------------------------------------------------
// heartbeatLoop / pollLoop — context cancellation
// ---------------------------------------------------------------------------

func TestHeartbeatLoop_CancelsOnContext(t *testing.T) {
	mock := &mockAgentClient{
		heartbeatResp: &pb.HeartbeatResp{},
	}
	r := newRunnerWithMock(t, mock)
	r.cfg.Agent.HeartbeatInterval = 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		r.heartbeatLoop(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Good — heartbeatLoop returned after ctx cancelled.
	case <-time.After(time.Second):
		t.Error("heartbeatLoop did not stop within 1s after context cancellation")
	}
}

func TestPollLoop_CancelsOnContext(t *testing.T) {
	mock := &mockAgentClient{
		pollTaskResp: &pb.TaskAssignment{HasTask: false},
	}
	r := newRunnerWithMock(t, mock)
	r.cfg.Agent.PollInterval = 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		r.pollLoop(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Good.
	case <-time.After(time.Second):
		t.Error("pollLoop did not stop within 1s after context cancellation")
	}
}

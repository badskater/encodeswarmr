package config

import (
	"os"
	"testing"
	"time"
)

// writeTempConfig writes the given YAML content to a temp file and returns the path.
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "controller-config-*.yaml")
	if err != nil {
		t.Fatalf("create temp config: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	f.Close()
	return f.Name()
}

// ---------------------------------------------------------------------------
// Load — happy path
// ---------------------------------------------------------------------------

func TestLoad_MinimalConfig(t *testing.T) {
	// Only the database URL is provided; all other fields should use defaults.
	path := writeTempConfig(t, `
database:
  url: "postgres://localhost/encoder"
auth:
  session_secret: "secret123"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Database.URL != "postgres://localhost/encoder" {
		t.Errorf("Database.URL = %q, want %q", cfg.Database.URL, "postgres://localhost/encoder")
	}
}

func TestLoad_DefaultValues(t *testing.T) {
	path := writeTempConfig(t, `
database:
  url: "postgres://localhost/encoder"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify a selection of defaults.
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.GRPC.Port != 9443 {
		t.Errorf("GRPC.Port = %d, want 9443", cfg.GRPC.Port)
	}
	if cfg.Auth.SessionTTL != 24*time.Hour {
		t.Errorf("Auth.SessionTTL = %v, want 24h", cfg.Auth.SessionTTL)
	}
	if cfg.Agent.DispatchInterval != 10*time.Second {
		t.Errorf("Agent.DispatchInterval = %v, want 10s", cfg.Agent.DispatchInterval)
	}
	if cfg.Agent.HeartbeatTimeout != 90*time.Second {
		t.Errorf("Agent.HeartbeatTimeout = %v, want 90s", cfg.Agent.HeartbeatTimeout)
	}
	if cfg.Agent.ScriptBaseDir != "/tmp/encoder-scripts" {
		t.Errorf("Agent.ScriptBaseDir = %q, want %q", cfg.Agent.ScriptBaseDir, "/tmp/encoder-scripts")
	}
	if cfg.Agent.TaskTimeoutSec != 3600 {
		t.Errorf("Agent.TaskTimeoutSec = %d, want 3600", cfg.Agent.TaskTimeoutSec)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, "info")
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Logging.Format = %q, want %q", cfg.Logging.Format, "json")
	}
	if cfg.Webhooks.WorkerCount != 4 {
		t.Errorf("Webhooks.WorkerCount = %d, want 4", cfg.Webhooks.WorkerCount)
	}
	if cfg.Webhooks.MaxRetries != 3 {
		t.Errorf("Webhooks.MaxRetries = %d, want 3", cfg.Webhooks.MaxRetries)
	}
	if cfg.Analysis.Concurrency != 2 {
		t.Errorf("Analysis.Concurrency = %d, want 2", cfg.Analysis.Concurrency)
	}
}

func TestLoad_OverridesDefaults(t *testing.T) {
	path := writeTempConfig(t, `
server:
  host: "127.0.0.1"
  port: 9090
database:
  url: "postgres://localhost/encoder"
logging:
  level: "debug"
  format: "text"
agent:
  dispatch_interval: "5s"
  heartbeat_timeout: "60s"
  script_base_dir: "/var/scripts"
  task_timeout_sec: 7200
webhooks:
  worker_count: 8
  max_retries: 5
analysis:
  concurrency: 4
  ffmpeg_bin: "/usr/local/bin/ffmpeg"
  ffprobe_bin: "/usr/local/bin/ffprobe"
  dovi_tool_bin: "/usr/local/bin/dovi_tool"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "127.0.0.1")
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %q, want debug", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("Logging.Format = %q, want text", cfg.Logging.Format)
	}
	if cfg.Agent.DispatchInterval != 5*time.Second {
		t.Errorf("Agent.DispatchInterval = %v, want 5s", cfg.Agent.DispatchInterval)
	}
	if cfg.Agent.HeartbeatTimeout != 60*time.Second {
		t.Errorf("Agent.HeartbeatTimeout = %v, want 60s", cfg.Agent.HeartbeatTimeout)
	}
	if cfg.Agent.ScriptBaseDir != "/var/scripts" {
		t.Errorf("Agent.ScriptBaseDir = %q, want /var/scripts", cfg.Agent.ScriptBaseDir)
	}
	if cfg.Agent.TaskTimeoutSec != 7200 {
		t.Errorf("Agent.TaskTimeoutSec = %d, want 7200", cfg.Agent.TaskTimeoutSec)
	}
	if cfg.Webhooks.WorkerCount != 8 {
		t.Errorf("Webhooks.WorkerCount = %d, want 8", cfg.Webhooks.WorkerCount)
	}
	if cfg.Webhooks.MaxRetries != 5 {
		t.Errorf("Webhooks.MaxRetries = %d, want 5", cfg.Webhooks.MaxRetries)
	}
	if cfg.Analysis.Concurrency != 4 {
		t.Errorf("Analysis.Concurrency = %d, want 4", cfg.Analysis.Concurrency)
	}
	if cfg.Analysis.FFmpegBin != "/usr/local/bin/ffmpeg" {
		t.Errorf("Analysis.FFmpegBin = %q", cfg.Analysis.FFmpegBin)
	}
	if cfg.Analysis.FFprobeBin != "/usr/local/bin/ffprobe" {
		t.Errorf("Analysis.FFprobeBin = %q", cfg.Analysis.FFprobeBin)
	}
	if cfg.Analysis.DoviToolBin != "/usr/local/bin/dovi_tool" {
		t.Errorf("Analysis.DoviToolBin = %q", cfg.Analysis.DoviToolBin)
	}
}

func TestLoad_GRPCSection(t *testing.T) {
	path := writeTempConfig(t, `
database:
  url: "postgres://localhost/encoder"
grpc:
  host: "0.0.0.0"
  port: 9444
  tls:
    cert: "/etc/certs/server.crt"
    key:  "/etc/certs/server.key"
    ca:   "/etc/certs/ca.crt"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.GRPC.Port != 9444 {
		t.Errorf("GRPC.Port = %d, want 9444", cfg.GRPC.Port)
	}
	if cfg.GRPC.TLS.CertFile != "/etc/certs/server.crt" {
		t.Errorf("GRPC.TLS.CertFile = %q", cfg.GRPC.TLS.CertFile)
	}
}

func TestLoad_OIDCSection(t *testing.T) {
	path := writeTempConfig(t, `
database:
  url: "postgres://localhost/encoder"
auth:
  session_secret: "super-secret"
  oidc:
    enabled: true
    provider_url: "https://accounts.google.com"
    client_id: "my-client-id"
    client_secret: "my-client-secret"
    redirect_url: "https://myapp.example.com/auth/callback"
    auto_provision: true
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Auth.OIDC.Enabled {
		t.Error("Auth.OIDC.Enabled = false, want true")
	}
	if cfg.Auth.OIDC.ProviderURL != "https://accounts.google.com" {
		t.Errorf("Auth.OIDC.ProviderURL = %q", cfg.Auth.OIDC.ProviderURL)
	}
	if cfg.Auth.OIDC.ClientID != "my-client-id" {
		t.Errorf("Auth.OIDC.ClientID = %q", cfg.Auth.OIDC.ClientID)
	}
	if !cfg.Auth.OIDC.AutoProvision {
		t.Error("Auth.OIDC.AutoProvision = false, want true")
	}
}

func TestLoad_AnalysisPathMappings(t *testing.T) {
	path := writeTempConfig(t, `
database:
  url: "postgres://localhost/encoder"
analysis:
  path_mappings:
    - name: "NAS media"
      windows: "\\\\NAS01\\media"
      linux: "/mnt/nas/media"
    - name: "NAS backup"
      windows: "\\\\NAS01\\backup"
      linux: "/mnt/nas/backup"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Analysis.PathMappings) != 2 {
		t.Fatalf("PathMappings len = %d, want 2", len(cfg.Analysis.PathMappings))
	}
	if cfg.Analysis.PathMappings[0].Name != "NAS media" {
		t.Errorf("PathMappings[0].Name = %q, want %q", cfg.Analysis.PathMappings[0].Name, "NAS media")
	}
	if cfg.Analysis.PathMappings[0].Linux != "/mnt/nas/media" {
		t.Errorf("PathMappings[0].Linux = %q", cfg.Analysis.PathMappings[0].Linux)
	}
}

func TestLoad_UpgradeSection(t *testing.T) {
	path := writeTempConfig(t, `
database:
  url: "postgres://localhost/encoder"
upgrade:
  bin_dir: "/opt/encoder/agents"
  version: "2.3.4"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Upgrade.BinDir != "/opt/encoder/agents" {
		t.Errorf("Upgrade.BinDir = %q, want %q", cfg.Upgrade.BinDir, "/opt/encoder/agents")
	}
	if cfg.Upgrade.Version != "2.3.4" {
		t.Errorf("Upgrade.Version = %q, want %q", cfg.Upgrade.Version, "2.3.4")
	}
}

func TestLoad_VNCSection(t *testing.T) {
	path := writeTempConfig(t, `
database:
  url: "postgres://localhost/encoder"
vnc:
  novnc_base_url: "http://controller:8080/novnc-assets"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.VNC.NoVNCBaseURL != "http://controller:8080/novnc-assets" {
		t.Errorf("VNC.NoVNCBaseURL = %q", cfg.VNC.NoVNCBaseURL)
	}
}

func TestLoad_DefaultVNCBaseURL(t *testing.T) {
	path := writeTempConfig(t, `database:
  url: "postgres://localhost/encoder"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	const wantURL = "https://unpkg.com/@novnc/novnc@1.5.0"
	if cfg.VNC.NoVNCBaseURL != wantURL {
		t.Errorf("VNC.NoVNCBaseURL default = %q, want %q", cfg.VNC.NoVNCBaseURL, wantURL)
	}
}

// ---------------------------------------------------------------------------
// Load — error paths
// ---------------------------------------------------------------------------

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeTempConfig(t, `this: is: not: valid: yaml: [unclosed`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	// An empty YAML file is technically valid; all values should be defaults.
	path := writeTempConfig(t, ``)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() empty file error = %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port default = %d, want 8080", cfg.Server.Port)
	}
}

func TestLoad_AgentAutoApprove(t *testing.T) {
	path := writeTempConfig(t, `
database:
  url: "postgres://localhost/encoder"
agent:
  auto_approve: true
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Agent.AutoApprove {
		t.Error("Agent.AutoApprove = false, want true")
	}
}

func TestLoad_LogRetentionDefaults(t *testing.T) {
	path := writeTempConfig(t, `database:
  url: "postgres://localhost/encoder"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Logging.TaskLogRetention != 720*time.Hour {
		t.Errorf("TaskLogRetention = %v, want 720h", cfg.Logging.TaskLogRetention)
	}
	if cfg.Logging.TaskLogMaxLinesPerJob != 500000 {
		t.Errorf("TaskLogMaxLinesPerJob = %d, want 500000", cfg.Logging.TaskLogMaxLinesPerJob)
	}
}

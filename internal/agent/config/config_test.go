package config

import (
	"os"
	"testing"
	"time"
)

// writeTempAgentConfig writes the given YAML content to a temp file and
// returns the path.
func writeTempAgentConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "agent-config-*.yaml")
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
	path := writeTempAgentConfig(t, `
controller:
  address: "controller.example.com:9443"
agent:
  work_dir: "C:\\encoder\\work"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Controller.Address != "controller.example.com:9443" {
		t.Errorf("Controller.Address = %q, want %q", cfg.Controller.Address, "controller.example.com:9443")
	}
}

func TestLoad_DefaultValues(t *testing.T) {
	path := writeTempAgentConfig(t, `
controller:
  address: "controller:9443"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Reconnect defaults.
	if cfg.Controller.Reconnect.InitialDelay != 5*time.Second {
		t.Errorf("Reconnect.InitialDelay = %v, want 5s", cfg.Controller.Reconnect.InitialDelay)
	}
	if cfg.Controller.Reconnect.MaxDelay != 5*time.Minute {
		t.Errorf("Reconnect.MaxDelay = %v, want 5m", cfg.Controller.Reconnect.MaxDelay)
	}
	if cfg.Controller.Reconnect.Multiplier != 2.0 {
		t.Errorf("Reconnect.Multiplier = %v, want 2.0", cfg.Controller.Reconnect.Multiplier)
	}

	// Agent defaults.
	if cfg.Agent.HeartbeatInterval != 30*time.Second {
		t.Errorf("Agent.HeartbeatInterval = %v, want 30s", cfg.Agent.HeartbeatInterval)
	}
	if cfg.Agent.PollInterval != 10*time.Second {
		t.Errorf("Agent.PollInterval = %v, want 10s", cfg.Agent.PollInterval)
	}
	if !cfg.Agent.CleanupOnSuccess {
		t.Error("Agent.CleanupOnSuccess = false, want true")
	}
	if cfg.Agent.KeepFailedJobs != 10 {
		t.Errorf("Agent.KeepFailedJobs = %d, want 10", cfg.Agent.KeepFailedJobs)
	}

	// GPU defaults.
	if !cfg.GPU.Enabled {
		t.Error("GPU.Enabled = false, want true")
	}
	if cfg.GPU.MonitorInterval != 5*time.Second {
		t.Errorf("GPU.MonitorInterval = %v, want 5s", cfg.GPU.MonitorInterval)
	}

	// VNC defaults.
	if cfg.VNC.Enabled {
		t.Error("VNC.Enabled = true, want false")
	}
	if cfg.VNC.Port != 5900 {
		t.Errorf("VNC.Port = %d, want 5900", cfg.VNC.Port)
	}

	// Logging defaults.
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level = %q, want info", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Logging.Format = %q, want json", cfg.Logging.Format)
	}
	if cfg.Logging.MaxSizeMB != 100 {
		t.Errorf("Logging.MaxSizeMB = %d, want 100", cfg.Logging.MaxSizeMB)
	}
	if cfg.Logging.MaxBackups != 5 {
		t.Errorf("Logging.MaxBackups = %d, want 5", cfg.Logging.MaxBackups)
	}
	if !cfg.Logging.Compress {
		t.Error("Logging.Compress = false, want true")
	}
	if cfg.Logging.StreamBufferSize != 1000 {
		t.Errorf("Logging.StreamBufferSize = %d, want 1000", cfg.Logging.StreamBufferSize)
	}
	if cfg.Logging.StreamFlushInterval != 1*time.Second {
		t.Errorf("Logging.StreamFlushInterval = %v, want 1s", cfg.Logging.StreamFlushInterval)
	}
}

func TestLoad_FullConfig(t *testing.T) {
	path := writeTempAgentConfig(t, `
controller:
  address: "controller.corp.example.com:9443"
  tls:
    cert: "/etc/agent/client.crt"
    key:  "/etc/agent/client.key"
    ca:   "/etc/agent/ca.crt"
  reconnect:
    initial_delay: "2s"
    max_delay: "10m"
    multiplier: 1.5

agent:
  hostname: "WIN-ENCODE-01"
  work_dir:  "C:\\encoder\\work"
  log_dir:   "C:\\encoder\\logs"
  offline_db: "C:\\encoder\\offline.db"
  heartbeat_interval: "15s"
  poll_interval: "5s"
  cleanup_on_success: false
  keep_failed_jobs: 20

tools:
  ffmpeg:    "C:\\tools\\ffmpeg.exe"
  ffprobe:   "C:\\tools\\ffprobe.exe"
  x265:      "C:\\tools\\x265.exe"
  x264:      "C:\\tools\\x264.exe"
  svt_av1:   "C:\\tools\\SvtAv1EncApp.exe"
  avs_pipe:  "C:\\tools\\avspipe.exe"
  vspipe:    "C:\\tools\\vspipe.exe"
  dovi_tool: "C:\\tools\\dovi_tool.exe"

gpu:
  enabled: true
  vendor: "nvidia"
  max_vram_mb: 8192
  monitor_interval: "10s"

vnc:
  enabled: true
  port: 5901
  password: "vnc-secret"
  installer_url: "http://controller:8080/tightvnc.msi"

allowed_shares:
  - "\\\\NAS01\\media"
  - "\\\\NAS01\\backup"

logging:
  level: "debug"
  format: "text"
  max_size_mb: 50
  max_backups: 3
  compress: false
  stream_buffer_size: 500
  stream_flush_interval: "500ms"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Controller TLS.
	if cfg.Controller.TLS.Cert != "/etc/agent/client.crt" {
		t.Errorf("TLS.Cert = %q", cfg.Controller.TLS.Cert)
	}
	if cfg.Controller.TLS.CA != "/etc/agent/ca.crt" {
		t.Errorf("TLS.CA = %q", cfg.Controller.TLS.CA)
	}

	// Reconnect overrides.
	if cfg.Controller.Reconnect.InitialDelay != 2*time.Second {
		t.Errorf("Reconnect.InitialDelay = %v, want 2s", cfg.Controller.Reconnect.InitialDelay)
	}
	if cfg.Controller.Reconnect.MaxDelay != 10*time.Minute {
		t.Errorf("Reconnect.MaxDelay = %v, want 10m", cfg.Controller.Reconnect.MaxDelay)
	}
	if cfg.Controller.Reconnect.Multiplier != 1.5 {
		t.Errorf("Reconnect.Multiplier = %v, want 1.5", cfg.Controller.Reconnect.Multiplier)
	}

	// Agent fields.
	if cfg.Agent.Hostname != "WIN-ENCODE-01" {
		t.Errorf("Agent.Hostname = %q, want WIN-ENCODE-01", cfg.Agent.Hostname)
	}
	if cfg.Agent.HeartbeatInterval != 15*time.Second {
		t.Errorf("Agent.HeartbeatInterval = %v, want 15s", cfg.Agent.HeartbeatInterval)
	}
	if cfg.Agent.PollInterval != 5*time.Second {
		t.Errorf("Agent.PollInterval = %v, want 5s", cfg.Agent.PollInterval)
	}
	if cfg.Agent.CleanupOnSuccess {
		t.Error("Agent.CleanupOnSuccess = true, want false")
	}
	if cfg.Agent.KeepFailedJobs != 20 {
		t.Errorf("Agent.KeepFailedJobs = %d, want 20", cfg.Agent.KeepFailedJobs)
	}
	if cfg.Agent.OfflineDB != "C:\\encoder\\offline.db" {
		t.Errorf("Agent.OfflineDB = %q", cfg.Agent.OfflineDB)
	}

	// Tools.
	if cfg.Tools.FFmpeg != "C:\\tools\\ffmpeg.exe" {
		t.Errorf("Tools.FFmpeg = %q", cfg.Tools.FFmpeg)
	}
	if cfg.Tools.X265 != "C:\\tools\\x265.exe" {
		t.Errorf("Tools.X265 = %q", cfg.Tools.X265)
	}
	if cfg.Tools.DoviTool != "C:\\tools\\dovi_tool.exe" {
		t.Errorf("Tools.DoviTool = %q", cfg.Tools.DoviTool)
	}

	// GPU.
	if !cfg.GPU.Enabled {
		t.Error("GPU.Enabled = false, want true")
	}
	if cfg.GPU.Vendor != "nvidia" {
		t.Errorf("GPU.Vendor = %q, want nvidia", cfg.GPU.Vendor)
	}
	if cfg.GPU.MaxVRAMMB != 8192 {
		t.Errorf("GPU.MaxVRAMMB = %d, want 8192", cfg.GPU.MaxVRAMMB)
	}
	if cfg.GPU.MonitorInterval != 10*time.Second {
		t.Errorf("GPU.MonitorInterval = %v, want 10s", cfg.GPU.MonitorInterval)
	}

	// VNC.
	if !cfg.VNC.Enabled {
		t.Error("VNC.Enabled = false, want true")
	}
	if cfg.VNC.Port != 5901 {
		t.Errorf("VNC.Port = %d, want 5901", cfg.VNC.Port)
	}
	if cfg.VNC.Password != "vnc-secret" {
		t.Errorf("VNC.Password = %q", cfg.VNC.Password)
	}

	// AllowedShares.
	if len(cfg.AllowedShares) != 2 {
		t.Errorf("AllowedShares len = %d, want 2", len(cfg.AllowedShares))
	}

	// Logging overrides.
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %q, want debug", cfg.Logging.Level)
	}
	if cfg.Logging.MaxSizeMB != 50 {
		t.Errorf("Logging.MaxSizeMB = %d, want 50", cfg.Logging.MaxSizeMB)
	}
	if cfg.Logging.Compress {
		t.Error("Logging.Compress = true, want false")
	}
	if cfg.Logging.StreamFlushInterval != 500*time.Millisecond {
		t.Errorf("Logging.StreamFlushInterval = %v, want 500ms", cfg.Logging.StreamFlushInterval)
	}
}

// ---------------------------------------------------------------------------
// Load — error paths
// ---------------------------------------------------------------------------

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/agent.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeTempAgentConfig(t, `this: is: not: valid: yaml: [unclosed`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	// Empty YAML is valid; all values should be defaults.
	path := writeTempAgentConfig(t, ``)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() empty file error = %v", err)
	}
	if cfg.VNC.Port != 5900 {
		t.Errorf("VNC.Port default = %d, want 5900", cfg.VNC.Port)
	}
}

func TestLoad_GPUDisabled(t *testing.T) {
	path := writeTempAgentConfig(t, `
gpu:
  enabled: false
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.GPU.Enabled {
		t.Error("GPU.Enabled = true, want false")
	}
}

// TestLoad_VNCEnabledWithPassword verifies that VNC config fields round-trip
// correctly, exercising the VNCConfig mapstructure decoder.
func TestLoad_VNCEnabledWithPassword(t *testing.T) {
	path := writeTempAgentConfig(t, `
vnc:
  enabled: true
  port: 5902
  password: "s3cr3t"
  installer_url: "http://example.com/tightvnc.msi"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.VNC.Enabled {
		t.Error("VNC.Enabled = false, want true")
	}
	if cfg.VNC.Port != 5902 {
		t.Errorf("VNC.Port = %d, want 5902", cfg.VNC.Port)
	}
	if cfg.VNC.Password != "s3cr3t" {
		t.Errorf("VNC.Password = %q", cfg.VNC.Password)
	}
	if cfg.VNC.InstallerURL != "http://example.com/tightvnc.msi" {
		t.Errorf("VNC.InstallerURL = %q", cfg.VNC.InstallerURL)
	}
}

// TestLoad_AllowedSharesField verifies that the allowed_shares list is decoded
// correctly from YAML.
func TestLoad_AllowedSharesField(t *testing.T) {
	path := writeTempAgentConfig(t, `
allowed_shares:
  - "\\\\NAS01\\media"
  - "\\\\NAS02\\archive"
  - "\\\\NAS03\\backup"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.AllowedShares) != 3 {
		t.Errorf("AllowedShares len = %d, want 3", len(cfg.AllowedShares))
	}
}

// TestLoad_ToolsAllFields verifies all tools config fields are decoded.
func TestLoad_ToolsAllFields(t *testing.T) {
	path := writeTempAgentConfig(t, `
tools:
  ffmpeg:    "/usr/bin/ffmpeg"
  ffprobe:   "/usr/bin/ffprobe"
  x265:      "/usr/local/bin/x265"
  x264:      "/usr/local/bin/x264"
  svt_av1:   "/usr/local/bin/SvtAv1EncApp"
  avs_pipe:  "/usr/local/bin/avspipe"
  vspipe:    "/usr/local/bin/vspipe"
  dovi_tool: "/usr/local/bin/dovi_tool"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Tools.FFmpeg != "/usr/bin/ffmpeg" {
		t.Errorf("Tools.FFmpeg = %q", cfg.Tools.FFmpeg)
	}
	if cfg.Tools.FFprobe != "/usr/bin/ffprobe" {
		t.Errorf("Tools.FFprobe = %q", cfg.Tools.FFprobe)
	}
	if cfg.Tools.SvtAv1 != "/usr/local/bin/SvtAv1EncApp" {
		t.Errorf("Tools.SvtAv1 = %q", cfg.Tools.SvtAv1)
	}
	if cfg.Tools.AvsPipe != "/usr/local/bin/avspipe" {
		t.Errorf("Tools.AvsPipe = %q", cfg.Tools.AvsPipe)
	}
	if cfg.Tools.VSPipe != "/usr/local/bin/vspipe" {
		t.Errorf("Tools.VSPipe = %q", cfg.Tools.VSPipe)
	}
	if cfg.Tools.DoviTool != "/usr/local/bin/dovi_tool" {
		t.Errorf("Tools.DoviTool = %q", cfg.Tools.DoviTool)
	}
}

// TestLoad_LogDirAndOfflineDB verifies that Agent sub-fields log_dir and
// offline_db are decoded properly.
func TestLoad_LogDirAndOfflineDB(t *testing.T) {
	path := writeTempAgentConfig(t, `
agent:
  work_dir:   "/tmp/work"
  log_dir:    "/tmp/logs"
  offline_db: "/tmp/offline.db"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Agent.WorkDir != "/tmp/work" {
		t.Errorf("Agent.WorkDir = %q", cfg.Agent.WorkDir)
	}
	if cfg.Agent.LogDir != "/tmp/logs" {
		t.Errorf("Agent.LogDir = %q", cfg.Agent.LogDir)
	}
	if cfg.Agent.OfflineDB != "/tmp/offline.db" {
		t.Errorf("Agent.OfflineDB = %q", cfg.Agent.OfflineDB)
	}
}

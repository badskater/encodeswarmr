package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config is the root controller configuration.
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	GRPC     GRPCConfig     `mapstructure:"grpc"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Agent    AgentConfig    `mapstructure:"agent"`
	Webhooks WebhooksConfig `mapstructure:"webhooks"`
	TLS      TLSConfig      `mapstructure:"tls"`
	Upgrade  UpgradeConfig  `mapstructure:"upgrade"`
	VNC      VNCConfig      `mapstructure:"vnc"`
	Analysis AnalysisConfig `mapstructure:"analysis"`
}

type ServerConfig struct {
	Host           string        `mapstructure:"host"`
	Port           int           `mapstructure:"port"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	AllowedOrigins []string      `mapstructure:"allowed_origins"`
}

type DatabaseConfig struct {
	URL             string        `mapstructure:"url"`
	MaxConns        int           `mapstructure:"max_conns"`
	MinConns        int           `mapstructure:"min_conns"`
	MaxConnLifetime time.Duration `mapstructure:"max_conn_lifetime"`
	MigrationsPath  string        `mapstructure:"migrations_path"`
}

type GRPCConfig struct {
	Host string    `mapstructure:"host"`
	Port int       `mapstructure:"port"`
	TLS  TLSConfig `mapstructure:"tls"`
}

type AuthConfig struct {
	SessionTTL    time.Duration `mapstructure:"session_ttl"`
	SessionSecret string        `mapstructure:"session_secret"`
	OIDC          OIDCConfig    `mapstructure:"oidc"`
}

type OIDCConfig struct {
	Enabled       bool              `mapstructure:"enabled"`
	ProviderURL   string            `mapstructure:"provider_url"`
	ClientID      string            `mapstructure:"client_id"`
	ClientSecret  string            `mapstructure:"client_secret"`
	RedirectURL   string            `mapstructure:"redirect_url"`
	AutoProvision bool              `mapstructure:"auto_provision"`
	// GroupsClaim is the JWT claim that contains the user's group memberships.
	// Defaults to "groups".
	GroupsClaim   string            `mapstructure:"groups_claim"`
	// RoleMappings maps OIDC group names to internal roles
	// (e.g. {"encoding-admins": "admin", "encoding-viewers": "viewer"}).
	// The highest-privilege matching role is used.
	RoleMappings  map[string]string `mapstructure:"role_mappings"`
	// DefaultRole is the role assigned when no group mapping matches.
	// Defaults to "viewer".
	DefaultRole   string            `mapstructure:"default_role"`
}

type LoggingConfig struct {
	Level                  string        `mapstructure:"level"`
	Format                 string        `mapstructure:"format"`
	TaskLogRetention       time.Duration `mapstructure:"task_log_retention"`
	TaskLogCleanupInterval time.Duration `mapstructure:"task_log_cleanup_interval"`
	TaskLogMaxLinesPerJob  int           `mapstructure:"task_log_max_lines_per_job"`
}

type AgentConfig struct {
	AutoApprove      bool          `mapstructure:"auto_approve"`
	HeartbeatTimeout time.Duration `mapstructure:"heartbeat_timeout"`
	DispatchInterval time.Duration `mapstructure:"dispatch_interval"`
	ScriptBaseDir    string        `mapstructure:"script_base_dir"`
	TaskTimeoutSec   int           `mapstructure:"task_timeout_sec"`
}

type WebhooksConfig struct {
	WorkerCount     int           `mapstructure:"worker_count"`
	DeliveryTimeout time.Duration `mapstructure:"delivery_timeout"`
	MaxRetries      int           `mapstructure:"max_retries"`
}

type TLSConfig struct {
	CertFile string `mapstructure:"cert"`
	KeyFile  string `mapstructure:"key"`
	CAFile   string `mapstructure:"ca"`
}

type UpgradeConfig struct {
	// BinDir is the directory containing agent binaries.
	// Files should be named: agent-{os}-{arch}[.exe]
	// e.g. agent-windows-amd64.exe, agent-linux-amd64
	BinDir  string `mapstructure:"bin_dir"`
	// Version is the current agent version string, e.g. "1.2.3"
	Version string `mapstructure:"version"`
}

// AnalysisConfig controls controller-side analysis execution (HDR detect,
// scene scanning, VMAF, audio encoding).  When ffmpeg_bin / ffprobe_bin are
// empty the binaries are looked up on PATH.  If neither is available and no
// path_mappings are configured, analysis jobs fall back to agent dispatch.
type AnalysisConfig struct {
	// FFmpegBin is the path to the ffmpeg binary. Defaults to "ffmpeg" (PATH).
	FFmpegBin string `mapstructure:"ffmpeg_bin"`
	// FFprobeBin is the path to the ffprobe binary. Defaults to "ffprobe" (PATH).
	FFprobeBin string `mapstructure:"ffprobe_bin"`
	// DoviToolBin is the optional path to dovi_tool for DV profile detection.
	DoviToolBin string `mapstructure:"dovi_tool_bin"`
	// Concurrency limits simultaneous controller-side analysis processes.
	// 0 = unlimited.
	Concurrency int `mapstructure:"concurrency"`
	// PathMappings seeds the DB with path mappings on startup.
	// Use the API/UI to manage them at runtime.
	PathMappings []PathMappingConfig `mapstructure:"path_mappings"`
}

// PathMappingConfig is a single UNC → Linux path mapping in the config file.
type PathMappingConfig struct {
	// Name is a human-readable label for this mapping.
	Name string `mapstructure:"name"`
	// Windows is the Windows UNC prefix, e.g. \\NAS01\media
	Windows string `mapstructure:"windows"`
	// Linux is the Linux mount prefix, e.g. /mnt/nas/media
	Linux string `mapstructure:"linux"`
}

// VNCConfig controls the web-based VNC remote desktop feature.
type VNCConfig struct {
	// NoVNCBaseURL is the base URL from which the noVNC JavaScript client
	// library is loaded. Defaults to the unpkg CDN. Set this to a local
	// path or URL for air-gapped deployments.
	// Example: "https://unpkg.com/@novnc/novnc@1.5.0"
	// Example (local): "http://controller:8080/novnc-assets"
	NoVNCBaseURL string `mapstructure:"novnc_base_url"`
}

// Load reads the YAML config file at path and returns a populated Config.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "30s")
	v.SetDefault("grpc.host", "0.0.0.0")
	v.SetDefault("grpc.port", 9443)
	v.SetDefault("auth.session_ttl", "24h")
	v.SetDefault("auth.oidc.groups_claim", "groups")
	v.SetDefault("auth.oidc.default_role", "viewer")
	v.SetDefault("agent.dispatch_interval", "10s")
	v.SetDefault("agent.heartbeat_timeout", "90s")
	v.SetDefault("agent.script_base_dir", "/tmp/encoder-scripts")
	v.SetDefault("agent.task_timeout_sec", 3600)
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.task_log_retention", "720h")
	v.SetDefault("logging.task_log_cleanup_interval", "6h")
	v.SetDefault("logging.task_log_max_lines_per_job", 500000)
	v.SetDefault("webhooks.worker_count", 4)
	v.SetDefault("webhooks.delivery_timeout", "10s")
	v.SetDefault("webhooks.max_retries", 3)
	v.SetDefault("upgrade.bin_dir", "/var/lib/distributed-encoder/agent-bins")
	v.SetDefault("upgrade.version", "0.0.0")
	v.SetDefault("vnc.novnc_base_url", "https://unpkg.com/@novnc/novnc@1.5.0")
	v.SetDefault("analysis.ffmpeg_bin", "")
	v.SetDefault("analysis.ffprobe_bin", "")
	v.SetDefault("analysis.dovi_tool_bin", "")
	v.SetDefault("analysis.concurrency", 2)

	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}
	return &cfg, nil
}

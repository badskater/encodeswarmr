package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config is the root controller configuration.
type Config struct {
	Server        ServerConfig        `mapstructure:"server"`
	Database      DatabaseConfig      `mapstructure:"database"`
	GRPC          GRPCConfig          `mapstructure:"grpc"`
	Auth          AuthConfig          `mapstructure:"auth"`
	Logging       LoggingConfig       `mapstructure:"logging"`
	Agent         AgentConfig         `mapstructure:"agent"`
	Webhooks      WebhooksConfig      `mapstructure:"webhooks"`
	TLS           TLSConfig           `mapstructure:"tls"`
	Upgrade       UpgradeConfig       `mapstructure:"upgrade"`
	VNC           VNCConfig           `mapstructure:"vnc"`
	Analysis      AnalysisConfig      `mapstructure:"analysis"`
	SMTP          SMTPConfig          `mapstructure:"smtp"`
	AutoScaling   AutoScalingConfig   `mapstructure:"auto_scaling"`
	Validation    ValidationConfig    `mapstructure:"validation"`
	Archive       ArchiveConfig       `mapstructure:"archive"`
	Tracing       TracingConfig       `mapstructure:"tracing"`
	WatchFolders  []WatchFolderConfig `mapstructure:"watch_folders"`
	FileManager   FileManagerConfig   `mapstructure:"file_manager"`
	Notifications NotificationsConfig `mapstructure:"notifications"`
	LogStream     LogStreamConfig     `mapstructure:"logstream"`
}

// LogStreamConfig controls connection limits for the per-task WebSocket log
// streaming endpoint (/api/v1/tasks/{id}/logs/stream).
type LogStreamConfig struct {
	// MaxPerUser is the maximum number of concurrent log-stream WebSocket
	// connections allowed per authenticated user ID.  0 disables the cap.
	// Default: 10.
	MaxPerUser int `mapstructure:"max_per_user"`
	// MaxPerIP is the maximum number of concurrent log-stream WebSocket
	// connections allowed per client IP address.  0 disables the cap.
	// Default: 20.
	MaxPerIP int `mapstructure:"max_per_ip"`
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
	// ThumbnailDir is the base directory where source preview thumbnails are
	// stored.  Each source gets a subdirectory named by its UUID.
	// Defaults to /var/lib/encodeswarmr/thumbnails.
	ThumbnailDir string `mapstructure:"thumbnail_dir"`
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

// SMTPConfig holds outbound email delivery settings.
// Password is sensitive and should be provided via environment variable or
// a secrets manager — never hardcoded in config files.
type SMTPConfig struct {
	// Host is the SMTP server hostname, e.g. "smtp.example.com".
	Host string `mapstructure:"host"`
	// Port is the SMTP server port. Common values: 25, 465 (SMTPS), 587 (STARTTLS).
	Port int `mapstructure:"port"`
	// Username is the SMTP authentication username.
	Username string `mapstructure:"username"`
	// Password is the SMTP authentication password.
	// Use the SMTP_PASSWORD environment variable to avoid storing it in the config file.
	Password string `mapstructure:"password"`
	// FromAddress is the RFC 5321 envelope sender address, e.g. "encodeswarmr@example.com".
	FromAddress string `mapstructure:"from_address"`
	// TLSEnabled controls whether the connection uses implicit TLS (port 465).
	TLSEnabled bool `mapstructure:"tls_enabled"`
	// STARTTLS controls whether the connection upgrades to TLS via STARTTLS (port 587).
	STARTTLS bool `mapstructure:"starttls"`
}

// AutoScalingConfig holds settings for the agent auto-scaling webhook hooks.
type AutoScalingConfig struct {
	// Enabled activates the auto-scaling checks in the engine loop.
	Enabled bool `mapstructure:"enabled"`
	// WebhookURL is the URL that receives scale_up / scale_down JSON payloads.
	WebhookURL string `mapstructure:"webhook_url"`
	// ScaleUpThreshold is the pending task count that triggers a scale-up call.
	// When pending tasks exceed this value and no agents are idle, a scale_up
	// event is fired.
	ScaleUpThreshold int `mapstructure:"scale_up_threshold"`
	// ScaleDownThreshold is the number of idle agents above which a scale_down
	// event is fired (subject to CooldownSeconds).
	ScaleDownThreshold int `mapstructure:"scale_down_threshold"`
	// CooldownSeconds is the minimum number of seconds between successive
	// scale events of the same type to avoid thrashing.
	CooldownSeconds int `mapstructure:"cooldown_seconds"`
}

// ValidationConfig controls post-encode output validation via ffprobe.
type ValidationConfig struct {
	// Enabled enables or disables post-encode validation. Default true.
	Enabled bool `mapstructure:"enabled"`
	// MinDurationRatio is the minimum acceptable output/source duration ratio.
	// Default 0.9 (output must be at least 90% of source duration).
	MinDurationRatio float64 `mapstructure:"min_duration_ratio"`
}

// ArchiveConfig controls the job history archival background job.
type ArchiveConfig struct {
	// Enabled enables or disables the archival job. Default true.
	Enabled bool `mapstructure:"enabled"`
	// RetentionDays is how many days a completed/failed job is kept in the
	// active jobs table before being moved to job_archive. Default 30.
	RetentionDays int `mapstructure:"retention_days"`
}

// WatchFolderConfig defines a folder to monitor for new media files.
// Watch folders trigger automatic source creation and analysis only — no
// encoding jobs are created automatically.
type WatchFolderConfig struct {
	// Name is a human-readable label for this watch folder.
	Name string `mapstructure:"name"`
	// Path is the Linux mount path the controller polls, e.g. /mnt/nas/incoming.
	Path string `mapstructure:"path"`
	// WindowsPath is the corresponding UNC path stored on Source records so
	// agents can access the file, e.g. \\NAS\incoming.
	WindowsPath string `mapstructure:"windows_path"`
	// FilePatterns is a list of glob patterns, e.g. ["*.mkv", "*.mp4"].
	// An empty list defaults to ["*.mkv", "*.mp4", "*.ts", "*.avi"].
	FilePatterns []string `mapstructure:"file_patterns"`
	// PollInterval controls how often the folder is scanned. Default 30s.
	PollInterval time.Duration `mapstructure:"poll_interval"`
	// AutoAnalyze, when true, schedules analysis + HDR detect for new files.
	AutoAnalyze bool `mapstructure:"auto_analyze"`
	// MoveAfterAnalysis is an optional category/tag to apply to the source
	// record after analysis jobs complete.
	MoveAfterAnalysis string `mapstructure:"move_after_analysis"`
	// Enabled controls whether this folder is actively polled.
	Enabled bool `mapstructure:"enabled"`
}
// FileManagerConfig controls the server-side file manager feature.
type FileManagerConfig struct {
	// AllowedPaths is the list of base directories the file manager may access.
	// Requests for paths outside these directories are rejected with 403.
	// Example: ["/mnt/nas/media", "/mnt/nas/output"]
	AllowedPaths []string `mapstructure:"allowed_paths"`
}

// NotificationsConfig holds settings for push notification channels.
type NotificationsConfig struct {
	Telegram TelegramConfig `mapstructure:"telegram"`
	Pushover PushoverConfig `mapstructure:"pushover"`
	Ntfy     NtfyConfig     `mapstructure:"ntfy"`
}

// TelegramConfig holds Telegram Bot API credentials.
// BotToken is sensitive — supply via environment variable TELEGRAM_BOT_TOKEN.
type TelegramConfig struct {
	// BotToken is the token obtained from @BotFather.
	BotToken string `mapstructure:"bot_token"`
	// ChatID is the target chat or channel ID (numeric or @username).
	ChatID string `mapstructure:"chat_id"`
}

// PushoverConfig holds Pushover application and user credentials.
type PushoverConfig struct {
	// AppToken is the Pushover application API token.
	AppToken string `mapstructure:"app_token"`
	// UserKey is the Pushover user/group key.
	UserKey string `mapstructure:"user_key"`
}

// NtfyConfig holds ntfy.sh (or self-hosted ntfy) settings.
type NtfyConfig struct {
	// Topic is the ntfy topic to publish to, e.g. "encodeswarmr-alerts".
	Topic string `mapstructure:"topic"`
	// ServerURL is the ntfy server base URL. Defaults to "https://ntfy.sh".
	ServerURL string `mapstructure:"server_url"`
	// Token is an optional Bearer token for private topics.
	Token string `mapstructure:"token"`
}

// TracingConfig controls OpenTelemetry distributed tracing.
type TracingConfig struct {
	// Enabled controls whether tracing is active.  Defaults to false.
	Enabled bool `mapstructure:"enabled"`
	// Endpoint is the OTLP receiver endpoint, e.g. "localhost:4317".
	// Ignored when Enabled is false.
	Endpoint string `mapstructure:"endpoint"`
	// SampleRate is a fraction in [0,1].  Defaults to 0.1 (10 % sampling).
	SampleRate float64 `mapstructure:"sample_rate"`
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
	v.SetDefault("upgrade.bin_dir", "/var/lib/encodeswarmr/agent-bins")
	v.SetDefault("upgrade.version", "0.0.0")
	v.SetDefault("vnc.novnc_base_url", "https://unpkg.com/@novnc/novnc@1.5.0")
	v.SetDefault("analysis.ffmpeg_bin", "")
	v.SetDefault("analysis.ffprobe_bin", "")
	v.SetDefault("analysis.dovi_tool_bin", "")
	v.SetDefault("analysis.concurrency", 2)
	v.SetDefault("analysis.thumbnail_dir", "/var/lib/encodeswarmr/thumbnails")
	v.SetDefault("smtp.port", 587)
	v.SetDefault("smtp.tls_enabled", false)
	v.SetDefault("smtp.starttls", true)
	v.SetDefault("auto_scaling.enabled", false)
	v.SetDefault("auto_scaling.scale_up_threshold", 10)
	v.SetDefault("auto_scaling.scale_down_threshold", 2)
	v.SetDefault("auto_scaling.cooldown_seconds", 300)
	v.SetDefault("validation.enabled", true)
	v.SetDefault("validation.min_duration_ratio", 0.9)
	v.SetDefault("archive.enabled", true)
	v.SetDefault("archive.retention_days", 30)
	v.SetDefault("tracing.enabled", false)
	v.SetDefault("tracing.endpoint", "localhost:4317")
	v.SetDefault("tracing.sample_rate", 0.1)
	v.SetDefault("logstream.max_per_user", 10)
	v.SetDefault("logstream.max_per_ip", 20)

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

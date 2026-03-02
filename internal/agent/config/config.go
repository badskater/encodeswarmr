package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config is the root agent configuration.
type Config struct {
	Controller    ControllerConfig `mapstructure:"controller"`
	Agent         AgentConfig      `mapstructure:"agent"`
	Tools         ToolsConfig      `mapstructure:"tools"`
	GPU           GPUConfig        `mapstructure:"gpu"`
	AllowedShares []string         `mapstructure:"allowed_shares"`
	Logging       LoggingConfig    `mapstructure:"logging"`
}

type ControllerConfig struct {
	Address   string          `mapstructure:"address"`
	TLS       TLSConfig       `mapstructure:"tls"`
	Reconnect ReconnectConfig `mapstructure:"reconnect"`
}

type TLSConfig struct {
	Cert string `mapstructure:"cert"`
	Key  string `mapstructure:"key"`
	CA   string `mapstructure:"ca"`
}

type ReconnectConfig struct {
	InitialDelay time.Duration `mapstructure:"initial_delay"`
	MaxDelay     time.Duration `mapstructure:"max_delay"`
	Multiplier   float64       `mapstructure:"multiplier"`
}

type AgentConfig struct {
	Hostname          string        `mapstructure:"hostname"`
	WorkDir           string        `mapstructure:"work_dir"`
	LogDir            string        `mapstructure:"log_dir"`
	OfflineDB         string        `mapstructure:"offline_db"`
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`
	PollInterval      time.Duration `mapstructure:"poll_interval"`
	CleanupOnSuccess  bool          `mapstructure:"cleanup_on_success"`
	KeepFailedJobs    int           `mapstructure:"keep_failed_jobs"`
}

type ToolsConfig struct {
	FFmpeg  string `mapstructure:"ffmpeg"`
	FFprobe string `mapstructure:"ffprobe"`
	X265    string `mapstructure:"x265"`
	X264    string `mapstructure:"x264"`
	SvtAv1  string `mapstructure:"svt_av1"`
	AvsPipe string `mapstructure:"avs_pipe"`
	VSPipe  string `mapstructure:"vspipe"`
}

type GPUConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	Vendor          string        `mapstructure:"vendor"`
	MaxVRAMMB       int           `mapstructure:"max_vram_mb"`
	MonitorInterval time.Duration `mapstructure:"monitor_interval"`
}

type LoggingConfig struct {
	Level               string        `mapstructure:"level"`
	Format              string        `mapstructure:"format"`
	MaxSizeMB           int           `mapstructure:"max_size_mb"`
	MaxBackups          int           `mapstructure:"max_backups"`
	Compress            bool          `mapstructure:"compress"`
	StreamBufferSize    int           `mapstructure:"stream_buffer_size"`
	StreamFlushInterval time.Duration `mapstructure:"stream_flush_interval"`
}

// Load reads the YAML config file at path and returns a populated Config.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	v.SetDefault("controller.reconnect.initial_delay", "5s")
	v.SetDefault("controller.reconnect.max_delay", "5m")
	v.SetDefault("controller.reconnect.multiplier", 2.0)
	v.SetDefault("agent.heartbeat_interval", "30s")
	v.SetDefault("agent.poll_interval", "10s")
	v.SetDefault("agent.cleanup_on_success", true)
	v.SetDefault("agent.keep_failed_jobs", 10)
	v.SetDefault("gpu.enabled", true)
	v.SetDefault("gpu.monitor_interval", "5s")
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.max_size_mb", 100)
	v.SetDefault("logging.max_backups", 5)
	v.SetDefault("logging.compress", true)
	v.SetDefault("logging.stream_buffer_size", 1000)
	v.SetDefault("logging.stream_flush_interval", "1s")

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

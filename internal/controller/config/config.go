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
}

type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
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
	Enabled       bool   `mapstructure:"enabled"`
	ProviderURL   string `mapstructure:"provider_url"`
	ClientID      string `mapstructure:"client_id"`
	ClientSecret  string `mapstructure:"client_secret"`
	RedirectURL   string `mapstructure:"redirect_url"`
	AutoProvision bool   `mapstructure:"auto_provision"`
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
	v.SetDefault("agent.dispatch_interval", "10s")
	v.SetDefault("agent.heartbeat_timeout", "90s")
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.task_log_retention", "720h")
	v.SetDefault("logging.task_log_cleanup_interval", "6h")
	v.SetDefault("logging.task_log_max_lines_per_job", 500000)
	v.SetDefault("webhooks.worker_count", 4)
	v.SetDefault("webhooks.delivery_timeout", "10s")
	v.SetDefault("webhooks.max_retries", 3)

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

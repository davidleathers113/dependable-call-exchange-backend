package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	Version     string `koanf:"version"`
	Environment string `koanf:"environment"`
	LogLevel    string `koanf:"log_level"`

	Server    ServerConfig    `koanf:"server"`
	Database  DatabaseConfig  `koanf:"database"`
	Redis     RedisConfig     `koanf:"redis"`
	Kafka     KafkaConfig     `koanf:"kafka"`
	Telemetry TelemetryConfig `koanf:"telemetry"`

	Telephony  TelephonyConfig  `koanf:"telephony"`
	Compliance ComplianceConfig `koanf:"compliance"`
	Security   SecurityConfig   `koanf:"security"`
	CORS       CORSConfig       `koanf:"cors"`
}

type ServerConfig struct {
	Port            int           `koanf:"port"`
	Address         string        `koanf:"address"` // Full address like :8080
	ReadTimeout     time.Duration `koanf:"read_timeout"`
	WriteTimeout    time.Duration `koanf:"write_timeout"`
	IdleTimeout     time.Duration `koanf:"idle_timeout"`
	ShutdownTimeout time.Duration `koanf:"shutdown_timeout"`

	// Computed fields
	ReadTimeoutSeconds  int `koanf:"-"`
	WriteTimeoutSeconds int `koanf:"-"`
	IdleTimeoutSeconds  int `koanf:"-"`

	GRPC GRPCConfig `koanf:"grpc"`
}

type GRPCConfig struct {
	Port int `koanf:"port"`
}

type DatabaseConfig struct {
	URL             string        `koanf:"url"`
	MaxOpenConns    int           `koanf:"max_open_conns"`
	MaxIdleConns    int           `koanf:"max_idle_conns"`
	ConnMaxLifetime time.Duration `koanf:"conn_max_lifetime"`
}

type RedisConfig struct {
	URL          string        `koanf:"url"`
	Address      string        `koanf:"address"` // Alternative to URL
	Password     string        `koanf:"password"`
	DB           int           `koanf:"db"`
	PoolSize     int           `koanf:"pool_size"`
	MinIdleConns int           `koanf:"min_idle_conns"`
	MaxRetries   int           `koanf:"max_retries"`
	DialTimeout  time.Duration `koanf:"dial_timeout"`
	ReadTimeout  time.Duration `koanf:"read_timeout"`
	WriteTimeout time.Duration `koanf:"write_timeout"`
}

type KafkaConfig struct {
	Brokers []string `koanf:"brokers"`
	GroupID string   `koanf:"group_id"`
}

type TelemetryConfig struct {
	Enabled       bool          `koanf:"enabled"`
	OTLPEndpoint  string        `koanf:"otlp_endpoint"`
	SamplingRate  float64       `koanf:"sampling_rate"`
	ExportTimeout time.Duration `koanf:"export_timeout"`
	BatchTimeout  time.Duration `koanf:"batch_timeout"`
}

type TelephonyConfig struct {
	SIPProxy    string   `koanf:"sip_proxy"`
	STUNServers []string `koanf:"stun_servers"`
}

type ComplianceConfig struct {
	TCPAEnabled      bool     `koanf:"tcpa_enabled"`
	GDPREnabled      bool     `koanf:"gdpr_enabled"`
	AllowedTimeZones []string `koanf:"allowed_timezones"`
}

type SecurityConfig struct {
	JWTSecret               string          `koanf:"jwt_secret"`
	TokenExpiry             time.Duration   `koanf:"token_expiry"`
	RefreshTokenExpiry      time.Duration   `koanf:"refresh_token_expiry"`
	TokenExpiryMinutes      int             `koanf:"-"` // Computed field
	RefreshTokenExpiryDays  int             `koanf:"-"` // Computed field
	RateLimit               RateLimitConfig `koanf:"rate_limit"`
}

type RateLimitConfig struct {
	RequestsPerSecond int `koanf:"requests_per_second"`
	Burst             int `koanf:"burst"` // Alternative name
	BurstSize         int `koanf:"burst_size"`
}

type CORSConfig struct {
	AllowedOrigins []string `koanf:"allowed_origins"`
	AllowedMethods []string `koanf:"allowed_methods"`
	AllowedHeaders []string `koanf:"allowed_headers"`
	MaxAge         int      `koanf:"max_age"`
}

// Load loads configuration from various sources
func Load(configPath ...string) (*Config, error) {
	k := koanf.New(".")

	// Load defaults
	defaults := &Config{
		Version:     "dev",
		Environment: "development",
		LogLevel:    "info",
		Server: ServerConfig{
			Port:            8080,
			Address:         ":8080",
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			IdleTimeout:     120 * time.Second,
			ShutdownTimeout: 30 * time.Second,
			GRPC: GRPCConfig{
				Port: 9090,
			},
		},
		Database: DatabaseConfig{
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
		},
		Redis: RedisConfig{
			URL:          "redis://localhost:6379",
			Address:      "localhost:6379",
			DB:           0,
			PoolSize:     10,
			MinIdleConns: 2,
			MaxRetries:   3,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		},
		Telemetry: TelemetryConfig{
			Enabled:       true,
			OTLPEndpoint:  "http://localhost:4317",
			SamplingRate:  0.1, // Sample 10% of traces in development
			ExportTimeout: 10 * time.Second,
			BatchTimeout:  5 * time.Second,
		},
		Compliance: ComplianceConfig{
			TCPAEnabled: true,
			GDPREnabled: true,
		},
		Security: SecurityConfig{
			JWTSecret:          "change-me-in-production",
			TokenExpiry:        24 * time.Hour,
			RefreshTokenExpiry: 7 * 24 * time.Hour,
			RateLimit: RateLimitConfig{
				RequestsPerSecond: 100,
				Burst:             200,
				BurstSize:         200,
			},
		},
		CORS: CORSConfig{
			AllowedOrigins: []string{"http://localhost:3000", "http://localhost:8080"},
			AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Content-Type", "Authorization", "X-Request-ID"},
			MaxAge:         86400,
		},
	}

	if err := k.Load(structs.Provider(defaults, "koanf"), nil); err != nil {
		return nil, fmt.Errorf("loading defaults: %w", err)
	}

	// Load from config file if exists
	cfgPath := "configs/config.yaml"
	if len(configPath) > 0 && configPath[0] != "" {
		cfgPath = configPath[0]
	}
	if err := k.Load(file.Provider(cfgPath), yaml.Parser()); err != nil {
		// Config file is optional, only log if it's not a "file not found" error
	}

	// Override with environment variables
	if err := k.Load(env.Provider("DCE_", ".", func(s string) string {
		return strings.Replace(strings.ToLower(
			strings.TrimPrefix(s, "DCE_")), "_", ".", -1)
	}), nil); err != nil {
		return nil, fmt.Errorf("loading environment variables: %w", err)
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Post-process configuration
	cfg.postProcess()

	return &cfg, nil
}

// postProcess computes derived fields after loading
func (c *Config) postProcess() {
	// Compute server address if not set
	if c.Server.Address == "" {
		c.Server.Address = fmt.Sprintf(":%d", c.Server.Port)
	}

	// Compute timeout seconds
	c.Server.ReadTimeoutSeconds = int(c.Server.ReadTimeout.Seconds())
	c.Server.WriteTimeoutSeconds = int(c.Server.WriteTimeout.Seconds())
	c.Server.IdleTimeoutSeconds = int(c.Server.IdleTimeout.Seconds())

	// Compute Redis address from URL if needed
	if c.Redis.Address == "" && c.Redis.URL != "" {
		// Extract host:port from redis://host:port format
		if strings.HasPrefix(c.Redis.URL, "redis://") {
			c.Redis.Address = strings.TrimPrefix(c.Redis.URL, "redis://")
		} else {
			c.Redis.Address = c.Redis.URL
		}
	}

	// Compute token expiry in different units
	c.Security.TokenExpiryMinutes = int(c.Security.TokenExpiry.Minutes())
	c.Security.RefreshTokenExpiryDays = int(c.Security.RefreshTokenExpiry.Hours() / 24)

	// Ensure RateLimit.Burst is set
	if c.Security.RateLimit.Burst == 0 && c.Security.RateLimit.BurstSize > 0 {
		c.Security.RateLimit.Burst = c.Security.RateLimit.BurstSize
	}
}

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

	Server   ServerConfig   `koanf:"server"`
	Database DatabaseConfig `koanf:"database"`
	Redis    RedisConfig    `koanf:"redis"`
	Kafka    KafkaConfig    `koanf:"kafka"`
	
	Telephony   TelephonyConfig   `koanf:"telephony"`
	Compliance  ComplianceConfig  `koanf:"compliance"`
	Security    SecurityConfig    `koanf:"security"`
}

type ServerConfig struct {
	Port            int           `koanf:"port"`
	ReadTimeout     time.Duration `koanf:"read_timeout"`
	WriteTimeout    time.Duration `koanf:"write_timeout"`
	ShutdownTimeout time.Duration `koanf:"shutdown_timeout"`
	
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
	URL      string `koanf:"url"`
	Password string `koanf:"password"`
	DB       int    `koanf:"db"`
}

type KafkaConfig struct {
	Brokers []string `koanf:"brokers"`
	GroupID string   `koanf:"group_id"`
}

type TelephonyConfig struct {
	SIPProxy  string `koanf:"sip_proxy"`
	STUNServers []string `koanf:"stun_servers"`
}

type ComplianceConfig struct {
	TCPAEnabled    bool     `koanf:"tcpa_enabled"`
	GDPREnabled    bool     `koanf:"gdpr_enabled"`
	AllowedTimeZones []string `koanf:"allowed_timezones"`
}

type SecurityConfig struct {
	JWTSecret     string        `koanf:"jwt_secret"`
	TokenExpiry   time.Duration `koanf:"token_expiry"`
	RateLimit     RateLimitConfig `koanf:"rate_limit"`
}

type RateLimitConfig struct {
	RequestsPerSecond int `koanf:"requests_per_second"`
	BurstSize         int `koanf:"burst_size"`
}

func Load() (*Config, error) {
	k := koanf.New(".")
	
	// Load defaults
	defaults := &Config{
		Version:     "dev",
		Environment: "development",
		LogLevel:    "info",
		Server: ServerConfig{
			Port:            8080,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
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
			DB: 0,
		},
		Compliance: ComplianceConfig{
			TCPAEnabled: true,
			GDPREnabled: true,
		},
		Security: SecurityConfig{
			TokenExpiry: 24 * time.Hour,
			RateLimit: RateLimitConfig{
				RequestsPerSecond: 100,
				BurstSize:         200,
			},
		},
	}

	if err := k.Load(structs.Provider(defaults, "koanf"), nil); err != nil {
		return nil, fmt.Errorf("loading defaults: %w", err)
	}

	// Load from config file if exists
	if err := k.Load(file.Provider("configs/config.yaml"), yaml.Parser()); err != nil {
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

	return &cfg, nil
}
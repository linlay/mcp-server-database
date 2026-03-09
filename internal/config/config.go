package config

import (
	"os"
	"strconv"
	"strings"
)

const (
	defaultTransportMode = "http"
	transportModeHTTP    = "http"
	transportModeStdio   = "stdio"
)

type ObservabilityConfig struct {
	LogEnabled        bool
	LogMaxBodyLength  int
	LogIncludeHeaders bool
}

type ServerConfig struct {
	Port                   int
	ShutdownTimeoutSeconds int
}

type RateLimitConfig struct {
	Enabled bool
	RPS     float64
	Burst   int
}

type MCPConfig struct {
	Transport                string
	ToolsSpecLocationPattern string
	HTTPMaxBodyBytes         int64
	RateLimit                RateLimitConfig
}

type DatabaseConfig struct {
	ConnectionsConfigPath      string
	DefaultQueryTimeoutSeconds int
	MaxResultRows              int
	MaxCellBytes               int
}

type Config struct {
	Server        ServerConfig
	MCP           MCPConfig
	Observability ObservabilityConfig
	Database      DatabaseConfig
}

func Load() Config {
	return Config{
		Server: ServerConfig{
			Port:                   readIntEnv("SERVER_PORT", 8080),
			ShutdownTimeoutSeconds: readIntEnv("SERVER_SHUTDOWN_TIMEOUT_SECONDS", 10),
		},
		MCP: MCPConfig{
			Transport:                readTransportEnv("MCP_TRANSPORT", defaultTransportMode),
			ToolsSpecLocationPattern: readStringEnv("MCP_TOOLS_SPEC_LOCATION_PATTERN", "./tools/*.yml"),
			HTTPMaxBodyBytes:         readInt64Env("MCP_HTTP_MAX_BODY_BYTES", 1024*1024),
			RateLimit: RateLimitConfig{
				Enabled: readBoolEnv("MCP_RATE_LIMIT_ENABLED", false),
				RPS:     readFloat64Env("MCP_RATE_LIMIT_RPS", 5),
				Burst:   readIntEnv("MCP_RATE_LIMIT_BURST", 10),
			},
		},
		Observability: ObservabilityConfig{
			LogEnabled:        readBoolEnv("MCP_OBSERVABILITY_LOG_ENABLED", true),
			LogMaxBodyLength:  readIntEnv("MCP_OBSERVABILITY_LOG_MAX_BODY_LENGTH", 2000),
			LogIncludeHeaders: readBoolEnv("MCP_OBSERVABILITY_LOG_INCLUDE_HEADERS", false),
		},
		Database: DatabaseConfig{
			ConnectionsConfigPath:      readStringEnv("DB_CONNECTIONS_CONFIG_PATH", "./configs"),
			DefaultQueryTimeoutSeconds: readIntEnv("DB_DEFAULT_QUERY_TIMEOUT_SECONDS", 15),
			MaxResultRows:              readIntEnv("DB_MAX_RESULT_ROWS", 200),
			MaxCellBytes:               readIntEnv("DB_MAX_CELL_BYTES", 4096),
		},
	}
}

func readTransportEnv(key string, fallback string) string {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case transportModeHTTP, transportModeStdio:
		return value
	default:
		return fallback
	}
}

func readStringEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func readIntEnv(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func readBoolEnv(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func readInt64Env(key string, fallback int64) int64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

func readFloat64Env(key string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

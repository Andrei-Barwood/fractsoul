package app

import (
	"os"
	"strings"
)

type Config struct {
	Port             string
	GinMode          string
	LogLevel         string
	NATSURL          string
	TelemetrySubject string
	DatabaseURL      string
	ProcessorEnabled bool
}

func LoadConfig() Config {
	return Config{
		Port:             getEnv("APP_PORT", "8080"),
		GinMode:          getEnv("GIN_MODE", "release"),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		NATSURL:          getEnv("NATS_URL", "nats://localhost:4222"),
		TelemetrySubject: getEnv("TELEMETRY_SUBJECT", "telemetry.raw.v1"),
		DatabaseURL:      getEnv("DATABASE_URL", ""),
		ProcessorEnabled: getEnvAsBool("TELEMETRY_PROCESSOR_ENABLED", true),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func getEnvAsBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}

	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

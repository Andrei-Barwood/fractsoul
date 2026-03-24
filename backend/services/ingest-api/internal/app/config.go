package app

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                string
	GinMode             string
	LogLevel            string
	NATSURL             string
	TelemetrySubject    string
	TelemetryStream     string
	TelemetryDLQSubject string
	ConsumerDurable     string
	DatabaseURL         string
	ProcessorEnabled    bool
	ProcessorMaxDeliver int
	ProcessorRetryDelay time.Duration
	IngestMaxBodyBytes  int64
}

func LoadConfig() Config {
	return Config{
		Port:                getEnv("APP_PORT", "8080"),
		GinMode:             getEnv("GIN_MODE", "release"),
		LogLevel:            getEnv("LOG_LEVEL", "info"),
		NATSURL:             getEnv("NATS_URL", "nats://localhost:4222"),
		TelemetrySubject:    getEnv("TELEMETRY_SUBJECT", "telemetry.raw.v1"),
		TelemetryStream:     getEnv("TELEMETRY_STREAM", "TELEMETRY"),
		TelemetryDLQSubject: getEnv("TELEMETRY_DLQ_SUBJECT", "telemetry.dlq.v1"),
		ConsumerDurable:     getEnv("TELEMETRY_CONSUMER_DURABLE", "telemetry-processor-v1"),
		DatabaseURL:         getEnv("DATABASE_URL", ""),
		ProcessorEnabled:    getEnvAsBool("TELEMETRY_PROCESSOR_ENABLED", true),
		ProcessorMaxDeliver: getEnvAsInt("PROCESSOR_MAX_DELIVER", 5),
		ProcessorRetryDelay: getEnvAsDuration("PROCESSOR_RETRY_DELAY", 2*time.Second),
		IngestMaxBodyBytes:  getEnvAsInt64("INGEST_MAX_BODY_BYTES", 1<<20),
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

func getEnvAsInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getEnvAsInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}

	return parsed
}

func getEnvAsDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return parsed
}

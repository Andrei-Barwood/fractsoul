package app

import "os"

type Config struct {
	Port             string
	GinMode          string
	LogLevel         string
	NATSURL          string
	TelemetrySubject string
}

func LoadConfig() Config {
	return Config{
		Port:             getEnv("APP_PORT", "8080"),
		GinMode:          getEnv("GIN_MODE", "release"),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		NATSURL:          getEnv("NATS_URL", "nats://localhost:4222"),
		TelemetrySubject: getEnv("TELEMETRY_SUBJECT", "telemetry.raw.v1"),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

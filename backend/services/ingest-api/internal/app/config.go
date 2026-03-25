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
	APIAuthEnabled      bool
	APIRBACEnabled      bool
	APIKeyHeader        string
	APIKeys             []string
	APIDefaultRole      string
	APIKeyRoles         map[string]string
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
	AlertsEnabled       bool
	AlertSuppressWindow time.Duration
	AlertNotifyTimeout  time.Duration
	AlertNotifyRetries  int
	AlertNotifyBackoff  time.Duration
	AlertQueueSize      int
	AlertWorkerCount    int
	AlertWebhookEnabled bool
	AlertWebhookURL     string
	AlertWebhookHeader  string
	AlertWebhookToken   string
	AlertEmailEnabled   bool
	AlertSMTPAddr       string
	AlertSMTPUsername   string
	AlertSMTPPassword   string
	AlertEmailFrom      string
	AlertEmailTo        []string
	AlertEmailSubject   string
}

func LoadConfig() Config {
	return Config{
		Port:                getEnv("APP_PORT", "8080"),
		GinMode:             getEnv("GIN_MODE", "release"),
		LogLevel:            getEnv("LOG_LEVEL", "info"),
		APIAuthEnabled:      getEnvAsBool("API_AUTH_ENABLED", false),
		APIRBACEnabled:      getEnvAsBool("API_RBAC_ENABLED", false),
		APIKeyHeader:        getEnv("API_KEY_HEADER", "X-API-Key"),
		APIKeys:             getEnvAsList("API_KEYS"),
		APIDefaultRole:      defaultAPIRole(getEnv("API_DEFAULT_ROLE", "admin")),
		APIKeyRoles:         getEnvAsKeyRoleMap("API_KEY_ROLES"),
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
		AlertsEnabled:       getEnvAsBool("ALERTS_ENABLED", true),
		AlertSuppressWindow: getEnvAsDuration("ALERT_SUPPRESS_WINDOW", 10*time.Minute),
		AlertNotifyTimeout:  getEnvAsDuration("ALERT_NOTIFY_TIMEOUT", 3*time.Second),
		AlertNotifyRetries:  getEnvAsInt("ALERT_NOTIFY_RETRIES", 3),
		AlertNotifyBackoff:  getEnvAsDuration("ALERT_NOTIFY_BACKOFF", 500*time.Millisecond),
		AlertQueueSize:      getEnvAsInt("ALERT_QUEUE_SIZE", 256),
		AlertWorkerCount:    getEnvAsInt("ALERT_WORKER_COUNT", 2),
		AlertWebhookEnabled: getEnvAsBool("ALERT_WEBHOOK_ENABLED", false),
		AlertWebhookURL:     getEnv("ALERT_WEBHOOK_URL", ""),
		AlertWebhookHeader:  getEnv("ALERT_WEBHOOK_HEADER", "Authorization"),
		AlertWebhookToken:   getEnv("ALERT_WEBHOOK_TOKEN", ""),
		AlertEmailEnabled:   getEnvAsBool("ALERT_EMAIL_ENABLED", false),
		AlertSMTPAddr:       getEnv("ALERT_SMTP_ADDR", ""),
		AlertSMTPUsername:   getEnv("ALERT_SMTP_USERNAME", ""),
		AlertSMTPPassword:   getEnv("ALERT_SMTP_PASSWORD", ""),
		AlertEmailFrom:      getEnv("ALERT_EMAIL_FROM", "alerts@fractsoul.local"),
		AlertEmailTo:        getEnvAsList("ALERT_EMAIL_TO"),
		AlertEmailSubject:   getEnv("ALERT_EMAIL_SUBJECT_PREFIX", "[Fractsoul Alert]"),
	}
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value != "" {
		return value
	}

	filePath := strings.TrimSpace(os.Getenv(key + "_FILE"))
	if filePath == "" {
		return fallback
	}

	contents, err := os.ReadFile(filePath)
	if err != nil {
		return fallback
	}

	value = strings.TrimSpace(string(contents))
	if value == "" {
		return fallback
	}

	return value
}

func getEnvAsBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(getEnv(key, "")))
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
	value := strings.TrimSpace(getEnv(key, ""))
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
	value := strings.TrimSpace(getEnv(key, ""))
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
	value := strings.TrimSpace(getEnv(key, ""))
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getEnvAsList(key string) []string {
	raw := strings.TrimSpace(getEnv(key, ""))
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}

	return values
}

func getEnvAsKeyRoleMap(key string) map[string]string {
	raw := strings.TrimSpace(getEnv(key, ""))
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	values := make(map[string]string, len(parts))
	for _, part := range parts {
		entry := strings.TrimSpace(part)
		if entry == "" {
			continue
		}

		tokens := strings.SplitN(entry, ":", 2)
		if len(tokens) != 2 {
			continue
		}

		apiKey := strings.TrimSpace(tokens[0])
		role := normalizeAPIRole(tokens[1])
		if apiKey == "" || role == "" {
			continue
		}
		values[apiKey] = role
	}

	if len(values) == 0 {
		return nil
	}

	return values
}

func normalizeAPIRole(value string) string {
	role := strings.TrimSpace(strings.ToLower(value))
	switch role {
	case "viewer", "operator", "admin":
		return role
	default:
		return ""
	}
}

func defaultAPIRole(value string) string {
	role := normalizeAPIRole(value)
	if role == "" {
		return "admin"
	}
	return role
}

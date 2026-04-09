package app

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                 string
	GinMode              string
	LogLevel             string
	DatabaseURL          string
	APIAuthEnabled       bool
	APIRBACEnabled       bool
	APIKeyHeader         string
	APIKeys              []string
	APIDefaultRole       string
	APIKeyRoles          map[string]string
	DefaultAt            string
	EventsEnabled        bool
	NATSURL              string
	EnergyStream         string
	FractsoulAPIBaseURL  string
	FractsoulAPIKey      string
	FractsoulAPITimeout  time.Duration
	ContextRackLimit     int
	ContextWindowMinutes int
}

func LoadConfig() Config {
	return Config{
		Port:                 getEnv("APP_PORT", "8081"),
		GinMode:              getEnv("GIN_MODE", "release"),
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		DatabaseURL:          getEnv("DATABASE_URL", ""),
		APIAuthEnabled:       getEnvAsBool("API_AUTH_ENABLED", false),
		APIRBACEnabled:       getEnvAsBool("API_RBAC_ENABLED", false),
		APIKeyHeader:         getEnv("API_KEY_HEADER", "X-API-Key"),
		APIKeys:              getEnvAsList("API_KEYS"),
		APIDefaultRole:       defaultAPIRole(getEnv("API_DEFAULT_ROLE", "admin")),
		APIKeyRoles:          getEnvAsKeyRoleMap("API_KEY_ROLES"),
		DefaultAt:            getEnv("ENERGY_DEFAULT_AT", "now"),
		EventsEnabled:        getEnvAsBool("ENERGY_EVENTS_ENABLED", true),
		NATSURL:              getEnv("NATS_URL", "nats://localhost:4222"),
		EnergyStream:         getEnv("ENERGY_STREAM", "ENERGY"),
		FractsoulAPIBaseURL:  getEnv("FRACTSOUL_API_BASE_URL", ""),
		FractsoulAPIKey:      getEnv("FRACTSOUL_API_KEY", ""),
		FractsoulAPITimeout:  getEnvAsDuration("FRACTSOUL_API_TIMEOUT", 5*time.Second),
		ContextRackLimit:     getEnvAsInt("ENERGY_CONTEXT_RACK_LIMIT", 3),
		ContextWindowMinutes: getEnvAsInt("ENERGY_CONTEXT_WINDOW_MINUTES", 60),
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

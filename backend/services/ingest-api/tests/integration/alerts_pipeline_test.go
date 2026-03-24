//go:build integration

package integration_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAlertsArePersistedAndSuppressedForDuplicates(t *testing.T) {
	if !envAsBool("INTEGRATION_EXPECT_ALERTS", true) {
		t.Skip("alerts integration checks disabled by INTEGRATION_EXPECT_ALERTS")
	}

	apiURL := getEnv("INTEGRATION_API_URL", "http://localhost:8080")
	databaseURL := getEnv("INTEGRATION_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/mining?sslmode=disable")
	apiKey := getEnv("INTEGRATION_API_KEY", "")
	apiKeyHeader := getEnv("INTEGRATION_API_KEY_HEADER", "X-API-Key")

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}
	ensureAlertsSchemaOrSkip(t, ctx, pool)

	minerID := "asic-799001"
	siteID := "site-cl-01"
	rackID := "rack-cl-01-01"
	baseTime := time.Now().UTC().Add(-30 * time.Second)

	firstEventID := uuid.NewString()
	postIngest(t, apiURL, apiKeyHeader, apiKey, map[string]any{
		"event_id":         firstEventID,
		"timestamp":        baseTime.Format(time.RFC3339Nano),
		"site_id":          siteID,
		"rack_id":          rackID,
		"miner_id":         minerID,
		"firmware_version": "integration-2026.2",
		"metrics": map[string]any{
			"hashrate_ths":   185.0,
			"power_watts":    3420.0,
			"temp_celsius":   101.3,
			"fan_rpm":        7200,
			"efficiency_jth": 18.4,
			"status":         "critical",
		},
		"tags": map[string]string{
			"source":     "integration-alerts",
			"asic_model": "S21",
		},
	})
	waitForPersistedEvent(t, ctx, pool, firstEventID, 10*time.Second)

	secondEventID := uuid.NewString()
	postIngest(t, apiURL, apiKeyHeader, apiKey, map[string]any{
		"event_id":         secondEventID,
		"timestamp":        baseTime.Add(5 * time.Second).Format(time.RFC3339Nano),
		"site_id":          siteID,
		"rack_id":          rackID,
		"miner_id":         minerID,
		"firmware_version": "integration-2026.2",
		"metrics": map[string]any{
			"hashrate_ths":   182.0,
			"power_watts":    3405.0,
			"temp_celsius":   100.8,
			"fan_rpm":        7180,
			"efficiency_jth": 18.7,
			"status":         "critical",
		},
		"tags": map[string]string{
			"source":     "integration-alerts",
			"asic_model": "S21",
		},
	})
	waitForPersistedEvent(t, ctx, pool, secondEventID, 10*time.Second)

	occurrences, status := waitForOverheatSuppressedAlert(t, ctx, pool, minerID, 12*time.Second)
	if occurrences < 2 {
		t.Fatalf("expected overheat alert occurrences >= 2, got %d", occurrences)
	}
	if status != "suppressed" {
		t.Fatalf("expected deduplicated status suppressed, got %s", status)
	}
}

func waitForOverheatSuppressedAlert(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	minerID string,
	timeout time.Duration,
) (int, string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var occurrences int
		var status string
		err := pool.QueryRow(ctx, `
SELECT
  occurrences,
  status
FROM alerts
WHERE miner_id = $1
  AND rule_id = 'overheat'
ORDER BY last_seen_at DESC
LIMIT 1
`, minerID).Scan(&occurrences, &status)
		if err == nil && occurrences >= 2 {
			return occurrences, status
		}
		time.Sleep(300 * time.Millisecond)
	}

	t.Fatalf("alert overheat for miner %s not persisted with dedupe in %s", minerID, timeout)
	return 0, ""
}

func ensureAlertsSchemaOrSkip(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	var relationName *string
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.alerts')::text`).Scan(&relationName); err != nil {
		t.Fatalf("check alerts relation: %v", err)
	}
	if relationName == nil || *relationName == "" {
		t.Skip("alerts table is missing; run migrations before integration tests")
	}
}

func envAsBool(key string, fallback bool) bool {
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

//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestIngestPipelinePersistsAlerts(t *testing.T) {
	apiURL := getEnvOrDefault("E2E_API_URL", "http://localhost:8080")
	databaseURL := getEnvOrDefault("E2E_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/mining?sslmode=disable")
	apiKey := getEnvOrDefault("E2E_API_KEY", "")
	apiKeyHeader := getEnvOrDefault("E2E_API_KEY_HEADER", "X-API-Key")

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
	ensureAlertsSchemaOrSkipE2E(t, ctx, pool)

	minerID := "asic-899001"
	siteID := "site-cl-01"
	rackID := "rack-cl-01-01"
	baseTime := time.Now().UTC().Add(-20 * time.Second)

	event1 := uuid.NewString()
	postAlertEvent(t, apiURL, apiKeyHeader, apiKey, map[string]any{
		"event_id":         event1,
		"timestamp":        baseTime.Format(time.RFC3339Nano),
		"site_id":          siteID,
		"rack_id":          rackID,
		"miner_id":         minerID,
		"firmware_version": "e2e-2026.2",
		"metrics": map[string]any{
			"hashrate_ths":   188.3,
			"power_watts":    3450.2,
			"temp_celsius":   100.4,
			"fan_rpm":        7150,
			"efficiency_jth": 18.3,
			"status":         "critical",
		},
		"tags": map[string]string{
			"source":     "e2e-alerts",
			"asic_model": "S21",
		},
	})
	waitForTelemetryEvent(t, ctx, pool, event1, 10*time.Second)

	event2 := uuid.NewString()
	postAlertEvent(t, apiURL, apiKeyHeader, apiKey, map[string]any{
		"event_id":         event2,
		"timestamp":        baseTime.Add(4 * time.Second).Format(time.RFC3339Nano),
		"site_id":          siteID,
		"rack_id":          rackID,
		"miner_id":         minerID,
		"firmware_version": "e2e-2026.2",
		"metrics": map[string]any{
			"hashrate_ths":   186.1,
			"power_watts":    3444.7,
			"temp_celsius":   99.9,
			"fan_rpm":        7135,
			"efficiency_jth": 18.5,
			"status":         "critical",
		},
		"tags": map[string]string{
			"source":     "e2e-alerts",
			"asic_model": "S21",
		},
	})
	waitForTelemetryEvent(t, ctx, pool, event2, 10*time.Second)

	occurrences, status := waitForE2EAlertSuppression(t, ctx, pool, minerID, 12*time.Second)
	if occurrences < 2 {
		t.Fatalf("expected alert occurrences >= 2, got %d", occurrences)
	}
	if status != "suppressed" {
		t.Fatalf("expected deduplicated alert status suppressed, got %s", status)
	}
}

func postAlertEvent(t *testing.T, apiURL, apiKeyHeader, apiKey string, payload map[string]any) {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiURL+"/v1/telemetry/ingest", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set(apiKeyHeader, apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer resp.Body.Close()

	payloadBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusAccepted, resp.StatusCode, string(payloadBody))
	}
}

func waitForTelemetryEvent(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	eventID string,
	timeout time.Duration,
) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var count int
		err := pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM telemetry_readings
WHERE event_id::text = $1
`, eventID).Scan(&count)
		if err != nil {
			t.Fatalf("query telemetry event: %v", err)
		}
		if count > 0 {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}

	t.Fatalf("event %s was not persisted within %s", eventID, timeout)
}

func waitForE2EAlertSuppression(
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

	t.Fatalf("alert overheat for miner %s not found in %s", minerID, timeout)
	return 0, ""
}

func ensureAlertsSchemaOrSkipE2E(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	var relationName *string
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.alerts')::text`).Scan(&relationName); err != nil {
		t.Fatalf("check alerts relation: %v", err)
	}
	if relationName == nil || *relationName == "" {
		t.Skip("alerts table is missing; run migrations before e2e")
	}
}

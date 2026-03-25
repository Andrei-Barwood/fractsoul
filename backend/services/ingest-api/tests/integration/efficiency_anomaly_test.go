//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEfficiencyAndAnomalyEndpoints(t *testing.T) {
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

	minerID := "asic-810001"
	siteID := "site-cl-01"
	rackID := "rack-cl-01-01"
	baseTime := time.Now().UTC().Add(-15 * time.Minute)

	payloads := []map[string]any{
		{
			"event_id":         uuid.NewString(),
			"timestamp":        baseTime.Format(time.RFC3339Nano),
			"site_id":          siteID,
			"rack_id":          rackID,
			"miner_id":         minerID,
			"firmware_version": "integration-2026.3",
			"metrics": map[string]any{
				"hashrate_ths":   196.0,
				"power_watts":    3530.0,
				"temp_celsius":   78.0,
				"fan_rpm":        6100,
				"efficiency_jth": 18.0,
				"status":         "ok",
			},
			"tags": map[string]string{
				"asic_model":     "S21",
				"ambient_temp_c": "27.0",
			},
		},
		{
			"event_id":         uuid.NewString(),
			"timestamp":        baseTime.Add(5 * time.Minute).Format(time.RFC3339Nano),
			"site_id":          siteID,
			"rack_id":          rackID,
			"miner_id":         minerID,
			"firmware_version": "integration-2026.3",
			"metrics": map[string]any{
				"hashrate_ths":   162.0,
				"power_watts":    3560.0,
				"temp_celsius":   95.0,
				"fan_rpm":        7250,
				"efficiency_jth": 22.0,
				"status":         "critical",
			},
			"tags": map[string]string{
				"asic_model":     "S21",
				"ambient_temp_c": "31.5",
			},
		},
	}

	for _, payload := range payloads {
		postIngest(t, apiURL, apiKeyHeader, apiKey, payload)
		waitForPersistedEvent(t, ctx, pool, payload["event_id"].(string), 10*time.Second)
	}

	minerEffURL := fmt.Sprintf("%s/v1/efficiency/miners?miner_id=%s&window_minutes=120&limit=20", apiURL, minerID)
	minerEffResp := getJSON(t, minerEffURL, apiKeyHeader, apiKey)
	if count, ok := minerEffResp["count"].(float64); !ok || int(count) <= 0 {
		t.Fatalf("expected miner efficiency count > 0, got %#v", minerEffResp["count"])
	}

	rackEffURL := fmt.Sprintf("%s/v1/efficiency/racks?site_id=%s&rack_id=%s&window_minutes=120&limit=20", apiURL, siteID, rackID)
	rackEffResp := getJSON(t, rackEffURL, apiKeyHeader, apiKey)
	if count, ok := rackEffResp["count"].(float64); !ok || int(count) <= 0 {
		t.Fatalf("expected rack efficiency count > 0, got %#v", rackEffResp["count"])
	}

	siteEffURL := fmt.Sprintf("%s/v1/efficiency/sites?site_id=%s&window_minutes=120&limit=20", apiURL, siteID)
	siteEffResp := getJSON(t, siteEffURL, apiKeyHeader, apiKey)
	if count, ok := siteEffResp["count"].(float64); !ok || int(count) <= 0 {
		t.Fatalf("expected site efficiency count > 0, got %#v", siteEffResp["count"])
	}

	from := url.QueryEscape(baseTime.Add(-5 * time.Minute).Format(time.RFC3339))
	to := url.QueryEscape(time.Now().UTC().Format(time.RFC3339))
	anomalyURL := fmt.Sprintf(
		"%s/v1/anomalies/miners/%s/analyze?resolution=minute&from=%s&to=%s&limit=180",
		apiURL,
		minerID,
		from,
		to,
	)
	anomalyResp := getJSON(t, anomalyURL, apiKeyHeader, apiKey)
	reportAny, ok := anomalyResp["report"].(map[string]any)
	if !ok {
		t.Fatalf("expected anomaly report object, got %#v", anomalyResp["report"])
	}
	if severity, ok := reportAny["severity_score"].(float64); !ok || severity <= 0 {
		t.Fatalf("expected anomaly severity_score > 0, got %#v", reportAny["severity_score"])
	}
	if _, ok := reportAny["impact_estimate"].(map[string]any); !ok {
		t.Fatalf("expected impact_estimate object, got %#v", reportAny["impact_estimate"])
	}
	guardrails, ok := reportAny["guardrails"].([]any)
	if !ok || len(guardrails) == 0 {
		t.Fatalf("expected guardrails array with items, got %#v", reportAny["guardrails"])
	}
}

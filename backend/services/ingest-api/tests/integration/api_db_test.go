//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestIngestPersistsTelemetryInDatabase(t *testing.T) {
	apiURL := getEnv("INTEGRATION_API_URL", "http://localhost:8080")
	databaseURL := getEnv("INTEGRATION_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/mining?sslmode=disable")
	apiKey := getEnv("INTEGRATION_API_KEY", "")
	apiKeyHeader := getEnv("INTEGRATION_API_KEY_HEADER", "X-API-Key")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}

	eventID := uuid.NewString()
	minerID := "asic-700001"
	rackID := "rack-cl-01-01"
	siteID := "site-cl-01"

	payload := map[string]any{
		"event_id":         eventID,
		"timestamp":        time.Now().UTC().Format(time.RFC3339Nano),
		"site_id":          siteID,
		"rack_id":          rackID,
		"miner_id":         minerID,
		"firmware_version": "integration-2026.1",
		"metrics": map[string]any{
			"hashrate_ths":   142.6,
			"power_watts":    3352.1,
			"temp_celsius":   75.3,
			"fan_rpm":        6280,
			"efficiency_jth": 23.5,
			"status":         "ok",
		},
		"tags": map[string]string{
			"source":     "integration-test",
			"load_pct":   "94.1",
			"asic_model": "S21",
		},
	}

	postIngest(t, apiURL, apiKeyHeader, apiKey, payload)
	waitForPersistedEvent(t, ctx, pool, eventID, 10*time.Second)
}

func TestReadEndpointsReturnPersistedTelemetry(t *testing.T) {
	apiURL := getEnv("INTEGRATION_API_URL", "http://localhost:8080")
	databaseURL := getEnv("INTEGRATION_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/mining?sslmode=disable")
	apiKey := getEnv("INTEGRATION_API_KEY", "")
	apiKeyHeader := getEnv("INTEGRATION_API_KEY_HEADER", "X-API-Key")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}

	eventID := uuid.NewString()
	minerID := "asic-700002"
	rackID := "rack-cl-01-02"
	siteID := "site-cl-01"
	eventTime := time.Now().UTC()

	payload := map[string]any{
		"event_id":         eventID,
		"timestamp":        eventTime.Format(time.RFC3339Nano),
		"site_id":          siteID,
		"rack_id":          rackID,
		"miner_id":         minerID,
		"firmware_version": "integration-2026.1",
		"metrics": map[string]any{
			"hashrate_ths":   121.2,
			"power_watts":    3488.4,
			"temp_celsius":   92.7,
			"fan_rpm":        7120,
			"efficiency_jth": 28.8,
			"status":         "critical",
		},
		"tags": map[string]string{
			"source":     "integration-test",
			"load_pct":   "96.8",
			"asic_model": "M50",
		},
	}

	postIngest(t, apiURL, apiKeyHeader, apiKey, payload)
	waitForPersistedEvent(t, ctx, pool, eventID, 10*time.Second)

	from := url.QueryEscape(eventTime.Add(-2 * time.Minute).Format(time.RFC3339))
	to := url.QueryEscape(eventTime.Add(5 * time.Minute).Format(time.RFC3339))
	rackURL := fmt.Sprintf(
		"%s/v1/telemetry/sites/%s/racks/%s/readings?status=critical&model=m50&miner_id=%s&from=%s&to=%s&limit=20",
		apiURL,
		siteID,
		rackID,
		minerID,
		from,
		to,
	)
	rackResp := getJSON(t, rackURL, apiKeyHeader, apiKey)
	if count, ok := rackResp["count"].(float64); !ok || int(count) <= 0 {
		t.Fatalf("expected rack endpoint count > 0, got %#v", rackResp["count"])
	}

	readingsURL := fmt.Sprintf(
		"%s/v1/telemetry/readings?model=m50&miner_id=%s&from=%s&to=%s&limit=20",
		apiURL,
		minerID,
		from,
		to,
	)
	readingsResp := getJSON(t, readingsURL, apiKeyHeader, apiKey)
	if count, ok := readingsResp["count"].(float64); !ok || int(count) <= 0 {
		t.Fatalf("expected readings endpoint count > 0 for model filter, got %#v", readingsResp["count"])
	}

	summaryURL := fmt.Sprintf("%s/v1/telemetry/summary?model=m50&window_minutes=60", apiURL)
	summaryResp := getJSON(t, summaryURL, apiKeyHeader, apiKey)
	summaryObject, ok := summaryResp["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary object, got %#v", summaryResp["summary"])
	}
	if samples, ok := summaryObject["samples"].(float64); !ok || int(samples) <= 0 {
		t.Fatalf("expected summary samples > 0 for model filter, got %#v", summaryObject["samples"])
	}

	tsURL := fmt.Sprintf(
		"%s/v1/telemetry/miners/%s/timeseries?resolution=minute&from=%s&to=%s&limit=30",
		apiURL,
		minerID,
		from,
		to,
	)
	seriesResp := getJSON(t, tsURL, apiKeyHeader, apiKey)
	if count, ok := seriesResp["count"].(float64); !ok || int(count) <= 0 {
		t.Fatalf("expected timeseries endpoint count > 0, got %#v", seriesResp["count"])
	}
}

func postIngest(t *testing.T, apiURL, apiKeyHeader, apiKey string, payload map[string]any) {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiURL+"/v1/telemetry/ingest", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build ingest request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set(apiKeyHeader, apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send ingest request: %v", err)
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusAccepted, resp.StatusCode, string(responseBody))
	}
}

func waitForPersistedEvent(
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
			t.Fatalf("query telemetry_readings by event_id: %v", err)
		}
		if count > 0 {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}

	t.Fatalf("event %s was not persisted within %s", eventID, timeout)
}

func getJSON(t *testing.T, endpoint, apiKeyHeader, apiKey string) map[string]any {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if apiKey != "" {
		req.Header.Set(apiKeyHeader, apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("perform request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status %d from %s, got %d body=%s", http.StatusOK, endpoint, resp.StatusCode, string(body))
	}

	var decoded map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode response from %s: %v", endpoint, err)
	}
	return decoded
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

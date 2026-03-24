//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSimulatorPipelineToDBAndReadAPI(t *testing.T) {
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
	ensureSchemaOrSkip(t, ctx, pool)

	beforeCount := readingsCount(t, ctx, pool)

	runSimulator(t, apiURL, apiKey)

	afterCount := readingsCount(t, ctx, pool)
	if afterCount <= beforeCount {
		t.Fatalf("expected telemetry_readings to grow, before=%d after=%d", beforeCount, afterCount)
	}

	readingsResp := getJSON(t, apiURL+"/v1/telemetry/readings?limit=5", apiKeyHeader, apiKey)
	if count, ok := readingsResp["count"].(float64); !ok || int(count) <= 0 {
		t.Fatalf("expected readings count > 0, got %#v", readingsResp["count"])
	}

	summaryResp := getJSON(t, apiURL+"/v1/telemetry/summary?window_minutes=30", apiKeyHeader, apiKey)
	summaryAny, ok := summaryResp["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary object, got %#v", summaryResp["summary"])
	}

	if samples, ok := summaryAny["samples"].(float64); !ok || int(samples) <= 0 {
		t.Fatalf("expected summary samples > 0, got %#v", summaryAny["samples"])
	}

	rackResp := getJSON(
		t,
		apiURL+"/v1/telemetry/sites/site-cl-01/racks/rack-cl-01-01/readings?limit=5",
		apiKeyHeader,
		apiKey,
	)
	if count, ok := rackResp["count"].(float64); !ok || int(count) <= 0 {
		t.Fatalf("expected rack readings count > 0, got %#v", rackResp["count"])
	}

	now := time.Now().UTC()
	from := url.QueryEscape(now.Add(-2 * time.Hour).Format(time.RFC3339))
	to := url.QueryEscape(now.Format(time.RFC3339))
	timeseriesResp := getJSON(
		t,
		apiURL+"/v1/telemetry/miners/asic-000001/timeseries?resolution=minute&from="+from+"&to="+to+"&limit=120",
		apiKeyHeader,
		apiKey,
	)
	if count, ok := timeseriesResp["count"].(float64); !ok || int(count) <= 0 {
		t.Fatalf("expected miner timeseries count > 0, got %#v", timeseriesResp["count"])
	}
}

func runSimulator(t *testing.T, apiURL, apiKey string) {
	t.Helper()

	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	args := []string{
		"run",
		"./cmd/simulator",
		"-api-url", apiURL,
		"-miners", "20",
		"-duration", "10s",
		"-tick", "2s",
		"-concurrency", "8",
	}
	if apiKey != "" {
		args = append(args, "-api-key", apiKey)
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = moduleRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run simulator: %v output=%s", err, string(output))
	}
}

func readingsCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int64 {
	t.Helper()

	var count int64
	err := pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM telemetry_readings
`).Scan(&count)
	if err != nil {
		t.Fatalf("count telemetry_readings: %v", err)
	}

	return count
}

func ensureSchemaOrSkip(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	var relationName *string
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.telemetry_readings')::text`).Scan(&relationName); err != nil {
		t.Fatalf("check telemetry_readings relation: %v", err)
	}

	if relationName == nil || *relationName == "" {
		t.Skip("telemetry_readings table is missing; run migrations before e2e")
	}
}

func getJSON(t *testing.T, url, apiKeyHeader, apiKey string) map[string]any {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, url, nil)
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
		t.Fatalf("expected status %d from %s, got %d", http.StatusOK, url, resp.StatusCode)
	}

	var decoded map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode response from %s: %v", url, err)
	}

	return decoded
}

func getEnvOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

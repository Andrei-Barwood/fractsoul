//go:build perf

package perf_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPerformanceWith100ASICs(t *testing.T) {
	apiURL := getEnv("PERF_API_URL", "http://localhost:8080")
	databaseURL := getEnv("PERF_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/mining?sslmode=disable")
	apiKey := getEnv("PERF_API_KEY", "")
	apiKeyHeader := getEnv("PERF_API_KEY_HEADER", "X-API-Key")
	simDuration := 20 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}

	beforeCount := readingsCount(t, ctx, pool)
	runSimulator100(t, apiURL, apiKey, simDuration)
	afterCount := readingsCount(t, ctx, pool)

	inserted := afterCount - beforeCount
	if inserted <= 0 {
		t.Fatalf("expected telemetry_readings to grow, before=%d after=%d", beforeCount, afterCount)
	}

	ingestRate := float64(inserted) / simDuration.Seconds()
	t.Logf("inserted events=%d ingest_rate=%.2f events/s", inserted, ingestRate)
	if ingestRate < 20 {
		t.Fatalf("expected ingest rate >= 20 events/s, got %.2f", ingestRate)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	now := time.Now().UTC()
	from := url.QueryEscape(now.Add(-2 * time.Hour).Format(time.RFC3339))
	to := url.QueryEscape(now.Format(time.RFC3339))

	rackURL := apiURL + "/v1/telemetry/sites/site-cl-01/racks/rack-cl-01-01/readings?limit=50"
	minerURL := apiURL + "/v1/telemetry/miners/asic-000001/timeseries?resolution=minute&from=" + from + "&to=" + to + "&limit=120"

	headers := map[string]string{}
	if apiKey != "" {
		headers[apiKeyHeader] = apiKey
	}

	rackStats := runEndpointLoad(t, client, rackURL, headers, 40, 8)
	minerStats := runEndpointLoad(t, client, minerURL, headers, 40, 8)

	t.Logf("rack endpoint p95=%s avg=%s", rackStats.P95, rackStats.Avg)
	t.Logf("miner endpoint p95=%s avg=%s", minerStats.P95, minerStats.Avg)

	maxP95 := 2 * time.Second
	if rackStats.P95 > maxP95 {
		t.Fatalf("rack endpoint p95 too high: %s > %s", rackStats.P95, maxP95)
	}
	if minerStats.P95 > maxP95 {
		t.Fatalf("miner endpoint p95 too high: %s > %s", minerStats.P95, maxP95)
	}
}

type latencyStats struct {
	Count int
	Avg   time.Duration
	P95   time.Duration
}

func runEndpointLoad(
	t *testing.T,
	client *http.Client,
	endpoint string,
	headers map[string]string,
	requests, workers int,
) latencyStats {
	t.Helper()

	var (
		mu        sync.Mutex
		latencies = make([]time.Duration, 0, requests)
		firstErr  error
	)

	jobs := make(chan int, requests)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				start := time.Now()
				req, err := http.NewRequest(http.MethodGet, endpoint, nil)
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("build request failed: %w", err)
					}
					mu.Unlock()
					continue
				}
				for key, value := range headers {
					req.Header.Set(key, value)
				}

				resp, err := client.Do(req) //nolint:noctx // short-lived perf probe
				elapsed := time.Since(start)
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("request failed: %w", err)
					}
					mu.Unlock()
					continue
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					mu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("unexpected status %d for %s", resp.StatusCode, endpoint)
					}
					mu.Unlock()
					continue
				}

				mu.Lock()
				latencies = append(latencies, elapsed)
				mu.Unlock()
			}
		}()
	}

	for i := 0; i < requests; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	if firstErr != nil {
		t.Fatal(firstErr)
	}
	if len(latencies) == 0 {
		t.Fatalf("no successful requests collected for %s", endpoint)
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	total := time.Duration(0)
	for _, latency := range latencies {
		total += latency
	}
	p95Index := int(float64(len(latencies)-1) * 0.95)

	return latencyStats{
		Count: len(latencies),
		Avg:   total / time.Duration(len(latencies)),
		P95:   latencies[p95Index],
	}
}

func runSimulator100(t *testing.T, apiURL, apiKey string, duration time.Duration) {
	t.Helper()

	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	args := []string{
		"run",
		"./cmd/simulator",
		"-api-url", apiURL,
		"-miners", "100",
		"-duration", duration.String(),
		"-tick", "2s",
		"-concurrency", "32",
		"-profile-mode", "mixed",
		"-schedule", "staggered",
		"-schedule-jitter", "200ms",
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
WHERE ts >= NOW() - INTERVAL '30 minutes'
`).Scan(&count)
	if err != nil {
		t.Fatalf("count telemetry_readings: %v", err)
	}

	return count
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

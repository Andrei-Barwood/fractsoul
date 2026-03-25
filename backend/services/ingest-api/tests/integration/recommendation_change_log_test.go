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
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRecommendationChangeLogApplyAndRollback(t *testing.T) {
	apiURL := getEnv("INTEGRATION_API_URL", "http://localhost:8080")
	databaseURL := getEnv("INTEGRATION_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/mining?sslmode=disable")
	apiKey := getEnv("INTEGRATION_API_KEY", "")
	apiKeyHeader := getEnv("INTEGRATION_API_KEY_HEADER", "X-API-Key")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}

	ensureRecommendationChangesSchemaOrSkip(t, ctx, pool)

	minerID := "asic-820001"
	siteID := "site-cl-01"
	rackID := "rack-cl-01-01"
	baseTime := time.Now().UTC().Add(-12 * time.Minute)

	events := []map[string]any{
		{
			"event_id":         uuid.NewString(),
			"timestamp":        baseTime.Format(time.RFC3339Nano),
			"site_id":          siteID,
			"rack_id":          rackID,
			"miner_id":         minerID,
			"firmware_version": "integration-2026.4",
			"metrics": map[string]any{
				"hashrate_ths":   198.0,
				"power_watts":    3520.0,
				"temp_celsius":   82.0,
				"fan_rpm":        6300,
				"efficiency_jth": 17.8,
				"status":         "ok",
			},
			"tags": map[string]string{
				"asic_model":     "S21",
				"ambient_temp_c": "27.5",
			},
		},
		{
			"event_id":         uuid.NewString(),
			"timestamp":        baseTime.Add(6 * time.Minute).Format(time.RFC3339Nano),
			"site_id":          siteID,
			"rack_id":          rackID,
			"miner_id":         minerID,
			"firmware_version": "integration-2026.4",
			"metrics": map[string]any{
				"hashrate_ths":   156.0,
				"power_watts":    3540.0,
				"temp_celsius":   96.0,
				"fan_rpm":        7300,
				"efficiency_jth": 22.7,
				"status":         "critical",
			},
			"tags": map[string]string{
				"asic_model":     "S21",
				"ambient_temp_c": "31.2",
			},
		},
	}

	for _, payload := range events {
		postIngest(t, apiURL, apiKeyHeader, apiKey, payload)
		waitForPersistedEvent(t, ctx, pool, payload["event_id"].(string), 10*time.Second)
	}

	from := url.QueryEscape(baseTime.Add(-2 * time.Minute).Format(time.RFC3339))
	to := url.QueryEscape(time.Now().UTC().Format(time.RFC3339))
	applyURL := fmt.Sprintf(
		"%s/v1/anomalies/miners/%s/changes/apply?resolution=minute&from=%s&to=%s&limit=180",
		apiURL,
		minerID,
		from,
		to,
	)

	applyResp := postJSON(t, applyURL, apiKeyHeader, apiKey, map[string]any{
		"reason":       "integration apply",
		"requested_by": "integration@test",
	}, http.StatusCreated)

	changeAny, ok := applyResp["change"].(map[string]any)
	if !ok {
		t.Fatalf("expected change object in apply response, got %#v", applyResp["change"])
	}
	applyChangeID, ok := changeAny["change_id"].(string)
	if !ok || applyChangeID == "" {
		t.Fatalf("expected apply change_id, got %#v", changeAny["change_id"])
	}

	var applyStatus string
	err = pool.QueryRow(ctx, `
SELECT status
FROM recommendation_changes
WHERE change_id = $1
`, applyChangeID).Scan(&applyStatus)
	if err != nil {
		t.Fatalf("query apply change status: %v", err)
	}
	if applyStatus != "applied" {
		t.Fatalf("expected apply change status 'applied', got %s", applyStatus)
	}

	rollbackURL := fmt.Sprintf("%s/v1/anomalies/changes/%s/rollback", apiURL, applyChangeID)
	rollbackResp := postJSON(t, rollbackURL, apiKeyHeader, apiKey, map[string]any{
		"reason":       "integration rollback",
		"requested_by": "integration@test",
	}, http.StatusCreated)

	rollbackChangeAny, ok := rollbackResp["change"].(map[string]any)
	if !ok {
		t.Fatalf("expected rollback change object, got %#v", rollbackResp["change"])
	}
	rollbackChangeID, ok := rollbackChangeAny["change_id"].(string)
	if !ok || rollbackChangeID == "" {
		t.Fatalf("expected rollback change_id, got %#v", rollbackChangeAny["change_id"])
	}

	var sourceStatus string
	var supersededBy *string
	err = pool.QueryRow(ctx, `
SELECT status, superseded_by_change_id
FROM recommendation_changes
WHERE change_id = $1
`, applyChangeID).Scan(&sourceStatus, &supersededBy)
	if err != nil {
		t.Fatalf("query source change after rollback: %v", err)
	}
	if sourceStatus != "rolled_back" {
		t.Fatalf("expected source status rolled_back, got %s", sourceStatus)
	}
	if supersededBy == nil || *supersededBy != rollbackChangeID {
		t.Fatalf("expected superseded_by_change_id=%s, got %#v", rollbackChangeID, supersededBy)
	}

	var parentID string
	var operation string
	err = pool.QueryRow(ctx, `
SELECT parent_change_id, operation
FROM recommendation_changes
WHERE change_id = $1
`, rollbackChangeID).Scan(&parentID, &operation)
	if err != nil {
		t.Fatalf("query rollback change linkage: %v", err)
	}
	if parentID != applyChangeID {
		t.Fatalf("expected rollback parent=%s, got %s", applyChangeID, parentID)
	}
	if operation != "rollback" {
		t.Fatalf("expected rollback operation, got %s", operation)
	}
}

func postJSON(
	t *testing.T,
	endpoint, apiKeyHeader, apiKey string,
	payload map[string]any,
	expectedStatus int,
) map[string]any {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set(apiKeyHeader, apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("perform request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedStatus {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status %d from %s, got %d body=%s", expectedStatus, endpoint, resp.StatusCode, string(raw))
	}

	decoded := map[string]any{}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode response from %s: %v", endpoint, err)
	}

	return decoded
}

func ensureRecommendationChangesSchemaOrSkip(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	var relationName *string
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.recommendation_changes')::text`).Scan(&relationName); err != nil {
		t.Fatalf("check recommendation_changes relation: %v", err)
	}
	if relationName == nil || *relationName == "" {
		t.Skip("recommendation_changes table is missing; run migrations before integration")
	}
}

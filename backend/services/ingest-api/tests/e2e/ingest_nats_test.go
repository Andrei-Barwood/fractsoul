//go:build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

func TestIngestPublishesToNATS(t *testing.T) {
	apiURL := getEnv("E2E_API_URL", "http://localhost:8080")
	natsURL := getEnv("E2E_NATS_URL", "nats://localhost:4222")
	subject := getEnv("E2E_TELEMETRY_SUBJECT", "telemetry.raw.v1")
	apiKey := getEnv("E2E_API_KEY", "")
	apiKeyHeader := getEnv("E2E_API_KEY_HEADER", "X-API-Key")

	nc, err := nats.Connect(natsURL, nats.Timeout(5*time.Second))
	if err != nil {
		t.Fatalf("connect nats: %v", err)
	}
	defer nc.Close()

	sub, err := nc.SubscribeSync(subject)
	if err != nil {
		t.Fatalf("subscribe nats: %v", err)
	}
	defer sub.Unsubscribe()

	if err := nc.Flush(); err != nil {
		t.Fatalf("flush nats: %v", err)
	}

	eventID := uuid.NewString()
	requestPayload := map[string]any{
		"event_id":         eventID,
		"timestamp":        time.Now().UTC().Format(time.RFC3339Nano),
		"site_id":          "site-cl-01",
		"rack_id":          "rack-a1",
		"miner_id":         "asic-0001",
		"firmware_version": "braiins-2026.1",
		"metrics": map[string]any{
			"hashrate_ths":   126.5,
			"power_watts":    3325,
			"temp_celsius":   71.2,
			"fan_rpm":        6400,
			"efficiency_jth": 26.3,
			"status":         "ok",
		},
	}

	body, err := json.Marshal(requestPayload)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
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
		t.Fatalf("post ingest: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusAccepted, resp.StatusCode, string(respBody))
	}

	msg, err := sub.NextMsg(5 * time.Second)
	if err != nil {
		t.Fatalf("wait nats message: %v", err)
	}

	var published map[string]any
	if err := json.Unmarshal(msg.Data, &published); err != nil {
		t.Fatalf("decode nats payload: %v", err)
	}

	if published["event_id"] != eventID {
		t.Fatalf("expected event_id %s, got %#v", eventID, published["event_id"])
	}

	if msg.Header.Get("X-Event-ID") != eventID {
		t.Fatalf("expected X-Event-ID %s, got %s", eventID, msg.Header.Get("X-Event-ID"))
	}

	if msg.Header.Get("X-Request-ID") == "" {
		t.Fatal("expected X-Request-ID header")
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

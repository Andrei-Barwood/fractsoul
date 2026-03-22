package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type publishedMessage struct {
	subject string
	payload []byte
	headers map[string]string
}

type stubPublisher struct {
	publishErr error
	published  []publishedMessage
}

func (p *stubPublisher) Publish(_ context.Context, subject string, payload []byte, headers map[string]string) error {
	if p.publishErr != nil {
		return p.publishErr
	}

	copiedHeaders := make(map[string]string, len(headers))
	for key, value := range headers {
		copiedHeaders[key] = value
	}

	copiedPayload := append([]byte(nil), payload...)
	p.published = append(p.published, publishedMessage{
		subject: subject,
		payload: copiedPayload,
		headers: copiedHeaders,
	})

	return nil
}

func (p *stubPublisher) Close() error {
	return nil
}

func buildTestRouter() (http.Handler, *stubPublisher) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{}
	return NewRouter(logger, publisher, "telemetry.raw.v1"), publisher
}

func TestIngestAcceptsValidPayload(t *testing.T) {
	router, publisher := buildTestRouter()

	payload := `{
		"event_id":"550e8400-e29b-41d4-a716-446655440000",
		"timestamp":"2026-03-22T14:00:00Z",
		"site_id":"site-cl-01",
		"rack_id":"rack-a1",
		"miner_id":"asic-0001",
		"firmware_version":"braiins-2026.1",
		"metrics":{
			"hashrate_ths":126.5,
			"power_watts":3325,
			"temp_celsius":71.2,
			"fan_rpm":6400,
			"efficiency_jth":26.3,
			"status":"ok"
		},
		"tags":{
			"pool":"mainnet"
		}
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/telemetry/ingest", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, resp.Code)
	}

	if got := resp.Header().Get("X-Request-ID"); got == "" {
		t.Fatal("expected X-Request-ID header")
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if accepted, ok := body["accepted"].(bool); !ok || !accepted {
		t.Fatalf("expected accepted=true, got %#v", body["accepted"])
	}

	if queue, ok := body["queue_topic"].(string); !ok || queue != "telemetry.raw.v1" {
		t.Fatalf("expected queue_topic telemetry.raw.v1, got %#v", body["queue_topic"])
	}

	if len(publisher.published) != 1 {
		t.Fatalf("expected one nats publish, got %d", len(publisher.published))
	}

	if publisher.published[0].subject != "telemetry.raw.v1" {
		t.Fatalf("expected subject telemetry.raw.v1, got %s", publisher.published[0].subject)
	}

	var publishedPayload map[string]any
	if err := json.Unmarshal(publisher.published[0].payload, &publishedPayload); err != nil {
		t.Fatalf("decode published payload: %v", err)
	}

	if publishedPayload["event_id"] != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("unexpected published event_id: %#v", publishedPayload["event_id"])
	}

	if publisher.published[0].headers["X-Request-ID"] == "" {
		t.Fatal("expected X-Request-ID publish header")
	}
}

func TestIngestRejectsInvalidPayload(t *testing.T) {
	router, _ := buildTestRouter()

	payload := `{"event_id":"not-a-uuid"}`

	req := httptest.NewRequest(http.MethodPost, "/v1/telemetry/ingest", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, resp.Code)
	}

	if !strings.Contains(resp.Body.String(), "validation_error") {
		t.Fatalf("expected validation_error in response, got %s", resp.Body.String())
	}
}

func TestIngestRejectsFutureTimestamp(t *testing.T) {
	router, _ := buildTestRouter()

	payload := `{
		"event_id":"550e8400-e29b-41d4-a716-446655440000",
		"timestamp":"2099-03-22T14:00:00Z",
		"site_id":"site-cl-01",
		"rack_id":"rack-a1",
		"miner_id":"asic-0001",
		"metrics":{
			"hashrate_ths":126.5,
			"power_watts":3325,
			"temp_celsius":71.2,
			"fan_rpm":6400,
			"efficiency_jth":26.3,
			"status":"ok"
		}
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/telemetry/ingest", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, resp.Code)
	}

	if !strings.Contains(resp.Body.String(), "timestamp_out_of_range") {
		t.Fatalf("expected timestamp_out_of_range in response, got %s", resp.Body.String())
	}
}

func TestIngestReturnsDependencyErrorWhenPublishFails(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{publishErr: errors.New("nats unavailable")}
	router := NewRouter(logger, publisher, "telemetry.raw.v1")

	payload := `{
		"event_id":"550e8400-e29b-41d4-a716-446655440000",
		"timestamp":"2026-03-22T14:00:00Z",
		"site_id":"site-cl-01",
		"rack_id":"rack-a1",
		"miner_id":"asic-0001",
		"metrics":{
			"hashrate_ths":126.5,
			"power_watts":3325,
			"temp_celsius":71.2,
			"fan_rpm":6400,
			"efficiency_jth":26.3,
			"status":"ok"
		}
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/telemetry/ingest", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, resp.Code)
	}

	if !strings.Contains(resp.Body.String(), "dependency_unavailable") {
		t.Fatalf("expected dependency_unavailable in response, got %s", resp.Body.String())
	}
}

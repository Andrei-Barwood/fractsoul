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
	"time"

	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/storage"
	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/telemetry"
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

type stubRepository struct {
	items      []storage.TelemetryReading
	summary    storage.TelemetrySummary
	listErr    error
	summaryErr error
}

func (r *stubRepository) PersistTelemetry(_ context.Context, _ telemetry.IngestRequest, _ []byte) error {
	return nil
}

func (r *stubRepository) ListReadings(_ context.Context, _ storage.ReadingsFilter) ([]storage.TelemetryReading, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.items, nil
}

func (r *stubRepository) SummarizeReadings(_ context.Context, _ storage.SummaryFilter) (storage.TelemetrySummary, error) {
	if r.summaryErr != nil {
		return storage.TelemetrySummary{}, r.summaryErr
	}
	return r.summary, nil
}

func (r *stubRepository) Close() {}

func buildTestRouter() (http.Handler, *stubPublisher) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{}
	return NewRouter(logger, publisher, "telemetry.raw.v1", nil, 1<<20), publisher
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

func TestIngestRejectsUnsupportedMediaType(t *testing.T) {
	router, _ := buildTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/v1/telemetry/ingest", bytes.NewBufferString(`{"event_id":"x"}`))
	req.Header.Set("Content-Type", "text/plain")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusUnsupportedMediaType, resp.Code, resp.Body.String())
	}

	if !strings.Contains(resp.Body.String(), "unsupported_media_type") {
		t.Fatalf("expected unsupported_media_type in response, got %s", resp.Body.String())
	}
}

func TestIngestRejectsUnknownField(t *testing.T) {
	router, _ := buildTestRouter()

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
		},
		"unexpected_field":"nope"
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/telemetry/ingest", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, resp.Code, resp.Body.String())
	}

	if !strings.Contains(resp.Body.String(), "invalid_json") {
		t.Fatalf("expected invalid_json in response, got %s", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "unexpected_field") {
		t.Fatalf("expected unknown field details in response, got %s", resp.Body.String())
	}
}

func TestIngestRejectsPayloadTooLarge(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{}
	router := NewRouter(logger, publisher, "telemetry.raw.v1", nil, 256)

	largeTag := strings.Repeat("a", 1024)
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
		},
		"tags":{"blob":"` + largeTag + `"}
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/telemetry/ingest", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusRequestEntityTooLarge, resp.Code, resp.Body.String())
	}

	if !strings.Contains(resp.Body.String(), "payload_too_large") {
		t.Fatalf("expected payload_too_large in response, got %s", resp.Body.String())
	}
}

func TestIngestNormalizesOperationalIDs(t *testing.T) {
	router, publisher := buildTestRouter()

	payload := `{
		"event_id":"550e8400-e29b-41d4-a716-446655440010",
		"timestamp":"2026-03-22T14:00:00Z",
		"site_id":"SITE-CL-1",
		"rack_id":"rack-a1",
		"miner_id":"ASIC_42",
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

	if resp.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusAccepted, resp.Code, resp.Body.String())
	}

	if len(publisher.published) != 1 {
		t.Fatalf("expected one nats publish, got %d", len(publisher.published))
	}

	var publishedPayload map[string]any
	if err := json.Unmarshal(publisher.published[0].payload, &publishedPayload); err != nil {
		t.Fatalf("decode published payload: %v", err)
	}

	if publishedPayload["site_id"] != "site-cl-01" {
		t.Fatalf("expected normalized site_id, got %#v", publishedPayload["site_id"])
	}

	if publishedPayload["rack_id"] != "rack-cl-01-01" {
		t.Fatalf("expected normalized rack_id, got %#v", publishedPayload["rack_id"])
	}

	if publishedPayload["miner_id"] != "asic-000042" {
		t.Fatalf("expected normalized miner_id, got %#v", publishedPayload["miner_id"])
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
	router := NewRouter(logger, publisher, "telemetry.raw.v1", nil, 1<<20)

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

func TestReadingsReturnsDependencyUnavailableWhenRepoMissing(t *testing.T) {
	router, _ := buildTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/v1/telemetry/readings", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusServiceUnavailable, resp.Code, resp.Body.String())
	}

	if !strings.Contains(resp.Body.String(), "dependency_unavailable") {
		t.Fatalf("expected dependency_unavailable in response, got %s", resp.Body.String())
	}
}

func TestReadingsReturnsItems(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{}
	repo := &stubRepository{
		items: []storage.TelemetryReading{
			{
				Timestamp:     time.Date(2026, 3, 24, 14, 0, 0, 0, time.UTC),
				EventID:       "550e8400-e29b-41d4-a716-446655440000",
				SiteID:        "site-cl-01",
				RackID:        "rack-cl-01-01",
				MinerID:       "asic-000001",
				HashrateTHs:   131.2,
				PowerWatts:    3410,
				TempCelsius:   74.5,
				FanRPM:        6200,
				EfficiencyJTH: 25.99,
				Status:        telemetry.StatusOK,
				LoadPct:       91.3,
				IngestedAt:    time.Date(2026, 3, 24, 14, 0, 1, 0, time.UTC),
			},
		},
	}
	router := NewRouter(logger, publisher, "telemetry.raw.v1", repo, 1<<20)

	req := httptest.NewRequest(http.MethodGet, "/v1/telemetry/readings?site_id=SITE-CL-1&limit=10", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, resp.Code, resp.Body.String())
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	count, ok := body["count"].(float64)
	if !ok || int(count) != 1 {
		t.Fatalf("expected count=1, got %#v", body["count"])
	}
}

func TestSummaryReturnsOK(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{}
	repo := &stubRepository{
		summary: storage.TelemetrySummary{
			WindowMinutes:    60,
			Samples:          120,
			AvgHashrateTHs:   129.4,
			AvgPowerWatts:    3380.5,
			AvgTempCelsius:   73.4,
			P95TempCelsius:   84.2,
			MaxTempCelsius:   91.8,
			AvgFanRPM:        6120,
			AvgEfficiencyJTH: 26.1,
		},
	}
	router := NewRouter(logger, publisher, "telemetry.raw.v1", repo, 1<<20)

	req := httptest.NewRequest(http.MethodGet, "/v1/telemetry/summary?window_minutes=120", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, resp.Code, resp.Body.String())
	}

	if !strings.Contains(resp.Body.String(), "\"samples\":120") {
		t.Fatalf("expected summary payload in response, got %s", resp.Body.String())
	}
}

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
	items              []storage.TelemetryReading
	summary            storage.TelemetrySummary
	series             []storage.MinerSeriesPoint
	minerEfficiency    []storage.MinerEfficiency
	rackEfficiency     []storage.RackEfficiency
	siteEfficiency     []storage.SiteEfficiency
	changes            []storage.RecommendationChange
	listErr            error
	summaryErr         error
	seriesErr          error
	minerEffErr        error
	rackEffErr         error
	siteEffErr         error
	createChangeErr    error
	rollbackChangeErr  error
	listChangesErr     error
	lastReadingsFilter storage.ReadingsFilter
	lastSummaryFilter  storage.SummaryFilter
	lastSeriesFilter   storage.MinerSeriesFilter
	lastEffFilter      storage.EfficiencyFilter
	lastChangeInput    storage.RecommendationChangeCreateInput
	lastRollbackInput  storage.RecommendationRollbackInput
	lastChangeFilter   storage.RecommendationChangeFilter
}

func (r *stubRepository) PersistTelemetry(_ context.Context, _ telemetry.IngestRequest, _ []byte) error {
	return nil
}

func (r *stubRepository) ListReadings(_ context.Context, filter storage.ReadingsFilter) ([]storage.TelemetryReading, error) {
	r.lastReadingsFilter = filter
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.items, nil
}

func (r *stubRepository) SummarizeReadings(_ context.Context, filter storage.SummaryFilter) (storage.TelemetrySummary, error) {
	r.lastSummaryFilter = filter
	if r.summaryErr != nil {
		return storage.TelemetrySummary{}, r.summaryErr
	}
	return r.summary, nil
}

func (r *stubRepository) ListMinerSeries(_ context.Context, filter storage.MinerSeriesFilter) ([]storage.MinerSeriesPoint, error) {
	r.lastSeriesFilter = filter
	if r.seriesErr != nil {
		return nil, r.seriesErr
	}
	return r.series, nil
}

func (r *stubRepository) ListMinerEfficiency(_ context.Context, filter storage.EfficiencyFilter) ([]storage.MinerEfficiency, error) {
	r.lastEffFilter = filter
	if r.minerEffErr != nil {
		return nil, r.minerEffErr
	}
	return r.minerEfficiency, nil
}

func (r *stubRepository) ListRackEfficiency(_ context.Context, filter storage.EfficiencyFilter) ([]storage.RackEfficiency, error) {
	r.lastEffFilter = filter
	if r.rackEffErr != nil {
		return nil, r.rackEffErr
	}
	return r.rackEfficiency, nil
}

func (r *stubRepository) ListSiteEfficiency(_ context.Context, filter storage.EfficiencyFilter) ([]storage.SiteEfficiency, error) {
	r.lastEffFilter = filter
	if r.siteEffErr != nil {
		return nil, r.siteEffErr
	}
	return r.siteEfficiency, nil
}

func (r *stubRepository) CreateRecommendationChange(
	_ context.Context,
	input storage.RecommendationChangeCreateInput,
) (storage.RecommendationChange, error) {
	r.lastChangeInput = input
	if r.createChangeErr != nil {
		return storage.RecommendationChange{}, r.createChangeErr
	}

	if len(r.changes) > 0 {
		return r.changes[0], nil
	}

	return storage.RecommendationChange{
		ChangeID:    "change-1",
		MinerID:     input.MinerID,
		SiteID:      input.SiteID,
		RackID:      input.RackID,
		Operation:   storage.ChangeOperationApply,
		Status:      storage.ChangeStatusApplied,
		Reason:      input.Reason,
		RequestedBy: input.RequestedBy,
		Summary:     input.Summary,
		CreatedAt:   time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC),
	}, nil
}

func (r *stubRepository) RollbackRecommendationChange(
	_ context.Context,
	input storage.RecommendationRollbackInput,
) (storage.RecommendationChange, error) {
	r.lastRollbackInput = input
	if r.rollbackChangeErr != nil {
		return storage.RecommendationChange{}, r.rollbackChangeErr
	}

	if len(r.changes) > 0 {
		return r.changes[0], nil
	}

	return storage.RecommendationChange{
		ChangeID:       "change-rb-1",
		ParentChangeID: input.ChangeID,
		Operation:      storage.ChangeOperationRollback,
		Status:         storage.ChangeStatusApplied,
		Reason:         input.Reason,
		RequestedBy:    input.RequestedBy,
		CreatedAt:      time.Date(2026, 3, 25, 10, 1, 0, 0, time.UTC),
	}, nil
}

func (r *stubRepository) ListRecommendationChanges(
	_ context.Context,
	filter storage.RecommendationChangeFilter,
) ([]storage.RecommendationChange, error) {
	r.lastChangeFilter = filter
	if r.listChangesErr != nil {
		return nil, r.listChangesErr
	}
	return r.changes, nil
}

func (r *stubRepository) Close() {}

func buildTestRouter() (http.Handler, *stubPublisher) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{}
	return NewRouter(logger, publisher, "telemetry.raw.v1", nil, 1<<20, APIKeyAuthConfig{}), publisher
}

func TestAuthAllowsHealthzWithoutAPIKey(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{}
	router := NewRouter(logger, publisher, "telemetry.raw.v1", nil, 1<<20, APIKeyAuthConfig{
		Enabled: true,
		Header:  "X-API-Key",
		Keys:    []string{"test-key"},
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.Code)
	}
}

func TestAuthAllowsMetricsWithoutAPIKey(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{}
	router := NewRouter(logger, publisher, "telemetry.raw.v1", nil, 1<<20, APIKeyAuthConfig{
		Enabled: true,
		Header:  "X-API-Key",
		Keys:    []string{"test-key"},
	})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "fractsoul_http_requests_total") {
		t.Fatalf("expected metrics payload, got %s", resp.Body.String())
	}
}

func TestDashboardRouteIsServed(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{}
	router := NewRouter(logger, publisher, "telemetry.raw.v1", nil, 1<<20, APIKeyAuthConfig{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "Dashboard Operativo v0") {
		t.Fatalf("expected dashboard html body, got %s", resp.Body.String())
	}
}

func TestAuthRejectsMissingAPIKey(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{}
	router := NewRouter(logger, publisher, "telemetry.raw.v1", nil, 1<<20, APIKeyAuthConfig{
		Enabled: true,
		Header:  "X-API-Key",
		Keys:    []string{"test-key"},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/telemetry/readings", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "unauthorized") {
		t.Fatalf("expected unauthorized response, got %s", resp.Body.String())
	}
}

func TestAuthRejectsInvalidAPIKey(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{}
	router := NewRouter(logger, publisher, "telemetry.raw.v1", nil, 1<<20, APIKeyAuthConfig{
		Enabled: true,
		Header:  "X-API-Key",
		Keys:    []string{"test-key"},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/telemetry/readings", nil)
	req.Header.Set("X-API-Key", "wrong-key")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, resp.Code, resp.Body.String())
	}
}

func TestAuthAcceptsValidAPIKey(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{}
	router := NewRouter(logger, publisher, "telemetry.raw.v1", nil, 1<<20, APIKeyAuthConfig{
		Enabled: true,
		Header:  "X-API-Key",
		Keys:    []string{"test-key"},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/telemetry/readings", nil)
	req.Header.Set("X-API-Key", "test-key")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Repository is nil by design, so a valid key should reach the handler and return dependency_unavailable.
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusServiceUnavailable, resp.Code, resp.Body.String())
	}
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
	router := NewRouter(logger, publisher, "telemetry.raw.v1", nil, 256, APIKeyAuthConfig{})

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
	router := NewRouter(logger, publisher, "telemetry.raw.v1", nil, 1<<20, APIKeyAuthConfig{})

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
	router := NewRouter(logger, publisher, "telemetry.raw.v1", repo, 1<<20, APIKeyAuthConfig{})

	req := httptest.NewRequest(http.MethodGet, "/v1/telemetry/readings?site_id=SITE-CL-1&model=s21&limit=10", nil)
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

	if repo.lastReadingsFilter.Model != "S21" {
		t.Fatalf("expected model filter S21, got %s", repo.lastReadingsFilter.Model)
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
	router := NewRouter(logger, publisher, "telemetry.raw.v1", repo, 1<<20, APIKeyAuthConfig{})

	req := httptest.NewRequest(http.MethodGet, "/v1/telemetry/summary?window_minutes=120&model=s21", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, resp.Code, resp.Body.String())
	}

	if !strings.Contains(resp.Body.String(), "\"samples\":120") {
		t.Fatalf("expected summary payload in response, got %s", resp.Body.String())
	}

	if repo.lastSummaryFilter.Model != "S21" {
		t.Fatalf("expected summary model filter S21, got %s", repo.lastSummaryFilter.Model)
	}
}

func TestRackReadingsEndpointAppliesPathAndStatusFilters(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{}
	repo := &stubRepository{
		items: []storage.TelemetryReading{
			{
				Timestamp: time.Date(2026, 3, 24, 14, 0, 0, 0, time.UTC),
				EventID:   "550e8400-e29b-41d4-a716-446655440011",
				SiteID:    "site-cl-01",
				RackID:    "rack-cl-01-01",
				MinerID:   "asic-000001",
				Status:    telemetry.StatusWarning,
			},
		},
	}
	router := NewRouter(logger, publisher, "telemetry.raw.v1", repo, 1<<20, APIKeyAuthConfig{})

	req := httptest.NewRequest(
		http.MethodGet,
		"/v1/telemetry/sites/SITE-CL-1/racks/rack-a1/readings?status=warning&limit=10",
		nil,
	)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, resp.Code, resp.Body.String())
	}

	if repo.lastReadingsFilter.SiteID != "site-cl-01" {
		t.Fatalf("expected normalized site_id, got %s", repo.lastReadingsFilter.SiteID)
	}
	if repo.lastReadingsFilter.RackID != "rack-cl-01-01" {
		t.Fatalf("expected normalized rack_id, got %s", repo.lastReadingsFilter.RackID)
	}
	if repo.lastReadingsFilter.Status != telemetry.StatusWarning {
		t.Fatalf("expected status filter warning, got %s", repo.lastReadingsFilter.Status)
	}
}

func TestMinerTimeseriesEndpointReturnsSeries(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{}
	repo := &stubRepository{
		series: []storage.MinerSeriesPoint{
			{
				Bucket:         time.Date(2026, 3, 24, 14, 0, 0, 0, time.UTC),
				Samples:        12,
				AvgHashrateTHs: 131.5,
			},
		},
	}
	router := NewRouter(logger, publisher, "telemetry.raw.v1", repo, 1<<20, APIKeyAuthConfig{})

	req := httptest.NewRequest(
		http.MethodGet,
		"/v1/telemetry/miners/ASIC_42/timeseries?resolution=hour&from=2026-03-24T00:00:00Z&to=2026-03-24T12:00:00Z&limit=24",
		nil,
	)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, resp.Code, resp.Body.String())
	}

	if repo.lastSeriesFilter.MinerID != "asic-000042" {
		t.Fatalf("expected normalized miner_id, got %s", repo.lastSeriesFilter.MinerID)
	}
	if repo.lastSeriesFilter.Resolution != storage.ResolutionHour {
		t.Fatalf("expected resolution hour, got %s", repo.lastSeriesFilter.Resolution)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if count, ok := body["count"].(float64); !ok || int(count) != 1 {
		t.Fatalf("expected count=1, got %#v", body["count"])
	}
}

func TestMinerTimeseriesRejectsInvalidResolution(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{}
	repo := &stubRepository{}
	router := NewRouter(logger, publisher, "telemetry.raw.v1", repo, 1<<20, APIKeyAuthConfig{})

	req := httptest.NewRequest(
		http.MethodGet,
		"/v1/telemetry/miners/asic-000001/timeseries?resolution=day",
		nil,
	)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "validation_error") {
		t.Fatalf("expected validation_error response, got %s", resp.Body.String())
	}
}

func TestEfficiencyMinerEndpointReturnsItems(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{}
	repo := &stubRepository{
		minerEfficiency: []storage.MinerEfficiency{
			{
				SiteID:         "site-cl-01",
				RackID:         "rack-cl-01-01",
				MinerID:        "asic-000001",
				MinerModel:     "S21",
				Samples:        12,
				RawJTH:         18.1,
				CompensatedJTH: 17.8,
				ThermalBand:    "optimal",
			},
		},
	}
	router := NewRouter(logger, publisher, "telemetry.raw.v1", repo, 1<<20, APIKeyAuthConfig{})

	req := httptest.NewRequest(http.MethodGet, "/v1/efficiency/miners?site_id=site-cl-01&window_minutes=120&limit=50", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, resp.Code, resp.Body.String())
	}
	if repo.lastEffFilter.WindowMinutes != 120 {
		t.Fatalf("expected window_minutes=120, got %d", repo.lastEffFilter.WindowMinutes)
	}
	if !strings.Contains(resp.Body.String(), "\"compensated_jth\":17.8") {
		t.Fatalf("expected efficiency payload, got %s", resp.Body.String())
	}
}

func TestEfficiencyRackEndpointReturnsItems(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{}
	repo := &stubRepository{
		rackEfficiency: []storage.RackEfficiency{
			{
				SiteID:         "site-cl-01",
				RackID:         "rack-cl-01-01",
				Miners:         10,
				Samples:        120,
				RawJTH:         19.4,
				CompensatedJTH: 18.9,
			},
		},
	}
	router := NewRouter(logger, publisher, "telemetry.raw.v1", repo, 1<<20, APIKeyAuthConfig{})

	req := httptest.NewRequest(http.MethodGet, "/v1/efficiency/racks?site_id=site-cl-01&window_minutes=60&limit=20", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "\"miners\":10") {
		t.Fatalf("expected rack efficiency payload, got %s", resp.Body.String())
	}
}

func TestAnomalyEndpointReturnsReport(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{}
	repo := &stubRepository{
		items: []storage.TelemetryReading{
			{
				Timestamp:   time.Date(2026, 3, 24, 14, 0, 0, 0, time.UTC),
				SiteID:      "site-cl-01",
				RackID:      "rack-cl-01-01",
				MinerID:     "asic-000042",
				MinerModel:  "S21",
				TempCelsius: 95,
				Tags: map[string]string{
					"ambient_temp_c": "30",
				},
			},
		},
		series: []storage.MinerSeriesPoint{
			{Bucket: time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC), AvgHashrateTHs: 195, AvgPowerWatts: 3500, AvgTempCelsius: 88, MaxTempCelsius: 93, AvgFanRPM: 6500, AvgEfficiencyJTH: 18.0},
			{Bucket: time.Date(2026, 3, 24, 13, 0, 0, 0, time.UTC), AvgHashrateTHs: 182, AvgPowerWatts: 3510, AvgTempCelsius: 94, MaxTempCelsius: 98, AvgFanRPM: 7200, AvgEfficiencyJTH: 19.2},
		},
	}
	router := NewRouter(logger, publisher, "telemetry.raw.v1", repo, 1<<20, APIKeyAuthConfig{})

	req := httptest.NewRequest(
		http.MethodGet,
		"/v1/anomalies/miners/ASIC_42/analyze?resolution=hour&from=2026-03-24T12:00:00Z&to=2026-03-24T14:00:00Z&limit=24",
		nil,
	)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "\"summary_line\"") {
		t.Fatalf("expected anomaly summary_line in response, got %s", resp.Body.String())
	}
	if repo.lastSeriesFilter.MinerID != "asic-000042" {
		t.Fatalf("expected normalized miner_id for anomaly endpoint, got %s", repo.lastSeriesFilter.MinerID)
	}
}

func TestApplyRecommendationChangeEndpointPersistsChange(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{}
	repo := &stubRepository{
		items: []storage.TelemetryReading{
			{
				Timestamp:   time.Date(2026, 3, 24, 14, 0, 0, 0, time.UTC),
				SiteID:      "site-cl-01",
				RackID:      "rack-cl-01-01",
				MinerID:     "asic-000042",
				MinerModel:  "S21",
				TempCelsius: 95,
				Tags: map[string]string{
					"ambient_temp_c": "30",
				},
			},
		},
		series: []storage.MinerSeriesPoint{
			{Bucket: time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC), AvgHashrateTHs: 195, AvgPowerWatts: 3500, AvgTempCelsius: 88, MaxTempCelsius: 93, AvgFanRPM: 6500, AvgEfficiencyJTH: 18.0},
			{Bucket: time.Date(2026, 3, 24, 13, 0, 0, 0, time.UTC), AvgHashrateTHs: 182, AvgPowerWatts: 3510, AvgTempCelsius: 94, MaxTempCelsius: 98, AvgFanRPM: 7200, AvgEfficiencyJTH: 19.2},
		},
		changes: []storage.RecommendationChange{
			{
				ChangeID:    "change-apply-1",
				MinerID:     "asic-000042",
				SiteID:      "site-cl-01",
				RackID:      "rack-cl-01-01",
				Operation:   storage.ChangeOperationApply,
				Status:      storage.ChangeStatusApplied,
				Reason:      "operator dry-run",
				RequestedBy: "ops@test",
				CreatedAt:   time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC),
			},
		},
	}
	router := NewRouter(logger, publisher, "telemetry.raw.v1", repo, 1<<20, APIKeyAuthConfig{})

	body := `{"reason":"operator dry-run","requested_by":"ops@test"}`
	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/anomalies/miners/ASIC_42/changes/apply?resolution=hour&from=2026-03-24T12:00:00Z&to=2026-03-24T14:00:00Z&limit=24",
		bytes.NewBufferString(body),
	)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, resp.Code, resp.Body.String())
	}
	if repo.lastChangeInput.MinerID != "asic-000042" {
		t.Fatalf("expected normalized miner_id in change input, got %s", repo.lastChangeInput.MinerID)
	}
	if len(repo.lastChangeInput.Recommendations) == 0 {
		t.Fatalf("expected recommendations to be persisted in change input")
	}
	if !strings.Contains(resp.Body.String(), "\"change_id\":\"change-apply-1\"") {
		t.Fatalf("expected change_id in response, got %s", resp.Body.String())
	}
}

func TestRollbackRecommendationChangeEndpoint(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{}
	repo := &stubRepository{
		changes: []storage.RecommendationChange{
			{
				ChangeID:       "change-rb-1",
				ParentChangeID: "change-apply-1",
				Operation:      storage.ChangeOperationRollback,
				Status:         storage.ChangeStatusApplied,
				Reason:         "rollback requested by operator",
				RequestedBy:    "ops@test",
				CreatedAt:      time.Date(2026, 3, 25, 10, 1, 0, 0, time.UTC),
			},
		},
	}
	router := NewRouter(logger, publisher, "telemetry.raw.v1", repo, 1<<20, APIKeyAuthConfig{})

	body := `{"reason":"rollback requested by operator","requested_by":"ops@test"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/anomalies/changes/change-apply-1/rollback", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, resp.Code, resp.Body.String())
	}
	if repo.lastRollbackInput.ChangeID != "change-apply-1" {
		t.Fatalf("expected rollback input change id, got %s", repo.lastRollbackInput.ChangeID)
	}
	if !strings.Contains(resp.Body.String(), "\"change_id\":\"change-rb-1\"") {
		t.Fatalf("expected rollback change id in response, got %s", resp.Body.String())
	}
}

func TestListRecommendationChangesEndpoint(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &stubPublisher{}
	repo := &stubRepository{
		changes: []storage.RecommendationChange{
			{
				ChangeID:    "change-1",
				MinerID:     "asic-000042",
				Operation:   storage.ChangeOperationApply,
				Status:      storage.ChangeStatusApplied,
				Reason:      "apply",
				CreatedAt:   time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC),
				SiteID:      "site-cl-01",
				RackID:      "rack-cl-01-01",
				RequestedBy: "ops@test",
			},
			{
				ChangeID:    "change-2",
				MinerID:     "asic-000042",
				Operation:   storage.ChangeOperationRollback,
				Status:      storage.ChangeStatusApplied,
				Reason:      "rollback",
				CreatedAt:   time.Date(2026, 3, 25, 11, 0, 0, 0, time.UTC),
				SiteID:      "site-cl-01",
				RackID:      "rack-cl-01-01",
				RequestedBy: "ops@test",
			},
		},
	}
	router := NewRouter(logger, publisher, "telemetry.raw.v1", repo, 1<<20, APIKeyAuthConfig{})

	req := httptest.NewRequest(http.MethodGet, "/v1/anomalies/changes?miner_id=ASIC_42&status=applied&limit=5", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, resp.Code, resp.Body.String())
	}
	if repo.lastChangeFilter.MinerID != "asic-000042" {
		t.Fatalf("expected normalized miner_id in filter, got %s", repo.lastChangeFilter.MinerID)
	}
	if repo.lastChangeFilter.Status != storage.ChangeStatusApplied {
		t.Fatalf("expected applied status filter, got %s", repo.lastChangeFilter.Status)
	}
	if !strings.Contains(resp.Body.String(), "\"count\":2") {
		t.Fatalf("expected count=2 in response, got %s", resp.Body.String())
	}
}

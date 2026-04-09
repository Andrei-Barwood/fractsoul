package fractsoul

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/orchestrator"
)

type Client struct {
	baseURL      string
	apiKeyHeader string
	apiKey       string
	httpClient   *http.Client
}

type ContextEnrichment struct {
	Source                   string            `json:"source"`
	WindowMinutes            int               `json:"window_minutes"`
	SiteEfficiency           *SiteEfficiency   `json:"site_efficiency,omitempty"`
	RackEfficiency           []RackEfficiency  `json:"rack_efficiency,omitempty"`
	TelemetrySummary         *TelemetrySummary `json:"telemetry_summary,omitempty"`
	ConstrainedRackAnomalies []RackAnomaly     `json:"constrained_rack_anomalies,omitempty"`
	Warnings                 []string          `json:"warnings,omitempty"`
}

type ContextOptions struct {
	WindowMinutes int
	RackLimit     int
	RequestID     string
}

type SiteEfficiency struct {
	SiteID            string    `json:"site_id"`
	Miners            int64     `json:"miners"`
	Racks             int64     `json:"racks"`
	Samples           int64     `json:"samples"`
	AvgHashrateTHs    float64   `json:"avg_hashrate_ths"`
	AvgPowerWatts     float64   `json:"avg_power_watts"`
	AvgTempCelsius    float64   `json:"avg_temp_celsius"`
	AvgAmbientCelsius float64   `json:"avg_ambient_celsius"`
	RawJTH            float64   `json:"raw_jth"`
	CompensatedJTH    float64   `json:"compensated_jth"`
	LastSeenAt        time.Time `json:"last_seen_at"`
}

type RackEfficiency struct {
	SiteID            string    `json:"site_id"`
	RackID            string    `json:"rack_id"`
	Miners            int64     `json:"miners"`
	Samples           int64     `json:"samples"`
	AvgHashrateTHs    float64   `json:"avg_hashrate_ths"`
	AvgPowerWatts     float64   `json:"avg_power_watts"`
	AvgTempCelsius    float64   `json:"avg_temp_celsius"`
	AvgAmbientCelsius float64   `json:"avg_ambient_celsius"`
	RawJTH            float64   `json:"raw_jth"`
	CompensatedJTH    float64   `json:"compensated_jth"`
	LastSeenAt        time.Time `json:"last_seen_at"`
}

type TelemetrySummary struct {
	WindowMinutes    int     `json:"window_minutes"`
	Samples          int64   `json:"samples"`
	AvgHashrateTHs   float64 `json:"avg_hashrate_ths"`
	AvgPowerWatts    float64 `json:"avg_power_watts"`
	AvgTempCelsius   float64 `json:"avg_temp_celsius"`
	P95TempCelsius   float64 `json:"p95_temp_celsius"`
	MaxTempCelsius   float64 `json:"max_temp_celsius"`
	AvgFanRPM        float64 `json:"avg_fan_rpm"`
	AvgEfficiencyJTH float64 `json:"avg_efficiency_jth"`
}

type RackReading struct {
	MinerID string `json:"miner_id"`
}

type RackAnomaly struct {
	RackID                   string  `json:"rack_id"`
	MinerID                  string  `json:"miner_id"`
	SummaryLine              string  `json:"summary_line"`
	SeverityLabel            string  `json:"severity_label"`
	SeverityScore            float64 `json:"severity_score"`
	ProbableCause            string  `json:"probable_cause"`
	HotspotTriggered         bool    `json:"hotspot_triggered"`
	HashDegradationTriggered bool    `json:"hash_degradation_triggered"`
}

type siteEfficiencyResponse struct {
	Items []SiteEfficiency `json:"items"`
}

type rackEfficiencyResponse struct {
	Items []RackEfficiency `json:"items"`
}

type telemetrySummaryResponse struct {
	Summary TelemetrySummary `json:"summary"`
}

type rackReadingsResponse struct {
	Items []RackReading `json:"items"`
}

type anomalyResponse struct {
	SummaryLine string `json:"summary_line"`
	Report      struct {
		SeverityLabel   string  `json:"severity_label"`
		SeverityScore   float64 `json:"severity_score"`
		ProbableCause   string  `json:"probable_cause"`
		Hotspot         struct {
			Triggered bool `json:"triggered"`
		} `json:"hotspot"`
		HashDegradation struct {
			Triggered bool `json:"triggered"`
		} `json:"hash_degradation"`
	} `json:"report"`
}

func NewClient(baseURL, apiKeyHeader, apiKey string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return &Client{
		baseURL:      strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKeyHeader: strings.TrimSpace(apiKeyHeader),
		apiKey:       strings.TrimSpace(apiKey),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.baseURL != ""
}

func (c *Client) LoadContext(ctx context.Context, siteID string, budget orchestrator.SiteBudget, options ContextOptions) *ContextEnrichment {
	if !c.Enabled() {
		return nil
	}

	if options.WindowMinutes <= 0 {
		options.WindowMinutes = 60
	}
	if options.RackLimit <= 0 {
		options.RackLimit = 3
	}

	enrichment := &ContextEnrichment{
		Source:        "fractsoul-ingest-api",
		WindowMinutes: options.WindowMinutes,
	}

	siteEfficiency, err := c.getSiteEfficiency(ctx, siteID, options.WindowMinutes, options.RequestID)
	if err != nil {
		enrichment.Warnings = append(enrichment.Warnings, fmt.Sprintf("site_efficiency_unavailable: %v", err))
	} else {
		enrichment.SiteEfficiency = siteEfficiency
	}

	telemetrySummary, err := c.getTelemetrySummary(ctx, siteID, options.WindowMinutes, options.RequestID)
	if err != nil {
		enrichment.Warnings = append(enrichment.Warnings, fmt.Sprintf("telemetry_summary_unavailable: %v", err))
	} else {
		enrichment.TelemetrySummary = telemetrySummary
	}

	selectedRackIDs := selectConstrainedRacks(budget.Racks, options.RackLimit)
	for _, rackID := range selectedRackIDs {
		rackEfficiency, err := c.getRackEfficiency(ctx, siteID, rackID, options.WindowMinutes, options.RequestID)
		if err != nil {
			enrichment.Warnings = append(enrichment.Warnings, fmt.Sprintf("rack_efficiency_unavailable:%s:%v", rackID, err))
		} else if rackEfficiency != nil {
			enrichment.RackEfficiency = append(enrichment.RackEfficiency, *rackEfficiency)
		}

		reading, err := c.getLatestRackReading(ctx, siteID, rackID, options.RequestID)
		if err != nil {
			enrichment.Warnings = append(enrichment.Warnings, fmt.Sprintf("rack_reading_unavailable:%s:%v", rackID, err))
			continue
		}
		if reading == nil || strings.TrimSpace(reading.MinerID) == "" {
			continue
		}

		anomaly, err := c.getMinerAnomaly(ctx, reading.MinerID, options.RequestID)
		if err != nil {
			enrichment.Warnings = append(enrichment.Warnings, fmt.Sprintf("rack_anomaly_unavailable:%s:%v", rackID, err))
			continue
		}

		enrichment.ConstrainedRackAnomalies = append(enrichment.ConstrainedRackAnomalies, RackAnomaly{
			RackID:                   rackID,
			MinerID:                  reading.MinerID,
			SummaryLine:              anomaly.SummaryLine,
			SeverityLabel:            anomaly.Report.SeverityLabel,
			SeverityScore:            anomaly.Report.SeverityScore,
			ProbableCause:            anomaly.Report.ProbableCause,
			HotspotTriggered:         anomaly.Report.Hotspot.Triggered,
			HashDegradationTriggered: anomaly.Report.HashDegradation.Triggered,
		})
	}

	if len(enrichment.Warnings) == 0 {
		enrichment.Warnings = nil
	}
	if len(enrichment.RackEfficiency) == 0 {
		enrichment.RackEfficiency = nil
	}
	if len(enrichment.ConstrainedRackAnomalies) == 0 {
		enrichment.ConstrainedRackAnomalies = nil
	}

	return enrichment
}

func selectConstrainedRacks(racks []orchestrator.RackBudget, limit int) []string {
	if limit <= 0 {
		return nil
	}

	type candidate struct {
		rackID           string
		weight           int
		safeDispatchable float64
	}

	candidates := make([]candidate, 0, len(racks))
	for _, rack := range racks {
		weight := len(rack.ConstraintFlags)
		if rack.SafeDispatchableKW <= 0 {
			weight += 2
		}
		if rack.CurrentLoadKW > rack.SafeCapacityKW {
			weight += 3
		}
		if weight == 0 {
			continue
		}
		candidates = append(candidates, candidate{
			rackID:           rack.RackID,
			weight:           weight,
			safeDispatchable: rack.SafeDispatchableKW,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].weight == candidates[j].weight {
			if candidates[i].safeDispatchable == candidates[j].safeDispatchable {
				return candidates[i].rackID < candidates[j].rackID
			}
			return candidates[i].safeDispatchable < candidates[j].safeDispatchable
		}
		return candidates[i].weight > candidates[j].weight
	})

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	result := make([]string, 0, len(candidates))
	for _, item := range candidates {
		result = append(result, item.rackID)
	}
	return result
}

func (c *Client) getSiteEfficiency(ctx context.Context, siteID string, windowMinutes int, requestID string) (*SiteEfficiency, error) {
	values := url.Values{}
	values.Set("site_id", siteID)
	values.Set("window_minutes", fmt.Sprintf("%d", windowMinutes))
	values.Set("limit", "1")

	var response siteEfficiencyResponse
	if err := c.get(ctx, "/v1/efficiency/sites", values, requestID, &response); err != nil {
		return nil, err
	}
	if len(response.Items) == 0 {
		return nil, nil
	}
	return &response.Items[0], nil
}

func (c *Client) getRackEfficiency(ctx context.Context, siteID, rackID string, windowMinutes int, requestID string) (*RackEfficiency, error) {
	values := url.Values{}
	values.Set("site_id", siteID)
	values.Set("rack_id", rackID)
	values.Set("window_minutes", fmt.Sprintf("%d", windowMinutes))
	values.Set("limit", "1")

	var response rackEfficiencyResponse
	if err := c.get(ctx, "/v1/efficiency/racks", values, requestID, &response); err != nil {
		return nil, err
	}
	if len(response.Items) == 0 {
		return nil, nil
	}
	return &response.Items[0], nil
}

func (c *Client) getTelemetrySummary(ctx context.Context, siteID string, windowMinutes int, requestID string) (*TelemetrySummary, error) {
	values := url.Values{}
	values.Set("site_id", siteID)
	values.Set("window_minutes", fmt.Sprintf("%d", windowMinutes))

	var response telemetrySummaryResponse
	if err := c.get(ctx, "/v1/telemetry/summary", values, requestID, &response); err != nil {
		return nil, err
	}
	return &response.Summary, nil
}

func (c *Client) getLatestRackReading(ctx context.Context, siteID, rackID, requestID string) (*RackReading, error) {
	var response rackReadingsResponse
	if err := c.get(ctx, fmt.Sprintf("/v1/telemetry/sites/%s/racks/%s/readings", url.PathEscape(siteID), url.PathEscape(rackID)), url.Values{
		"limit": []string{"1"},
	}, requestID, &response); err != nil {
		return nil, err
	}
	if len(response.Items) == 0 {
		return nil, nil
	}
	return &response.Items[0], nil
}

func (c *Client) getMinerAnomaly(ctx context.Context, minerID, requestID string) (*anomalyResponse, error) {
	values := url.Values{}
	values.Set("resolution", "minute")
	values.Set("limit", "120")

	var response anomalyResponse
	if err := c.get(ctx, fmt.Sprintf("/v1/anomalies/miners/%s/analyze", url.PathEscape(minerID)), values, requestID, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) get(ctx context.Context, path string, query url.Values, requestID string, target any) error {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build request %s: %w", endpoint, err)
	}

	request.Header.Set("Accept", "application/json")
	if strings.TrimSpace(requestID) != "" {
		request.Header.Set("X-Request-ID", requestID)
	}
	if c.apiKey != "" {
		headerName := c.apiKeyHeader
		if headerName == "" {
			headerName = "X-API-Key"
		}
		request.Header.Set(headerName, c.apiKey)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("perform request %s: %w", endpoint, err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("request %s returned status %d", endpoint, response.StatusCode)
	}

	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode response %s: %w", endpoint, err)
	}

	return nil
}

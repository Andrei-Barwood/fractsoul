package storage

import (
	"context"
	"time"

	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/telemetry"
)

type ReadingsFilter struct {
	SiteID  string
	RackID  string
	MinerID string
	Model   string
	Status  telemetry.Status
	From    *time.Time
	To      *time.Time
	Limit   int
}

type SummaryFilter struct {
	SiteID        string
	RackID        string
	MinerID       string
	Model         string
	WindowMinutes int
}

type EfficiencyFilter struct {
	SiteID        string
	RackID        string
	MinerID       string
	Model         string
	WindowMinutes int
	Limit         int
}

type BucketResolution string

const (
	ResolutionMinute BucketResolution = "minute"
	ResolutionHour   BucketResolution = "hour"
)

type MinerSeriesFilter struct {
	MinerID    string
	From       time.Time
	To         time.Time
	Resolution BucketResolution
	Limit      int
}

type MinerSeriesPoint struct {
	Bucket           time.Time `json:"bucket"`
	Samples          int64     `json:"samples"`
	AvgHashrateTHs   float64   `json:"avg_hashrate_ths"`
	AvgPowerWatts    float64   `json:"avg_power_watts"`
	AvgTempCelsius   float64   `json:"avg_temp_celsius"`
	MaxTempCelsius   float64   `json:"max_temp_celsius"`
	AvgFanRPM        float64   `json:"avg_fan_rpm"`
	AvgEfficiencyJTH float64   `json:"avg_efficiency_jth"`
	CriticalEvents   int64     `json:"critical_events"`
}

type TelemetryReading struct {
	Timestamp       time.Time         `json:"timestamp"`
	EventID         string            `json:"event_id"`
	SiteID          string            `json:"site_id"`
	RackID          string            `json:"rack_id"`
	MinerID         string            `json:"miner_id"`
	MinerModel      string            `json:"miner_model,omitempty"`
	FirmwareVersion string            `json:"firmware_version,omitempty"`
	HashrateTHs     float64           `json:"hashrate_ths"`
	PowerWatts      float64           `json:"power_watts"`
	TempCelsius     float64           `json:"temp_celsius"`
	FanRPM          int               `json:"fan_rpm"`
	EfficiencyJTH   float64           `json:"efficiency_jth"`
	Status          telemetry.Status  `json:"status"`
	LoadPct         float64           `json:"load_pct"`
	Tags            map[string]string `json:"tags,omitempty"`
	IngestedAt      time.Time         `json:"ingested_at"`
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

type MinerEfficiency struct {
	SiteID            string    `json:"site_id"`
	RackID            string    `json:"rack_id"`
	MinerID           string    `json:"miner_id"`
	MinerModel        string    `json:"miner_model"`
	Samples           int64     `json:"samples"`
	AvgHashrateTHs    float64   `json:"avg_hashrate_ths"`
	AvgPowerWatts     float64   `json:"avg_power_watts"`
	AvgTempCelsius    float64   `json:"avg_temp_celsius"`
	AvgAmbientCelsius float64   `json:"avg_ambient_celsius"`
	RawJTH            float64   `json:"raw_jth"`
	CompensatedJTH    float64   `json:"compensated_jth"`
	BaselineJTH       float64   `json:"baseline_jth"`
	DeltaPct          float64   `json:"delta_pct"`
	ThermalBand       string    `json:"thermal_band"`
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

type Repository interface {
	PersistTelemetry(ctx context.Context, request telemetry.IngestRequest, rawPayload []byte) error
	ListReadings(ctx context.Context, filter ReadingsFilter) ([]TelemetryReading, error)
	SummarizeReadings(ctx context.Context, filter SummaryFilter) (TelemetrySummary, error)
	ListMinerSeries(ctx context.Context, filter MinerSeriesFilter) ([]MinerSeriesPoint, error)
	ListMinerEfficiency(ctx context.Context, filter EfficiencyFilter) ([]MinerEfficiency, error)
	ListRackEfficiency(ctx context.Context, filter EfficiencyFilter) ([]RackEfficiency, error)
	ListSiteEfficiency(ctx context.Context, filter EfficiencyFilter) ([]SiteEfficiency, error)
	Close()
}

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
	From    *time.Time
	To      *time.Time
	Limit   int
}

type SummaryFilter struct {
	SiteID        string
	RackID        string
	MinerID       string
	WindowMinutes int
}

type TelemetryReading struct {
	Timestamp       time.Time         `json:"timestamp"`
	EventID         string            `json:"event_id"`
	SiteID          string            `json:"site_id"`
	RackID          string            `json:"rack_id"`
	MinerID         string            `json:"miner_id"`
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

type Repository interface {
	PersistTelemetry(ctx context.Context, request telemetry.IngestRequest, rawPayload []byte) error
	ListReadings(ctx context.Context, filter ReadingsFilter) ([]TelemetryReading, error)
	SummarizeReadings(ctx context.Context, filter SummaryFilter) (TelemetrySummary, error)
	Close()
}

package anomaly

import (
	"testing"
	"time"

	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/storage"
)

func TestAnalyzeDetectsHotspot(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	series := []storage.MinerSeriesPoint{
		{Bucket: now.Add(-40 * time.Minute), AvgHashrateTHs: 190, AvgPowerWatts: 3450, AvgTempCelsius: 86, MaxTempCelsius: 92, AvgFanRPM: 6400, AvgEfficiencyJTH: 18.2},
		{Bucket: now.Add(-20 * time.Minute), AvgHashrateTHs: 186, AvgPowerWatts: 3465, AvgTempCelsius: 93, MaxTempCelsius: 97, AvgFanRPM: 7000, AvgEfficiencyJTH: 18.6},
		{Bucket: now, AvgHashrateTHs: 182, AvgPowerWatts: 3490, AvgTempCelsius: 98, MaxTempCelsius: 102, AvgFanRPM: 7300, AvgEfficiencyJTH: 19.2},
	}

	report := Analyze(storage.TelemetryReading{
		SiteID:      "site-cl-01",
		RackID:      "rack-cl-01-01",
		MinerID:     "asic-100001",
		MinerModel:  "S21",
		TempCelsius: 101,
		Tags: map[string]string{
			"ambient_temp_c": "32",
		},
	}, series, nil)

	if !report.Hotspot.Triggered {
		t.Fatalf("expected hotspot to be triggered")
	}
	if report.SeverityScore < 60 {
		t.Fatalf("expected severity >= 60, got %.2f", report.SeverityScore)
	}
}

func TestAnalyzeDetectsProgressiveHashDegradation(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	series := []storage.MinerSeriesPoint{
		{Bucket: now.Add(-60 * time.Minute), AvgHashrateTHs: 198, AvgPowerWatts: 3500, AvgTempCelsius: 74, MaxTempCelsius: 78, AvgFanRPM: 5900, AvgEfficiencyJTH: 17.7},
		{Bucket: now.Add(-40 * time.Minute), AvgHashrateTHs: 188, AvgPowerWatts: 3490, AvgTempCelsius: 75, MaxTempCelsius: 79, AvgFanRPM: 6000, AvgEfficiencyJTH: 18.5},
		{Bucket: now.Add(-20 * time.Minute), AvgHashrateTHs: 168, AvgPowerWatts: 3470, AvgTempCelsius: 76, MaxTempCelsius: 80, AvgFanRPM: 6100, AvgEfficiencyJTH: 20.6},
		{Bucket: now, AvgHashrateTHs: 152, AvgPowerWatts: 3460, AvgTempCelsius: 77, MaxTempCelsius: 81, AvgFanRPM: 6200, AvgEfficiencyJTH: 22.7},
	}

	report := Analyze(storage.TelemetryReading{
		SiteID:      "site-cl-01",
		RackID:      "rack-cl-01-02",
		MinerID:     "asic-100002",
		MinerModel:  "S21",
		TempCelsius: 77,
		Tags: map[string]string{
			"ambient_temp_c": "26",
		},
	}, series, nil)

	if !report.HashDegradation.Triggered {
		t.Fatalf("expected hash degradation to be triggered")
	}
	if report.Features.HashrateDropPct < 0.15 {
		t.Fatalf("expected hashrate drop pct >= 0.15, got %.3f", report.Features.HashrateDropPct)
	}
	if len(report.Recommendations) != 3 {
		t.Fatalf("expected 3 recommendations, got %d", len(report.Recommendations))
	}
}

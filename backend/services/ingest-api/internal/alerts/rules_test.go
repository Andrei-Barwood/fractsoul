package alerts

import (
	"testing"
	"time"

	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/telemetry"
)

func TestEngineTriggersOverheatRule(t *testing.T) {
	engine := NewEngine(DefaultRules())
	event := telemetry.IngestRequest{
		EventID:   "550e8400-e29b-41d4-a716-446655440001",
		Timestamp: time.Date(2026, 3, 24, 15, 0, 0, 0, time.UTC),
		SiteID:    "site-cl-01",
		RackID:    "rack-cl-01-01",
		MinerID:   "asic-000001",
		Metrics: telemetry.Metrics{
			HashrateTHs: 190,
			PowerWatts:  3450,
			TempCelsius: 99.2,
			FanRPM:      7100,
			Status:      telemetry.StatusCritical,
		},
		Tags: map[string]string{
			"asic_model": "s21",
		},
	}

	items := engine.Evaluate(event)
	if len(items) == 0 {
		t.Fatalf("expected at least one alert")
	}

	var found bool
	for _, item := range items {
		if item.RuleID != "overheat" {
			continue
		}
		found = true
		if item.Severity != SeverityCritical {
			t.Fatalf("expected overheat severity critical, got %s", item.Severity)
		}
		if item.MetricName != "temp_celsius" {
			t.Fatalf("expected temp_celsius metric, got %s", item.MetricName)
		}
	}
	if !found {
		t.Fatalf("expected overheat rule alert")
	}
}

func TestEngineTriggersPowerSpikeRule(t *testing.T) {
	engine := NewEngine(DefaultRules())
	event := telemetry.IngestRequest{
		EventID:   "550e8400-e29b-41d4-a716-446655440002",
		Timestamp: time.Date(2026, 3, 24, 15, 0, 0, 0, time.UTC),
		SiteID:    "site-cl-01",
		RackID:    "rack-cl-01-01",
		MinerID:   "asic-000002",
		Metrics: telemetry.Metrics{
			HashrateTHs: 196,
			PowerWatts:  4900,
			TempCelsius: 83.1,
			FanRPM:      6200,
			Status:      telemetry.StatusWarning,
		},
		Tags: map[string]string{
			"asic_model": "s21",
		},
	}

	items := engine.Evaluate(event)
	var found bool
	for _, item := range items {
		if item.RuleID == "power_spike" {
			found = true
			if item.Severity != SeverityCritical {
				t.Fatalf("expected power_spike severity critical, got %s", item.Severity)
			}
		}
	}
	if !found {
		t.Fatalf("expected power_spike rule alert")
	}
}

func TestEngineTriggersHashrateDropRule(t *testing.T) {
	engine := NewEngine(DefaultRules())
	event := telemetry.IngestRequest{
		EventID:   "550e8400-e29b-41d4-a716-446655440003",
		Timestamp: time.Date(2026, 3, 24, 15, 0, 0, 0, time.UTC),
		SiteID:    "site-cl-01",
		RackID:    "rack-cl-01-02",
		MinerID:   "asic-000003",
		Metrics: telemetry.Metrics{
			HashrateTHs: 80,
			PowerWatts:  2900,
			TempCelsius: 79,
			FanRPM:      5900,
			Status:      telemetry.StatusWarning,
		},
		Tags: map[string]string{
			"asic_model": "s19xp",
		},
	}

	items := engine.Evaluate(event)
	var found bool
	for _, item := range items {
		if item.RuleID == "hashrate_drop" {
			found = true
			if item.Severity != SeverityWarning {
				t.Fatalf("expected hashrate_drop severity warning, got %s", item.Severity)
			}
		}
	}
	if !found {
		t.Fatalf("expected hashrate_drop rule alert")
	}
}

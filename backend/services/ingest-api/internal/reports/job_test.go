package reports

import (
	"strings"
	"testing"
	"time"
)

func TestParseSchedule(t *testing.T) {
	hour, minute, err := parseSchedule("09:30")
	if err != nil {
		t.Fatalf("expected valid schedule, got err: %v", err)
	}
	if hour != 9 || minute != 30 {
		t.Fatalf("expected 09:30 parsed as 9,30 got %d,%d", hour, minute)
	}
}

func TestParseScheduleRejectsInvalid(t *testing.T) {
	_, _, err := parseSchedule("25:61")
	if err == nil {
		t.Fatalf("expected error for invalid schedule")
	}
}

func TestNextRunAt(t *testing.T) {
	loc := time.FixedZone("TEST", -3*60*60)
	now := time.Date(2026, 3, 25, 8, 30, 0, 0, loc)
	next := NextRunAt(now, 8, 0)

	expected := time.Date(2026, 3, 26, 8, 0, 0, 0, loc)
	if !next.Equal(expected) {
		t.Fatalf("expected %s got %s", expected, next)
	}
}

func TestRenderExecutiveOperationalMarkdown(t *testing.T) {
	loc := time.FixedZone("TEST", -3*60*60)
	report := BuildReport(
		time.Date(2026, 3, 24, 0, 0, 0, 0, loc),
		loc,
		time.Date(2026, 3, 24, 3, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 25, 3, 0, 0, 0, time.UTC),
		DailyMetrics{
			Global: GlobalMetrics{
				Samples:          200,
				ActiveMiners:     100,
				AvgHashrateTHs:   180.5,
				AvgPowerWatts:    3450.1,
				AvgTempCelsius:   79.4,
				AvgEfficiencyJTH: 19.2,
				CriticalEvents:   4,
				WarningEvents:    18,
			},
		},
		time.Date(2026, 3, 25, 8, 0, 0, 0, loc),
	)

	markdown := RenderExecutiveOperationalMarkdown(report)
	if !strings.Contains(markdown, "Reporte Diario Ejecutivo-Operativo") {
		t.Fatalf("expected markdown title")
	}
	if !strings.Contains(markdown, "KPIs Globales") {
		t.Fatalf("expected kpi section in markdown")
	}
	if !strings.Contains(markdown, "Plan de Accion (24h)") {
		t.Fatalf("expected action section in markdown")
	}
}

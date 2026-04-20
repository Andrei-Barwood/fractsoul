package orchestrator

import (
	"strings"
	"testing"
	"time"
)

func TestComputeSiteBudgetDeratesDegradedTransformer(t *testing.T) {
	input := BudgetInput{
		At: time.Date(2026, 4, 20, 14, 0, 0, 0, time.UTC),
		Site: SiteProfile{
			SiteID:                 "site-cl-01",
			CampusName:             "Copiapo Norte",
			TargetCapacityMW:       1,
			OperatingReservePct:    10,
			AmbientReferenceC:      25,
			AmbientDerateStartC:    30,
			AmbientDeratePctPerDeg: 1,
			AdvisoryMode:           "advisory-first",
		},
		AmbientCelsius: 34,
		Transformers: []CapacityAsset{
			{
				ID:                     "tx-a",
				Kind:                   AssetKindTransformer,
				SiteID:                 "site-cl-01",
				Name:                   "TX A",
				NominalCapacityKW:      300,
				OperatingMarginPct:     10,
				AmbientDerateStartC:    30,
				AmbientDeratePctPerDeg: 1,
				Status:                 StatusDegraded,
			},
		},
		Feeders: []CapacityAsset{
			{
				ID:                     "feeder-a",
				Kind:                   AssetKindFeeder,
				SiteID:                 "site-cl-01",
				Name:                   "Feeder A",
				NominalCapacityKW:      300,
				OperatingMarginPct:     8,
				AmbientDerateStartC:    30,
				AmbientDeratePctPerDeg: 1,
				Status:                 StatusActive,
			},
		},
		Racks: []RackProfile{
			{
				RackID:                "rack-a",
				SiteID:                "site-cl-01",
				FeederID:              "feeder-a",
				NominalCapacityKW:     120,
				OperatingMarginPct:    10,
				ThermalDensityLimitKW: 140,
				Status:                StatusActive,
			},
		},
		CurrentRackLoadKW: map[string]float64{
			"rack-a": 90,
		},
	}

	budget := ComputeSiteBudget(input)

	if len(budget.Transformers) != 1 {
		t.Fatalf("expected one transformer budget, got %d", len(budget.Transformers))
	}
	if budget.Transformers[0].EffectiveCapacityKW >= 300 {
		t.Fatalf("expected degraded transformer effective capacity to be derated, got %.2f", budget.Transformers[0].EffectiveCapacityKW)
	}
	if budget.SafeCapacityKW > budget.Transformers[0].SafeCapacityKW {
		t.Fatalf("expected site safe capacity to be capped by degraded transformer, got site %.2f > transformer %.2f", budget.SafeCapacityKW, budget.Transformers[0].SafeCapacityKW)
	}
}

func TestValidateDispatchRejectsFeederOutOfService(t *testing.T) {
	budget := SiteBudget{
		SiteID:              "site-cl-02",
		PolicyMode:          "advisory-first",
		CalculatedAt:        time.Date(2026, 4, 20, 18, 0, 0, 0, time.UTC),
		SafeCapacityKW:      500,
		CurrentLoadKW:       250,
		AvailableCapacityKW: 250,
		Feeders: []AssetBudget{
			{
				ID:                  "feeder-out",
				Kind:                AssetKindFeeder,
				CurrentLoadKW:       45,
				SafeCapacityKW:      0,
				AvailableCapacityKW: 0,
				Status:              StatusInactive,
			},
		},
		Racks: []RackBudget{
			{
				RackID:                "rack-out",
				FeederID:              "feeder-out",
				CriticalityClass:      LoadClassNormalProduction,
				CriticalityRank:       2,
				CurrentLoadKW:         45,
				SafeCapacityKW:        80,
				AvailableCapacityKW:   35,
				ThermalDensityLimitKW: 120,
				ThermalHeadroomKW:     75,
				UpRampRemainingKW:     20,
				DownRampRemainingKW:   20,
			},
		},
	}

	result := ValidateDispatch(budget, []DispatchRequest{{RackID: "rack-out", DeltaKW: 10}}, "ops@fractsoul.local")

	if result.SummaryStatus != "rejected" {
		t.Fatalf("expected rejected summary status, got %q", result.SummaryStatus)
	}
	if len(result.Decisions) != 1 {
		t.Fatalf("expected one decision, got %d", len(result.Decisions))
	}
	if result.Decisions[0].Status != "rejected" {
		t.Fatalf("expected rejected dispatch decision, got %q", result.Decisions[0].Status)
	}
	if len(result.Decisions[0].Violations) == 0 || result.Decisions[0].Violations[0].Code != "feeder_capacity_exceeded" {
		t.Fatalf("expected feeder_capacity_exceeded violation, got %#v", result.Decisions[0].Violations)
	}
}

func TestProjectSiteRiskFlagsExtremeClimateAndExpensiveTariff(t *testing.T) {
	now := time.Date(2026, 4, 20, 20, 0, 0, 0, time.UTC)
	points := ProjectSiteRisk(RiskProjectionInput{
		At: now,
		Site: SiteProfile{
			SiteID:                 "site-cl-03",
			TargetCapacityMW:       1,
			OperatingReservePct:    10,
			AmbientReferenceC:      25,
			AmbientDerateStartC:    30,
			AmbientDeratePctPerDeg: 1,
		},
		Budget: SiteBudget{
			SiteID:         "site-cl-03",
			CurrentLoadKW:  860,
			SafeCapacityKW: 900,
			AmbientCelsius: 36,
		},
		Samples: []SiteProjectionSample{
			{Bucket: now.Add(-3 * time.Hour), LoadKW: 780, AmbientCelsius: 31},
			{Bucket: now.Add(-2 * time.Hour), LoadKW: 820, AmbientCelsius: 33},
			{Bucket: now.Add(-1 * time.Hour), LoadKW: 860, AmbientCelsius: 36, CriticalEvents: 2, SensorErrorRate: 0.14},
		},
		CurrentTariff: &TariffWindow{
			SiteID:          "site-cl-03",
			TariffCode:      "peak_response",
			PriceUSDPerMWh:  168,
			IsExpensiveBand: true,
		},
	})

	if len(points) != 4 {
		t.Fatalf("expected four projected risk points, got %d", len(points))
	}
	if points[0].RiskScore <= 25 {
		t.Fatalf("expected materially elevated risk score, got %.2f", points[0].RiskScore)
	}
	reasons := strings.Join(points[0].Reasons, ",")
	if !strings.Contains(reasons, "expensive_tariff_active") {
		t.Fatalf("expected expensive tariff reason, got %q", reasons)
	}
	if !strings.Contains(reasons, "sensor_quality_degraded") {
		t.Fatalf("expected sensor quality degraded reason, got %q", reasons)
	}
}

func TestRunShadowPilotTracksMissingDataAndEscalation(t *testing.T) {
	day := time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC)
	result := RunShadowPilot(HistoricalReplayInput{
		Day: day,
		Site: SiteProfile{
			SiteID:                 "site-cl-04",
			TargetCapacityMW:       0.10,
			OperatingReservePct:    10,
			AmbientReferenceC:      25,
			AmbientDerateStartC:    30,
			AmbientDeratePctPerDeg: 0.5,
			RampUpKWPerInterval:    10,
			RampDownKWPerInterval:  15,
			RampIntervalSeconds:    300,
			AdvisoryMode:           "advisory-first",
		},
		Points: []HistoricalRackPoint{
			{
				Bucket:                day.Add(5 * time.Minute),
				RackID:                "rack-pref",
				CriticalityClass:      LoadClassPreferredProduction,
				CurrentLoadKW:         70,
				AvgHashrateTHs:        0,
				AvgPowerWatts:         70000,
				AvgAmbientCelsius:     0,
				NominalHashrateTHs:    0,
				NominalPowerWatts:     0,
				RampUpKWPerInterval:   10,
				RampDownKWPerInterval: 15,
			},
			{
				Bucket:                day.Add(5 * time.Minute),
				RackID:                "rack-sac",
				CriticalityClass:      LoadClassSacrificableLoad,
				CurrentLoadKW:         5,
				AvgHashrateTHs:        40,
				AvgPowerWatts:         5000,
				AvgAmbientCelsius:     29,
				NominalHashrateTHs:    50,
				NominalPowerWatts:     6000,
				RampUpKWPerInterval:   10,
				RampDownKWPerInterval: 15,
			},
			{
				Bucket:                day.Add(5 * time.Minute),
				RackID:                "rack-norm",
				CriticalityClass:      LoadClassNormalProduction,
				CurrentLoadKW:         30,
				AvgHashrateTHs:        120,
				AvgPowerWatts:         30000,
				AvgAmbientCelsius:     29,
				NominalHashrateTHs:    125,
				NominalPowerWatts:     31000,
				RampUpKWPerInterval:   10,
				RampDownKWPerInterval: 15,
			},
		},
	})

	if result.DecisionsWouldEscalate <= 0 {
		t.Fatalf("expected at least one overload interval requiring escalation, got %+v", result)
	}
	if result.MissingDataCount <= 0 {
		t.Fatalf("expected missing data to be detected, got %+v", result)
	}
	if len(result.MissingData) == 0 {
		t.Fatalf("expected missing data details to be populated")
	}
}

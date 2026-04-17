package orchestrator

import (
	"testing"
	"time"
)

func TestComputeSiteBudgetAppliesCriticalityAndRampPolicies(t *testing.T) {
	now := time.Date(2026, 4, 8, 15, 0, 0, 0, time.UTC)
	input := BudgetInput{
		At: now,
		Site: SiteProfile{
			SiteID:                 "site-cl-01",
			CampusName:             "Copiapo Norte",
			TargetCapacityMW:       1,
			OperatingReservePct:    10,
			AmbientReferenceC:      25,
			AmbientDerateStartC:    30,
			AmbientDeratePctPerDeg: 1,
			RampUpKWPerInterval:    15,
			RampDownKWPerInterval:  20,
			RampIntervalSeconds:    300,
			AdvisoryMode:           "advisory-first",
		},
		AmbientCelsius: 35,
		Feeders: []CapacityAsset{
			{
				ID:                     "feeder-a",
				Kind:                   AssetKindFeeder,
				SiteID:                 "site-cl-01",
				Name:                   "Feeder A",
				NominalCapacityKW:      250,
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
				CriticalityClass:      LoadClassPreferredProduction,
				CriticalityReason:     "preferred production lane",
				Status:                StatusActive,
			},
			{
				RackID:                "rack-b",
				SiteID:                "site-cl-01",
				FeederID:              "feeder-a",
				NominalCapacityKW:     120,
				OperatingMarginPct:    10,
				ThermalDensityLimitKW: 95,
				CriticalityClass:      LoadClassSacrificableLoad,
				RampUpKWPerInterval:   8,
				RampDownKWPerInterval: 12,
				Status:                StatusActive,
			},
		},
		CurrentRackLoadKW: map[string]float64{
			"rack-a": 80,
			"rack-b": 95,
		},
	}

	budget := ComputeSiteBudget(input)

	if budget.PolicyMode != "advisory-first" {
		t.Fatalf("expected advisory-first policy mode, got %q", budget.PolicyMode)
	}
	if budget.RampPolicy.IntervalSeconds != 300 {
		t.Fatalf("expected 300 second ramp interval, got %d", budget.RampPolicy.IntervalSeconds)
	}
	if budget.SafeDispatchableKW != 15 {
		t.Fatalf("expected site safe dispatchable limited by ramp to 15, got %.2f", budget.SafeDispatchableKW)
	}
	if len(budget.Racks) != 2 {
		t.Fatalf("expected 2 rack budgets, got %d", len(budget.Racks))
	}
	if budget.Racks[1].CriticalityClass != LoadClassSacrificableLoad {
		t.Fatalf("expected rack-b sacrificable class, got %q", budget.Racks[1].CriticalityClass)
	}
	if budget.Racks[1].SafeDispatchableKW != 0 {
		t.Fatalf("expected rack-b dispatch to be exhausted by thermal limit, got %.2f", budget.Racks[1].SafeDispatchableKW)
	}
	if budget.Racks[1].DownRampRemainingKW != 12 {
		t.Fatalf("expected rack-b down-ramp remaining 12, got %.2f", budget.Racks[1].DownRampRemainingKW)
	}
}

func TestValidateDispatchPrioritizesPreferredRackUnderSiteRamp(t *testing.T) {
	budget := SiteBudget{
		SiteID:              "site-cl-01",
		PolicyMode:          "advisory-first",
		CalculatedAt:        time.Date(2026, 4, 8, 15, 0, 0, 0, time.UTC),
		SafeCapacityKW:      500,
		CurrentLoadKW:       420,
		AvailableCapacityKW: 80,
		RampPolicy: RampPolicy{
			IntervalSeconds:       300,
			RampUpKWPerInterval:   15,
			RampDownKWPerInterval: 20,
		},
		Feeders: []AssetBudget{
			{
				ID:                  "feeder-a",
				Kind:                AssetKindFeeder,
				CurrentLoadKW:       233.8,
				SafeCapacityKW:      240,
				AvailableCapacityKW: 30,
			},
		},
		Racks: []RackBudget{
			{
				RackID:                "rack-a",
				FeederID:              "feeder-a",
				CriticalityClass:      LoadClassPreferredProduction,
				CriticalityRank:       3,
				CurrentLoadKW:         100,
				SafeCapacityKW:        120,
				AvailableCapacityKW:   20,
				ThermalDensityLimitKW: 140,
				ThermalHeadroomKW:     40,
				RampUpLimitKW:         20,
				UpRampRemainingKW:     20,
				DownRampRemainingKW:   20,
			},
			{
				RackID:                "rack-b",
				FeederID:              "feeder-a",
				CriticalityClass:      LoadClassSacrificableLoad,
				CriticalityRank:       1,
				CurrentLoadKW:         90,
				SafeCapacityKW:        120,
				AvailableCapacityKW:   30,
				ThermalDensityLimitKW: 140,
				ThermalHeadroomKW:     50,
				RampUpLimitKW:         30,
				UpRampRemainingKW:     30,
				DownRampRemainingKW:   20,
			},
		},
	}

	result := ValidateDispatch(budget, []DispatchRequest{
		{RackID: "rack-b", DeltaKW: 10},
		{RackID: "rack-a", DeltaKW: 12},
	}, "ops@fractsoul.local")

	if result.SummaryStatus != "partial" {
		t.Fatalf("expected partial summary, got %q", result.SummaryStatus)
	}
	if len(result.Decisions) != 2 {
		t.Fatalf("expected 2 decisions, got %d", len(result.Decisions))
	}
	if result.Decisions[0].Status != "partial" {
		t.Fatalf("expected rack-b request to be partial after preferred allocation, got %q", result.Decisions[0].Status)
	}
	if result.Decisions[0].AcceptedDeltaKW != 3 {
		t.Fatalf("expected rack-b accepted 3 kW after preferred dispatch, got %.2f", result.Decisions[0].AcceptedDeltaKW)
	}
	if result.Decisions[1].Status != "accepted" {
		t.Fatalf("expected rack-a request to be fully accepted, got %q", result.Decisions[1].Status)
	}
	if result.Decisions[1].AcceptedDeltaKW != 12 {
		t.Fatalf("expected rack-a accepted 12 kW, got %.2f", result.Decisions[1].AcceptedDeltaKW)
	}
}

func TestBuildOperationalViewIncludesReadableRecommendations(t *testing.T) {
	budget := SiteBudget{
		SiteID:              "site-cl-02",
		PolicyMode:          "advisory-first",
		CalculatedAt:        time.Date(2026, 4, 9, 10, 0, 0, 0, time.UTC),
		SafeCapacityKW:      180,
		CurrentLoadKW:       210,
		AvailableCapacityKW: 0,
		RampPolicy: RampPolicy{
			IntervalSeconds:       300,
			RampUpKWPerInterval:   20,
			RampDownKWPerInterval: 35,
		},
		Racks: []RackBudget{
			{
				RackID:              "rack-safe",
				CriticalityClass:    LoadClassSafetyBlocked,
				CriticalityRank:     0,
				SafetyBlocked:       true,
				SafetyBlockReason:   "aisle isolation remains active",
				CurrentLoadKW:       20,
				AvailableCapacityKW: 0,
				DownRampRemainingKW: 20,
			},
			{
				RackID:              "rack-sac",
				CriticalityClass:    LoadClassSacrificableLoad,
				CriticalityRank:     1,
				CurrentLoadKW:       50,
				AvailableCapacityKW: 10,
				DownRampRemainingKW: 15,
			},
		},
	}

	view := BuildOperationalView(budget)

	if len(view.ActiveConstraints) == 0 {
		t.Fatalf("expected active constraints to be present")
	}
	if len(view.PendingRecommendations) < 2 {
		t.Fatalf("expected at least 2 pending recommendations, got %d", len(view.PendingRecommendations))
	}
	if view.PendingRecommendations[0].Action != "isolate" {
		t.Fatalf("expected first recommendation to isolate safety-blocked rack, got %q", view.PendingRecommendations[0].Action)
	}
	if len(view.BlockedActions) == 0 {
		t.Fatalf("expected blocked actions to be present")
	}
	if len(view.Explanations) == 0 {
		t.Fatalf("expected decision explanations to be present")
	}
}

func TestRunHistoricalReplayBuildsScenarioComparisons(t *testing.T) {
	day := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	result := RunHistoricalReplay(HistoricalReplayInput{
		Day: day,
		Site: SiteProfile{
			SiteID:                 "site-cl-01",
			TargetCapacityMW:       1,
			OperatingReservePct:    10,
			AmbientDerateStartC:    30,
			AmbientDeratePctPerDeg: 0.5,
			RampUpKWPerInterval:    20,
			RampDownKWPerInterval:  30,
			RampIntervalSeconds:    300,
			AdvisoryMode:           "advisory-first",
		},
		ObservedPersistedAlerts: 2,
		Points: []HistoricalRackPoint{
			{
				Bucket:                day.Add(5 * time.Minute),
				RackID:                "rack-a",
				MinerModel:            "S21",
				CriticalityClass:      LoadClassPreferredProduction,
				CurrentLoadKW:         90,
				AvgHashrateTHs:        300,
				AvgPowerWatts:         90000,
				AvgTempCelsius:        74,
				MaxTempCelsius:        82,
				AvgAmbientCelsius:     26,
				AvgEfficiencyJTH:      300,
				NominalHashrateTHs:    320,
				NominalPowerWatts:     95000,
				RampUpKWPerInterval:   20,
				RampDownKWPerInterval: 30,
			},
			{
				Bucket:                day.Add(5 * time.Minute),
				RackID:                "rack-b",
				MinerModel:            "S19XP",
				CriticalityClass:      LoadClassSacrificableLoad,
				CurrentLoadKW:         70,
				AvgHashrateTHs:        210,
				AvgPowerWatts:         70000,
				AvgTempCelsius:        80,
				MaxTempCelsius:        92,
				AvgAmbientCelsius:     26,
				AvgEfficiencyJTH:      333.33,
				NominalHashrateTHs:    240,
				NominalPowerWatts:     76000,
				RampUpKWPerInterval:   10,
				RampDownKWPerInterval: 15,
			},
		},
	})

	if result.ObservedPersistedAlerts != 2 {
		t.Fatalf("expected observed persisted alerts 2, got %d", result.ObservedPersistedAlerts)
	}
	if len(result.Scenarios) != 2 {
		t.Fatalf("expected 2 replay scenarios, got %d", len(result.Scenarios))
	}
	if result.Observed.EstimatedAlertCount == 0 {
		t.Fatalf("expected observed scenario to estimate alerts")
	}
	if result.Scenarios[0].Name == "" || result.Scenarios[1].Name == "" {
		t.Fatalf("expected replay scenario names to be populated")
	}
}

package orchestrator

import (
	"testing"
	"time"
)

func TestComputeSiteBudgetAppliesDeratingAndRackConstraints(t *testing.T) {
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
				Status:                StatusActive,
			},
			{
				RackID:                "rack-b",
				SiteID:                "site-cl-01",
				FeederID:              "feeder-a",
				NominalCapacityKW:     120,
				OperatingMarginPct:    10,
				ThermalDensityLimitKW: 140,
				Status:                StatusActive,
			},
		},
		CurrentRackLoadKW: map[string]float64{
			"rack-a": 112,
			"rack-b": 90,
		},
	}

	budget := ComputeSiteBudget(input)

	if budget.PolicyMode != "advisory-first" {
		t.Fatalf("expected advisory-first policy mode, got %q", budget.PolicyMode)
	}
	if budget.CurrentLoadKW != 202 {
		t.Fatalf("expected current load 202, got %.2f", budget.CurrentLoadKW)
	}
	if budget.AvailableCapacityKW <= 0 {
		t.Fatalf("expected positive site available capacity, got %.2f", budget.AvailableCapacityKW)
	}
	if len(budget.Racks) != 2 {
		t.Fatalf("expected 2 rack budgets, got %d", len(budget.Racks))
	}
	if budget.Racks[0].RackID != "rack-a" {
		t.Fatalf("expected rack-a first after sorting, got %s", budget.Racks[0].RackID)
	}
	if budget.Racks[0].SafeDispatchableKW != 0 {
		t.Fatalf("expected rack-a safe dispatchable to be exhausted, got %.2f", budget.Racks[0].SafeDispatchableKW)
	}
}

func TestValidateDispatchRejectsOnFeederAndRackConstraints(t *testing.T) {
	budget := SiteBudget{
		SiteID:             "site-cl-01",
		PolicyMode:         "advisory-first",
		CalculatedAt:       time.Date(2026, 4, 8, 15, 0, 0, 0, time.UTC),
		SafeCapacityKW:     500,
		CurrentLoadKW:      420,
		AvailableCapacityKW: 80,
		Feeders: []AssetBudget{
			{
				ID:                  "feeder-a",
				Kind:                AssetKindFeeder,
				CurrentLoadKW:       233.8,
				SafeCapacityKW:      240,
				AvailableCapacityKW: 6.2,
			},
		},
		Racks: []RackBudget{
			{
				RackID:                "rack-a",
				FeederID:              "feeder-a",
				CurrentLoadKW:         112.4,
				SafeCapacityKW:        108,
				AvailableCapacityKW:   0,
				ThermalDensityLimitKW: 140,
				ThermalHeadroomKW:     27.6,
			},
			{
				RackID:                "rack-b",
				FeederID:              "feeder-a",
				CurrentLoadKW:         90,
				SafeCapacityKW:        120,
				AvailableCapacityKW:   30,
				ThermalDensityLimitKW: 140,
				ThermalHeadroomKW:     50,
			},
		},
	}

	result := ValidateDispatch(budget, []DispatchRequest{
		{RackID: "rack-a", DeltaKW: 8},
		{RackID: "rack-b", DeltaKW: 15},
	}, "ops@fractsoul.local")

	if result.SummaryStatus != "partial" {
		t.Fatalf("expected partial summary, got %q", result.SummaryStatus)
	}
	if len(result.Decisions) != 2 {
		t.Fatalf("expected 2 decisions, got %d", len(result.Decisions))
	}
	if result.Decisions[0].Status != "rejected" {
		t.Fatalf("expected rack-a rejected, got %q", result.Decisions[0].Status)
	}
	if result.Decisions[1].Status != "partial" {
		t.Fatalf("expected rack-b partial, got %q", result.Decisions[1].Status)
	}
	if result.Decisions[1].AcceptedDeltaKW != 6.2 {
		t.Fatalf("expected rack-b accepted 6.2, got %.2f", result.Decisions[1].AcceptedDeltaKW)
	}
}

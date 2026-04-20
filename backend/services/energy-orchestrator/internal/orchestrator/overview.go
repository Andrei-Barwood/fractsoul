package orchestrator

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

func BuildSiteOverview(site SiteProfile, view OperationalView, projection []SiteRiskProjectionPoint, currentTariff *TariffWindow) SiteOverview {
	sacrificableCount := 0
	safetyBlockedCount := 0
	for _, rack := range view.Budget.Racks {
		switch normalizeLoadCriticalityClass(string(rack.CriticalityClass)) {
		case LoadClassSacrificableLoad:
			sacrificableCount++
		case LoadClassSafetyBlocked:
			safetyBlockedCount++
		}
	}

	topRisks := make([]string, 0, 4)
	for _, constraint := range view.ActiveConstraints {
		if constraint.Severity == "critical" || constraint.Severity == "warning" {
			topRisks = append(topRisks, constraint.Code)
		}
	}
	if currentTariff != nil && currentTariff.IsExpensiveBand {
		topRisks = append(topRisks, "expensive_tariff_active")
	}
	topRisks = dedupeAndSort(topRisks)
	if len(topRisks) > 4 {
		topRisks = topRisks[:4]
	}

	overview := SiteOverview{
		SiteID:                     site.SiteID,
		CampusName:                 site.CampusName,
		CalculatedAt:               view.CalculatedAt,
		CurrentLoadKW:              view.Budget.CurrentLoadKW,
		AllowedLoadKW:              view.Budget.SafeCapacityKW,
		MarginRemainingKW:          view.Budget.AvailableCapacityKW,
		SacrificableRackCount:      sacrificableCount,
		SafetyBlockedRackCount:     safetyBlockedCount,
		ActiveConstraintCount:      len(view.ActiveConstraints),
		PendingRecommendationCount: len(view.PendingRecommendations),
		TopRisks:                   topRisks,
		RiskProjection:             projection,
	}
	if currentTariff != nil {
		overview.CurrentTariffCode = currentTariff.TariffCode
		overview.CurrentTariffPriceUSDPerMWh = currentTariff.PriceUSDPerMWh
		overview.CurrentTariffExpensive = currentTariff.IsExpensiveBand
	}
	return overview
}

func ProjectSiteRisk(input RiskProjectionInput) []SiteRiskProjectionPoint {
	if input.At.IsZero() {
		input.At = time.Now().UTC()
	}

	samples := make([]SiteProjectionSample, 0, len(input.Samples))
	for _, sample := range input.Samples {
		samples = append(samples, sample)
	}
	sort.SliceStable(samples, func(i, j int) bool { return samples[i].Bucket.Before(samples[j].Bucket) })

	baseLoad := input.Budget.CurrentLoadKW
	baseAmbient := input.Budget.AmbientCelsius
	baseCriticalEvents := int64(0)
	baseSensorErrorRate := 0.0
	if len(samples) > 0 {
		last := samples[len(samples)-1]
		if last.LoadKW > 0 {
			baseLoad = last.LoadKW
		}
		if last.AmbientCelsius > 0 {
			baseAmbient = last.AmbientCelsius
		}
		baseCriticalEvents = last.CriticalEvents
		baseSensorErrorRate = last.SensorErrorRate
	}

	loadTrendPerHour := 0.0
	ambientTrendPerHour := 0.0
	if len(samples) >= 2 {
		first := samples[0]
		last := samples[len(samples)-1]
		hours := last.Bucket.Sub(first.Bucket).Hours()
		if hours > 0 {
			loadTrendPerHour = (last.LoadKW - first.LoadKW) / hours
			ambientTrendPerHour = (last.AmbientCelsius - first.AmbientCelsius) / hours
		}
	}

	if loadTrendPerHour > 60 {
		loadTrendPerHour = 60
	}
	if loadTrendPerHour < -60 {
		loadTrendPerHour = -60
	}
	if ambientTrendPerHour > 4 {
		ambientTrendPerHour = 4
	}
	if ambientTrendPerHour < -4 {
		ambientTrendPerHour = -4
	}

	points := make([]SiteRiskProjectionPoint, 0, 4)
	for hourOffset := 1; hourOffset <= 4; hourOffset++ {
		projectedAmbient := math.Max(baseAmbient+(ambientTrendPerHour*float64(hourOffset)), -40)
		projectedLoad := math.Max(baseLoad+(loadTrendPerHour*float64(hourOffset)), 0)
		safeCapacity := siteSafeCapacityForAmbient(input.Site, projectedAmbient)
		marginKW := safeCapacity - projectedLoad

		score := 0.0
		reasons := make([]string, 0, 5)
		loadRatio := 0.0
		if safeCapacity > 0 {
			loadRatio = projectedLoad / safeCapacity
		}
		switch {
		case loadRatio >= 1:
			score += 65
			reasons = append(reasons, "projected_site_overload")
		case loadRatio >= 0.92:
			score += 40
			reasons = append(reasons, "projected_headroom_low")
		case loadRatio >= 0.85:
			score += 20
			reasons = append(reasons, "projected_headroom_tight")
		}
		if projectedAmbient >= input.Site.AmbientDerateStartC+8 {
			score += 20
			reasons = append(reasons, "extreme_heat_derating")
		} else if projectedAmbient >= input.Site.AmbientDerateStartC+3 {
			score += 10
			reasons = append(reasons, "high_ambient_derating")
		}
		if baseCriticalEvents > 0 {
			score += math.Min(float64(baseCriticalEvents)*6, 18)
			reasons = append(reasons, "recent_critical_events")
		}
		if baseSensorErrorRate >= 0.10 {
			score += 15
			reasons = append(reasons, "sensor_quality_degraded")
		}
		if input.CurrentTariff != nil && input.CurrentTariff.IsExpensiveBand {
			score += 12
			reasons = append(reasons, "expensive_tariff_active")
		}

		level := "low"
		switch {
		case score >= 75:
			level = "critical"
		case score >= 50:
			level = "high"
		case score >= 25:
			level = "moderate"
		}

		points = append(points, SiteRiskProjectionPoint{
			At:                      input.At.Add(time.Duration(hourOffset) * time.Hour).UTC(),
			ProjectedLoadKW:         round2(projectedLoad),
			ProjectedSafeCapacityKW: round2(safeCapacity),
			ProjectedMarginKW:       round2(marginKW),
			ProjectedAmbientCelsius: round2(projectedAmbient),
			RiskLevel:               level,
			RiskScore:               round2(score),
			Reasons:                 dedupeAndSort(reasons),
		})
	}

	return points
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func BuildCampusOverview(calculatedAt time.Time, sites []SiteOverview) CampusOverview {
	sort.SliceStable(sites, func(i, j int) bool {
		if sites[i].CurrentTariffExpensive != sites[j].CurrentTariffExpensive {
			return sites[i].CurrentTariffExpensive
		}
		if len(sites[i].TopRisks) != len(sites[j].TopRisks) {
			return len(sites[i].TopRisks) > len(sites[j].TopRisks)
		}
		return strings.Compare(sites[i].SiteID, sites[j].SiteID) < 0
	})

	return CampusOverview{
		CalculatedAt: calculatedAt.UTC(),
		SiteCount:    len(sites),
		Sites:        sites,
	}
}

func SensitiveReviewLevel(review RecommendationReviewRequest) (RecommendationReviewSensitivity, bool) {
	if review.Action == "isolate" {
		return SensitivityHigh, true
	}
	if normalizeLoadCriticalityClass(string(review.CriticalityClass)) == LoadClassPreferredProduction {
		return SensitivityHigh, true
	}
	if math.Abs(review.RecommendedDeltaKW) >= 25 {
		return SensitivityHigh, true
	}
	return SensitivityStandard, false
}

func BuildReviewSummary(review RecommendationReview) []string {
	lines := []string{
		fmt.Sprintf("decision %s for %s on site %s", review.Decision, review.Action, review.SiteID),
	}
	if review.RequiresDualConfirmation && review.Status == ReviewStatusPendingSecondApproval {
		lines = append(lines, "a second approver is still required before execution")
	}
	if review.PostponedUntil != nil {
		lines = append(lines, fmt.Sprintf("postponed until %s", review.PostponedUntil.UTC().Format(time.RFC3339)))
	}
	return lines
}

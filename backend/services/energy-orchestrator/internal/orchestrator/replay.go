package orchestrator

import (
	"math"
	"sort"
	"strings"
	"time"
)

type replayScenario struct {
	Name                  string
	Description           string
	SafeCapacityFactor    float64
	ThermalSoftLimitC     float64
	RampUpKWPerInterval   float64
	RampDownKWPerInterval float64
}

type replayAccumulator struct {
	totalPowerWatts     float64
	totalHashrateTHs    float64
	peakPowerKW         float64
	maxTempCelsius      float64
	totalAmbientCelsius float64
	ambientSamples      float64
	estimatedAlerts     int64
	energyMWh           float64
}

func RunHistoricalReplay(input HistoricalReplayInput) HistoricalReplayResult {
	result := HistoricalReplayResult{
		SiteID:                  input.Site.SiteID,
		PolicyMode:              strings.TrimSpace(input.Site.AdvisoryMode),
		Day:                     input.Day.UTC(),
		RampPolicy:              RampPolicy{IntervalSeconds: input.Site.RampIntervalSeconds, RampUpKWPerInterval: input.Site.RampUpKWPerInterval, RampDownKWPerInterval: input.Site.RampDownKWPerInterval},
		ObservedPersistedAlerts: input.ObservedPersistedAlerts,
	}
	if result.PolicyMode == "" {
		result.PolicyMode = "advisory-first"
	}
	if result.RampPolicy.IntervalSeconds <= 0 {
		result.RampPolicy.IntervalSeconds = 300
	}
	if len(input.Points) == 0 {
		return result
	}

	observed := aggregateObservedScenario(input)
	observed.ObservedAlertRows = input.ObservedPersistedAlerts
	result.Observed = observed

	scenarios := []replayScenario{
		{
			Name:                  "priority_balanced",
			Description:           "curtails blocked and sacrificial load first while preserving preferred production as much as possible",
			SafeCapacityFactor:    1.00,
			ThermalSoftLimitC:     88,
			RampUpKWPerInterval:   maxFloat(input.Site.RampUpKWPerInterval, 0),
			RampDownKWPerInterval: maxFloat(input.Site.RampDownKWPerInterval, 0),
		},
		{
			Name:                  "protective_thermal",
			Description:           "uses a tighter thermal ceiling and more aggressive down-ramp to reduce thermal excursions and alerts",
			SafeCapacityFactor:    0.95,
			ThermalSoftLimitC:     84,
			RampUpKWPerInterval:   maxFloat(input.Site.RampUpKWPerInterval*0.75, 0),
			RampDownKWPerInterval: maxFloat(input.Site.RampDownKWPerInterval*1.50, 0),
		},
	}

	comparisons := make([]ReplayScenarioMetrics, 0, len(scenarios))
	for _, scenario := range scenarios {
		metrics := simulateReplayScenario(input, scenario)
		metrics.DeltaAvgJTHPct = deltaPct(metrics.AvgJTH, observed.AvgJTH)
		metrics.DeltaPeakPowerPct = deltaPct(metrics.PeakPowerKW, observed.PeakPowerKW)
		metrics.DeltaMaxTempPct = deltaPct(metrics.MaxTempCelsius, observed.MaxTempCelsius)
		metrics.DeltaAlertCountPct = deltaPct(float64(metrics.EstimatedAlertCount), float64(observed.EstimatedAlertCount))
		comparisons = append(comparisons, metrics)
	}

	sort.SliceStable(comparisons, func(i, j int) bool {
		return comparisons[i].Name < comparisons[j].Name
	})
	result.Scenarios = comparisons
	return result
}

func aggregateObservedScenario(input HistoricalReplayInput) ReplayScenarioMetrics {
	points := make([]HistoricalRackPoint, 0, len(input.Points))
	points = append(points, input.Points...)
	sort.SliceStable(points, func(i, j int) bool {
		if points[i].Bucket.Equal(points[j].Bucket) {
			return points[i].RackID < points[j].RackID
		}
		return points[i].Bucket.Before(points[j].Bucket)
	})

	acc := replayAccumulator{}
	intervalHours := float64(maxInt(input.Site.RampIntervalSeconds, 300)) / 3600.0
	currentBucket := time.Time{}
	sitePowerKW := 0.0

	for _, point := range points {
		if currentBucket.IsZero() || !point.Bucket.Equal(currentBucket) {
			if !currentBucket.IsZero() {
				acc.peakPowerKW = math.Max(acc.peakPowerKW, sitePowerKW)
			}
			currentBucket = point.Bucket
			sitePowerKW = 0
		}

		sitePowerKW += point.CurrentLoadKW
		acc.totalPowerWatts += point.AvgPowerWatts
		acc.totalHashrateTHs += point.AvgHashrateTHs
		acc.maxTempCelsius = math.Max(acc.maxTempCelsius, point.MaxTempCelsius)
		acc.totalAmbientCelsius += point.AvgAmbientCelsius
		acc.ambientSamples++
		acc.estimatedAlerts += estimateReplayAlerts(point.MinerModel, point.AvgHashrateTHs, point.AvgPowerWatts, point.MaxTempCelsius, point.NominalHashrateTHs, point.NominalPowerWatts)
		acc.energyMWh += (point.AvgPowerWatts * intervalHours) / 1_000_000.0
	}
	acc.peakPowerKW = math.Max(acc.peakPowerKW, sitePowerKW)

	return finalizeReplayMetrics("observed", "observed site operation without replayed policy changes", acc)
}

func simulateReplayScenario(input HistoricalReplayInput, scenario replayScenario) ReplayScenarioMetrics {
	pointsByBucket := make(map[time.Time][]HistoricalRackPoint)
	buckets := make([]time.Time, 0)
	for _, point := range input.Points {
		if _, exists := pointsByBucket[point.Bucket]; !exists {
			buckets = append(buckets, point.Bucket)
		}
		pointsByBucket[point.Bucket] = append(pointsByBucket[point.Bucket], point)
	}
	sort.SliceStable(buckets, func(i, j int) bool { return buckets[i].Before(buckets[j]) })

	prevLoadByRack := make(map[string]float64)
	acc := replayAccumulator{}
	intervalSeconds := maxInt(input.Site.RampIntervalSeconds, 300)
	intervalHours := float64(intervalSeconds) / 3600.0

	for _, bucket := range buckets {
		points := pointsByBucket[bucket]
		sort.SliceStable(points, func(i, j int) bool {
			if curtailmentPriority(points[i].CriticalityClass) == curtailmentPriority(points[j].CriticalityClass) {
				return points[i].RackID < points[j].RackID
			}
			return curtailmentPriority(points[i].CriticalityClass) < curtailmentPriority(points[j].CriticalityClass)
		})

		ambient := averageAmbient(points)
		siteCurrentLoadKW := 0.0
		for _, point := range points {
			siteCurrentLoadKW += point.CurrentLoadKW
		}
		siteSafeCapacityKW := siteSafeCapacityForAmbient(input.Site, ambient) * scenario.SafeCapacityFactor
		remainingOverageKW := maxFloat(siteCurrentLoadKW-siteSafeCapacityKW, 0)
		bucketPowerKW := 0.0

		for _, point := range points {
			currentLoadKW := point.CurrentLoadKW
			desiredLoadKW := currentLoadKW
			if point.SafetyLocked || normalizeLoadCriticalityClass(string(point.CriticalityClass)) == LoadClassSafetyBlocked {
				desiredLoadKW = 0
			} else if remainingOverageKW > 0 {
				reduction := math.Min(remainingOverageKW, desiredLoadKW)
				desiredLoadKW -= reduction
				remainingOverageKW -= reduction
			}

			if scenario.ThermalSoftLimitC > 0 && point.AvgTempCelsius > point.AvgAmbientCelsius && desiredLoadKW > 0 {
				safeRatio := (scenario.ThermalSoftLimitC - point.AvgAmbientCelsius) / (point.AvgTempCelsius - point.AvgAmbientCelsius)
				if safeRatio < 0 {
					safeRatio = 0
				}
				if safeRatio < 1 {
					desiredLoadKW = math.Min(desiredLoadKW, currentLoadKW*safeRatio)
				}
			}

			prevLoadKW := currentLoadKW
			if previous, exists := prevLoadByRack[point.RackID]; exists {
				prevLoadKW = previous
			}

			rampUpLimitKW := scenario.RampUpKWPerInterval
			if point.RampUpKWPerInterval > 0 {
				rampUpLimitKW = point.RampUpKWPerInterval
			}
			rampDownLimitKW := scenario.RampDownKWPerInterval
			if point.RampDownKWPerInterval > 0 {
				rampDownLimitKW = point.RampDownKWPerInterval
			}
			if point.SafetyLocked {
				rampUpLimitKW = 0
				rampDownLimitKW = currentLoadKW
			}

			replayedLoadKW := applyRamp(prevLoadKW, desiredLoadKW, rampUpLimitKW, rampDownLimitKW)
			if replayedLoadKW < 0 {
				replayedLoadKW = 0
			}
			prevLoadByRack[point.RackID] = replayedLoadKW

			loadRatio := 0.0
			if currentLoadKW > 0 {
				loadRatio = replayedLoadKW / currentLoadKW
			}
			replayedPowerWatts := replayedLoadKW * 1000
			replayedHashrateTHs := point.AvgHashrateTHs * loadRatio
			if point.AvgEfficiencyJTH > 0 && replayedPowerWatts > 0 {
				replayedHashrateTHs = replayedPowerWatts / point.AvgEfficiencyJTH
			}
			replayedTempC := point.AvgAmbientCelsius + ((point.AvgTempCelsius - point.AvgAmbientCelsius) * loadRatio)
			replayedMaxTempC := point.AvgAmbientCelsius + ((point.MaxTempCelsius - point.AvgAmbientCelsius) * loadRatio)

			bucketPowerKW += replayedLoadKW
			acc.totalPowerWatts += replayedPowerWatts
			acc.totalHashrateTHs += replayedHashrateTHs
			acc.maxTempCelsius = math.Max(acc.maxTempCelsius, replayedMaxTempC)
			acc.totalAmbientCelsius += point.AvgAmbientCelsius
			acc.ambientSamples++
			acc.estimatedAlerts += estimateReplayAlerts(point.MinerModel, replayedHashrateTHs, replayedPowerWatts, replayedMaxTempC, point.NominalHashrateTHs, point.NominalPowerWatts)
			acc.energyMWh += (replayedPowerWatts * intervalHours) / 1_000_000.0
			if replayedTempC > acc.maxTempCelsius {
				acc.maxTempCelsius = replayedTempC
			}
		}

		acc.peakPowerKW = math.Max(acc.peakPowerKW, bucketPowerKW)
	}

	return finalizeReplayMetrics(scenario.Name, scenario.Description, acc)
}

func finalizeReplayMetrics(name, description string, acc replayAccumulator) ReplayScenarioMetrics {
	avgJTH := 0.0
	if acc.totalPowerWatts > 0 && acc.totalHashrateTHs > 0 {
		avgJTH = acc.totalPowerWatts / acc.totalHashrateTHs
	}
	avgAmbient := 0.0
	if acc.ambientSamples > 0 {
		avgAmbient = acc.totalAmbientCelsius / acc.ambientSamples
	}

	return ReplayScenarioMetrics{
		Name:                name,
		Description:         description,
		AvgJTH:              avgJTH,
		PeakPowerKW:         acc.peakPowerKW,
		MaxTempCelsius:      acc.maxTempCelsius,
		EstimatedAlertCount: acc.estimatedAlerts,
		EnergyMWh:           acc.energyMWh,
		AvgAmbientCelsius:   avgAmbient,
	}
}

func siteSafeCapacityForAmbient(site SiteProfile, ambientCelsius float64) float64 {
	nominalCapacityKW := site.TargetCapacityMW * 1000
	effectiveCapacityKW := nominalCapacityKW * ambientDerateFactor(ambientCelsius, site.AmbientDerateStartC, site.AmbientDeratePctPerDeg)
	reservedCapacityKW := effectiveCapacityKW * (site.OperatingReservePct / 100)
	return maxFloat(effectiveCapacityKW-reservedCapacityKW, 0)
}

func averageAmbient(points []HistoricalRackPoint) float64 {
	total := 0.0
	count := 0.0
	for _, point := range points {
		total += point.AvgAmbientCelsius
		count++
	}
	if count == 0 {
		return 25
	}
	return total / count
}

func applyRamp(previousLoadKW, desiredLoadKW, rampUpKW, rampDownKW float64) float64 {
	if desiredLoadKW > previousLoadKW && rampUpKW > 0 {
		return math.Min(desiredLoadKW, previousLoadKW+rampUpKW)
	}
	if desiredLoadKW < previousLoadKW && rampDownKW > 0 {
		return math.Max(desiredLoadKW, previousLoadKW-rampDownKW)
	}
	if desiredLoadKW > previousLoadKW && rampUpKW <= 0 {
		return previousLoadKW
	}
	return desiredLoadKW
}

func estimateReplayAlerts(model string, hashrateTHs, powerWatts, maxTempCelsius, nominalHashrateTHs, nominalPowerWatts float64) int64 {
	alerts := int64(0)
	if maxTempCelsius >= 90 {
		alerts++
	}
	if nominalPowerWatts > 0 && powerWatts >= nominalPowerWatts*1.20 {
		alerts++
	}
	if nominalHashrateTHs > 0 && hashrateTHs < nominalHashrateTHs*0.70 {
		alerts++
	}
	return alerts
}

func deltaPct(value, reference float64) float64 {
	if reference == 0 {
		return 0
	}
	return ((value - reference) / reference) * 100
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

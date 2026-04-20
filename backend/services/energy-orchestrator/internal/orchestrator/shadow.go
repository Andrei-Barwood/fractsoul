package orchestrator

import (
	"sort"
	"time"
)

type gapCounter struct {
	code        string
	description string
	count       int
}

func RunShadowPilot(input HistoricalReplayInput) ShadowPilotResult {
	replay := RunHistoricalReplay(input)
	result := ShadowPilotResult{
		SiteID:     input.Site.SiteID,
		Day:        input.Day.UTC(),
		PolicyMode: replay.PolicyMode,
		Replay:     replay,
	}

	gaps := map[string]*gapCounter{}

	pointsByBucket := make(map[time.Time][]HistoricalRackPoint)
	buckets := make([]time.Time, 0)
	for _, point := range input.Points {
		if _, ok := pointsByBucket[point.Bucket]; !ok {
			buckets = append(buckets, point.Bucket)
		}
		pointsByBucket[point.Bucket] = append(pointsByBucket[point.Bucket], point)

		if point.AvgAmbientCelsius == 0 {
			incrementGap(gaps, "missing_ambient", "some telemetry points do not contain ambient temperature")
		}
		if point.NominalHashrateTHs <= 0 || point.NominalPowerWatts <= 0 {
			incrementGap(gaps, "missing_nominal_reference", "some telemetry points are missing nominal miner references")
		}
		if point.AvgHashrateTHs <= 0 && point.AvgPowerWatts > 0 {
			incrementGap(gaps, "sensor_hashrate_error", "some telemetry points report power without a valid hashrate reading")
		}
	}
	sort.SliceStable(buckets, func(i, j int) bool { return buckets[i].Before(buckets[j]) })

	for _, bucket := range buckets {
		points := pointsByBucket[bucket]
		siteCurrentLoadKW := 0.0
		siteSafeCapacityKW := siteSafeCapacityForAmbient(input.Site, averageAmbient(points))
		sacrificablePoolKW := 0.0
		for _, point := range points {
			siteCurrentLoadKW += point.CurrentLoadKW
			if normalizeLoadCriticalityClass(string(point.CriticalityClass)) == LoadClassSacrificableLoad {
				sacrificablePoolKW += point.CurrentLoadKW
			}
			if point.SafetyLocked && point.CurrentLoadKW > 0 {
				result.RecommendationsEvaluated++
				result.DecisionsCorrect++
			}
			if point.AvgAmbientCelsius == 0 || point.NominalHashrateTHs <= 0 || point.NominalPowerWatts <= 0 {
				result.DecisionsBlocked++
			}
		}

		if siteCurrentLoadKW > siteSafeCapacityKW {
			reductionGapKW := siteCurrentLoadKW - siteSafeCapacityKW
			if sacrificablePoolKW >= reductionGapKW {
				result.RecommendationsEvaluated++
				result.DecisionsCorrect++
			} else {
				result.RecommendationsEvaluated++
				result.DecisionsWouldEscalate++
			}
		}
	}

	if len(replay.Scenarios) > 0 {
		best := replay.Scenarios[0]
		for _, scenario := range replay.Scenarios[1:] {
			if scenario.EstimatedAlertCount < best.EstimatedAlertCount {
				best = scenario
				continue
			}
			if scenario.EstimatedAlertCount == best.EstimatedAlertCount && scenario.PeakPowerKW < best.PeakPowerKW {
				best = scenario
			}
		}
		if best.EstimatedAlertCount < replay.Observed.EstimatedAlertCount {
			result.DecisionsCorrect += 1
		}
	}

	missingData := make([]ShadowPilotDataGap, 0, len(gaps))
	for _, gap := range gaps {
		missingData = append(missingData, ShadowPilotDataGap{
			Code:        gap.code,
			Description: gap.description,
			Count:       gap.count,
		})
		result.MissingDataCount += gap.count
	}
	sort.SliceStable(missingData, func(i, j int) bool {
		if missingData[i].Count == missingData[j].Count {
			return missingData[i].Code < missingData[j].Code
		}
		return missingData[i].Count > missingData[j].Count
	})
	result.MissingData = missingData

	summary := make([]string, 0, 4)
	summary = append(summary, "shadow mode compares observed operation against advisory alternatives without executing changes")
	if replay.Observed.EstimatedAlertCount > 0 && len(replay.Scenarios) > 0 {
		summary = append(summary, "at least one advisory scenario improves alerts or peak load versus observed operation")
	}
	if result.DecisionsWouldEscalate > 0 {
		summary = append(summary, "some overload intervals still require escalation because sacrificial capacity is insufficient")
	}
	if result.MissingDataCount > 0 {
		summary = append(summary, "data quality gaps still limit how many recommendations can be evaluated confidently")
	}
	result.Summary = summary

	return result
}

func incrementGap(gaps map[string]*gapCounter, code, description string) {
	if gaps[code] == nil {
		gaps[code] = &gapCounter{
			code:        code,
			description: description,
		}
	}
	gaps[code].count++
}

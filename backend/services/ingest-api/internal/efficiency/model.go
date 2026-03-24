package efficiency

import (
	"math"
	"strconv"
	"strings"
)

type Baseline struct {
	Model                string
	NominalJTH           float64
	OptimalTempMinC      float64
	OptimalTempMaxC      float64
	ElevatedTempMaxC     float64
	HotspotTempC         float64
	AmbientReferenceC    float64
	AmbientPenaltyPerDeg float64
}

var defaultBaseline = Baseline{
	Model:                "UNKNOWN",
	NominalJTH:           26.5,
	OptimalTempMinC:      55,
	OptimalTempMaxC:      80,
	ElevatedTempMaxC:     90,
	HotspotTempC:         95,
	AmbientReferenceC:    25,
	AmbientPenaltyPerDeg: 0.004,
}

var baselines = map[string]Baseline{
	"S21": {
		Model:                "S21",
		NominalJTH:           17.8,
		OptimalTempMinC:      55,
		OptimalTempMaxC:      78,
		ElevatedTempMaxC:     88,
		HotspotTempC:         95,
		AmbientReferenceC:    25,
		AmbientPenaltyPerDeg: 0.004,
	},
	"S19XP": {
		Model:                "S19XP",
		NominalJTH:           21.3,
		OptimalTempMinC:      55,
		OptimalTempMaxC:      76,
		ElevatedTempMaxC:     86,
		HotspotTempC:         93,
		AmbientReferenceC:    25,
		AmbientPenaltyPerDeg: 0.0045,
	},
	"M50": {
		Model:                "M50",
		NominalJTH:           28.0,
		OptimalTempMinC:      54,
		OptimalTempMaxC:      75,
		ElevatedTempMaxC:     85,
		HotspotTempC:         92,
		AmbientReferenceC:    25,
		AmbientPenaltyPerDeg: 0.005,
	},
}

func NormalizeModel(model string) string {
	value := strings.ToUpper(strings.TrimSpace(model))
	if value == "" {
		return "UNKNOWN"
	}
	return value
}

func BaselineForModel(model string) Baseline {
	normalized := NormalizeModel(model)
	if baseline, ok := baselines[normalized]; ok {
		return baseline
	}
	return defaultBaseline
}

func ComputeJTH(powerWatts, hashrateTHs float64) float64 {
	if powerWatts <= 0 || hashrateTHs <= 0 {
		return 0
	}
	return powerWatts / hashrateTHs
}

func CompensateJTH(rawJTH, ambientC float64, baseline Baseline) float64 {
	if rawJTH <= 0 {
		return 0
	}

	deltaAmbient := ambientC - baseline.AmbientReferenceC
	factor := 1 + (deltaAmbient * baseline.AmbientPenaltyPerDeg)
	if factor < 0.5 {
		factor = 0.5
	}
	if factor > 1.8 {
		factor = 1.8
	}

	return rawJTH / factor
}

func ClassifyThermalBand(tempC float64, baseline Baseline) string {
	switch {
	case tempC < baseline.OptimalTempMinC:
		return "cold"
	case tempC <= baseline.OptimalTempMaxC:
		return "optimal"
	case tempC <= baseline.ElevatedTempMaxC:
		return "elevated"
	case tempC <= baseline.HotspotTempC:
		return "warning"
	default:
		return "hotspot"
	}
}

func DeltaPct(value, reference float64) float64 {
	if value <= 0 || reference <= 0 {
		return 0
	}
	return ((value - reference) / reference) * 100
}

func ParseAmbient(tags map[string]string, fallback float64) float64 {
	if len(tags) == 0 {
		return fallback
	}

	keys := []string{"ambient_temp_c", "ambient_c"}
	for _, key := range keys {
		raw := strings.TrimSpace(tags[key])
		if raw == "" {
			continue
		}
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			continue
		}
		return value
	}

	return fallback
}

func Clamp(value, min, max float64) float64 {
	return math.Min(math.Max(value, min), max)
}

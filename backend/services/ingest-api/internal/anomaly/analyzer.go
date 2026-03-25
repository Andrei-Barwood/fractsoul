package anomaly

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/efficiency"
	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/storage"
)

type FeatureVector struct {
	Samples                 int     `json:"samples"`
	AvgHashrateTHs          float64 `json:"avg_hashrate_ths"`
	AvgPowerWatts           float64 `json:"avg_power_watts"`
	AvgTempCelsius          float64 `json:"avg_temp_celsius"`
	MaxTempCelsius          float64 `json:"max_temp_celsius"`
	AvgFanRPM               float64 `json:"avg_fan_rpm"`
	AvgEfficiencyJTH        float64 `json:"avg_efficiency_jth"`
	HashrateDropPct         float64 `json:"hashrate_drop_pct"`
	HashrateTrendTHsPerHour float64 `json:"hashrate_trend_ths_per_hour"`
	TempTrendCPerHour       float64 `json:"temp_trend_c_per_hour"`
	PowerTrendWPerHour      float64 `json:"power_trend_w_per_hour"`
	AmbientCelsius          float64 `json:"ambient_celsius"`
	CompensatedJTH          float64 `json:"compensated_jth"`
	ThermalBand             string  `json:"thermal_band"`
}

type Detection struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	Triggered bool               `json:"triggered"`
	Score     float64            `json:"score"`
	Evidence  map[string]float64 `json:"evidence,omitempty"`
}

type Recommendation struct {
	Parameter      string `json:"parameter"`
	SuggestedDelta string `json:"suggested_delta"`
	RequestedDelta string `json:"requested_delta,omitempty"`
	Reason         string `json:"reason"`
}

type GuardrailDecision struct {
	Parameter      string `json:"parameter"`
	RequestedDelta string `json:"requested_delta"`
	AppliedDelta   string `json:"applied_delta"`
	Action         string `json:"action"`
	Reason         string `json:"reason"`
}

type ImpactSnapshot struct {
	HashrateTHs    float64 `json:"hashrate_ths"`
	PowerWatts     float64 `json:"power_watts"`
	TempCelsius    float64 `json:"temp_celsius"`
	FanRPM         float64 `json:"fan_rpm"`
	CompensatedJTH float64 `json:"compensated_jth"`
}

type ImpactDelta struct {
	HashrateTHsPct    float64 `json:"hashrate_ths_pct"`
	PowerWattsPct     float64 `json:"power_watts_pct"`
	TempCelsiusDelta  float64 `json:"temp_celsius_delta"`
	FanRPMPct         float64 `json:"fan_rpm_pct"`
	CompensatedJTHPct float64 `json:"compensated_jth_pct"`
}

type ImpactEstimate struct {
	Before      ImpactSnapshot `json:"before"`
	After       ImpactSnapshot `json:"after"`
	Delta       ImpactDelta    `json:"delta"`
	Confidence  float64        `json:"confidence"`
	Assumptions []string       `json:"assumptions"`
}

type Report struct {
	MinerID         string              `json:"miner_id"`
	SiteID          string              `json:"site_id"`
	RackID          string              `json:"rack_id"`
	MinerModel      string              `json:"miner_model"`
	WindowFrom      time.Time           `json:"window_from"`
	WindowTo        time.Time           `json:"window_to"`
	Features        FeatureVector       `json:"features"`
	Hotspot         Detection           `json:"hotspot"`
	HashDegradation Detection           `json:"hash_degradation"`
	SeverityScore   float64             `json:"severity_score"`
	SeverityLabel   string              `json:"severity_label"`
	ProbableCause   string              `json:"probable_cause"`
	Recommendations []Recommendation    `json:"recommendations"`
	Guardrails      []GuardrailDecision `json:"guardrails"`
	ImpactEstimate  ImpactEstimate      `json:"impact_estimate"`
}

func Analyze(
	latest storage.TelemetryReading,
	series []storage.MinerSeriesPoint,
	ambientOverride *float64,
) Report {
	baseline := efficiency.BaselineForModel(latest.MinerModel)

	ambient := efficiency.ParseAmbient(latest.Tags, baseline.AmbientReferenceC)
	if ambientOverride != nil {
		ambient = *ambientOverride
	}

	features := buildFeatures(series, ambient, baseline)
	hotspot := detectHotspot(features, baseline)
	hashDrop := detectHashDegradation(features, baseline)

	severityScore := severityFromDetections(features, hotspot, hashDrop)
	severityLabel := labelFromScore(severityScore)
	probableCause := probableCauseFromDetections(features, hotspot, hashDrop)
	recommendations, guardrails := applyGuardrails(features, baseline, hotspot, recommendActions(features, hotspot, hashDrop))
	impactEstimate := estimateImpact(features, baseline, recommendations)

	return Report{
		MinerID:         latest.MinerID,
		SiteID:          latest.SiteID,
		RackID:          latest.RackID,
		MinerModel:      latest.MinerModel,
		WindowFrom:      firstBucket(series),
		WindowTo:        lastBucket(series),
		Features:        features,
		Hotspot:         hotspot,
		HashDegradation: hashDrop,
		SeverityScore:   severityScore,
		SeverityLabel:   severityLabel,
		ProbableCause:   probableCause,
		Recommendations: recommendations,
		Guardrails:      guardrails,
		ImpactEstimate:  impactEstimate,
	}
}

func buildFeatures(series []storage.MinerSeriesPoint, ambient float64, baseline efficiency.Baseline) FeatureVector {
	if len(series) == 0 {
		return FeatureVector{
			AmbientCelsius: ambient,
		}
	}

	hashValues := make([]float64, 0, len(series))
	tempValues := make([]float64, 0, len(series))
	powerValues := make([]float64, 0, len(series))
	fanValues := make([]float64, 0, len(series))
	effValues := make([]float64, 0, len(series))
	maxTemp := 0.0

	for index, point := range series {
		hashValues = append(hashValues, point.AvgHashrateTHs)
		tempValues = append(tempValues, point.AvgTempCelsius)
		powerValues = append(powerValues, point.AvgPowerWatts)
		fanValues = append(fanValues, point.AvgFanRPM)
		effValues = append(effValues, point.AvgEfficiencyJTH)
		if index == 0 || point.MaxTempCelsius > maxTemp {
			maxTemp = point.MaxTempCelsius
		}
	}

	avgHash := mean(hashValues)
	avgPower := mean(powerValues)
	avgTemp := mean(tempValues)
	avgFan := mean(fanValues)
	avgEff := mean(effValues)
	rawJTH := efficiency.ComputeJTH(avgPower, avgHash)
	compensated := efficiency.CompensateJTH(rawJTH, ambient, baseline)

	return FeatureVector{
		Samples:                 len(series),
		AvgHashrateTHs:          avgHash,
		AvgPowerWatts:           avgPower,
		AvgTempCelsius:          avgTemp,
		MaxTempCelsius:          maxTemp,
		AvgFanRPM:               avgFan,
		AvgEfficiencyJTH:        avgEff,
		HashrateDropPct:         computeDropPct(hashValues),
		HashrateTrendTHsPerHour: linearTrendPerHour(series, hashValues),
		TempTrendCPerHour:       linearTrendPerHour(series, tempValues),
		PowerTrendWPerHour:      linearTrendPerHour(series, powerValues),
		AmbientCelsius:          ambient,
		CompensatedJTH:          compensated,
		ThermalBand:             efficiency.ClassifyThermalBand(avgTemp, baseline),
	}
}

func detectHotspot(features FeatureVector, baseline efficiency.Baseline) Detection {
	score := 0.0
	overflow := features.MaxTempCelsius - baseline.HotspotTempC
	if overflow > 0 {
		score += overflow * 10
	}
	if features.TempTrendCPerHour > 1.0 {
		score += features.TempTrendCPerHour * 8
	}
	if features.AvgFanRPM > 6800 {
		score += 8
	}
	score = efficiency.Clamp(score, 0, 100)

	triggered := features.MaxTempCelsius >= baseline.HotspotTempC ||
		(features.AvgTempCelsius >= baseline.ElevatedTempMaxC && features.TempTrendCPerHour > 1)

	return Detection{
		ID:        "hotspot_thermal",
		Name:      "Hotspot termico",
		Triggered: triggered,
		Score:     score,
		Evidence: map[string]float64{
			"max_temp_celsius":        features.MaxTempCelsius,
			"hotspot_threshold_c":     baseline.HotspotTempC,
			"temp_trend_c_per_hour":   features.TempTrendCPerHour,
			"avg_fan_rpm":             features.AvgFanRPM,
			"elevated_threshold_c":    baseline.ElevatedTempMaxC,
			"ambient_compensated_jth": features.CompensatedJTH,
		},
	}
}

func detectHashDegradation(features FeatureVector, baseline efficiency.Baseline) Detection {
	score := 0.0
	if features.HashrateDropPct > 0 {
		score += features.HashrateDropPct * 150
	}
	if features.HashrateTrendTHsPerHour < 0 {
		score += math.Abs(features.HashrateTrendTHsPerHour) * 0.45
	}
	if features.CompensatedJTH > baseline.NominalJTH {
		score += (features.CompensatedJTH - baseline.NominalJTH) * 2.2
	}
	score = efficiency.Clamp(score, 0, 100)

	triggered := features.HashrateDropPct >= 0.15 || features.HashrateTrendTHsPerHour <= -8

	return Detection{
		ID:        "hash_degradation_progressive",
		Name:      "Degradacion progresiva de hash",
		Triggered: triggered,
		Score:     score,
		Evidence: map[string]float64{
			"hashrate_drop_pct":       features.HashrateDropPct,
			"hashrate_trend_ths_hour": features.HashrateTrendTHsPerHour,
			"compensated_jth":         features.CompensatedJTH,
			"nominal_jth":             baseline.NominalJTH,
		},
	}
}

func severityFromDetections(features FeatureVector, hotspot, hashDrop Detection) float64 {
	score := 0.0
	if hotspot.Triggered {
		score = math.Max(score, hotspot.Score)
	}
	if hashDrop.Triggered {
		score = math.Max(score, hashDrop.Score)
	}
	if hotspot.Triggered && hashDrop.Triggered {
		score += 10
	}
	if features.ThermalBand == "hotspot" {
		score += 6
	}
	return efficiency.Clamp(score, 0, 100)
}

func labelFromScore(score float64) string {
	switch {
	case score >= 85:
		return "critical"
	case score >= 65:
		return "high"
	case score >= 35:
		return "medium"
	default:
		return "low"
	}
}

func probableCauseFromDetections(features FeatureVector, hotspot, hashDrop Detection) string {
	switch {
	case hotspot.Triggered && hashDrop.Triggered:
		return "Probable throttling termico sostenido: temperatura alta y caida de hashrate en paralelo."
	case hotspot.Triggered:
		return "Posible restriccion de flujo de aire o ensuciamiento: temperatura maxima en rango hotspot."
	case hashDrop.Triggered:
		return "Posible degradacion de chips o inestabilidad de voltaje/frecuencia: hashrate en tendencia descendente."
	default:
		if strings.EqualFold(features.ThermalBand, "optimal") {
			return "Operacion dentro de parametros normales."
		}
		return "Sin anomalia severa; monitorear tendencia termica."
	}
}

func recommendActions(features FeatureVector, hotspot, hashDrop Detection) []Recommendation {
	switch {
	case hotspot.Triggered:
		return []Recommendation{
			{
				Parameter:      "fan",
				SuggestedDelta: "+12%",
				Reason:         "Aumentar evacuacion termica y reducir tiempo en banda hotspot.",
			},
			{
				Parameter:      "freq",
				SuggestedDelta: "-6%",
				Reason:         "Disminuir carga termica para estabilizar la maquina.",
			},
			{
				Parameter:      "volt",
				SuggestedDelta: "-15mV",
				Reason:         "Reducir disipacion sin sacrificar estabilidad base.",
			},
		}
	case hashDrop.Triggered:
		return []Recommendation{
			{
				Parameter:      "fan",
				SuggestedDelta: "+5%",
				Reason:         "Mejorar margen termico mientras se monitorea degradacion.",
			},
			{
				Parameter:      "freq",
				SuggestedDelta: "-3%",
				Reason:         "Evitar errores de hashing por inestabilidad progresiva.",
			},
			{
				Parameter:      "volt",
				SuggestedDelta: "+10mV",
				Reason:         "Compensar margen electrico ante degradacion de hash.",
			},
		}
	default:
		return []Recommendation{
			{
				Parameter:      "fan",
				SuggestedDelta: "0%",
				Reason:         "Mantener setpoint actual; no se detecta riesgo alto.",
			},
			{
				Parameter:      "freq",
				SuggestedDelta: "0%",
				Reason:         "Operacion estable dentro de banda esperada.",
			},
			{
				Parameter:      "volt",
				SuggestedDelta: "0mV",
				Reason:         "Sin ajustes requeridos en esta ventana.",
			},
		}
	}
}

func applyGuardrails(
	features FeatureVector,
	baseline efficiency.Baseline,
	hotspot Detection,
	recommendations []Recommendation,
) ([]Recommendation, []GuardrailDecision) {
	safeRecommendations := make([]Recommendation, 0, len(recommendations))
	decisions := make([]GuardrailDecision, 0, len(recommendations))

	for _, rec := range recommendations {
		requestedRaw := strings.TrimSpace(rec.SuggestedDelta)
		param := strings.ToLower(strings.TrimSpace(rec.Parameter))

		parsed, err := parseDelta(requestedRaw)
		if err != nil {
			applied := zeroDeltaForParameter(param)
			rec.RequestedDelta = requestedRaw
			rec.SuggestedDelta = applied
			rec.Reason = rec.Reason + " Guardrail: ajuste no parseable, se neutraliza."

			safeRecommendations = append(safeRecommendations, rec)
			decisions = append(decisions, GuardrailDecision{
				Parameter:      rec.Parameter,
				RequestedDelta: requestedRaw,
				AppliedDelta:   applied,
				Action:         "blocked",
				Reason:         "delta no interpretable por el motor de guardrails",
			})
			continue
		}

		applied := parsed.Value
		action := "allow"
		reason := "ajuste dentro de limites operativos."

		switch param {
		case "fan":
			if parsed.Unit != "%" {
				applied = 0
				action = "blocked"
				reason = "fan debe expresarse como porcentaje (%)."
				break
			}
			clamped := efficiency.Clamp(applied, -15, 20)
			if clamped != applied {
				applied = clamped
				action = "clamped"
				reason = "fan limitado al rango seguro [-15%, +20%]."
			}
			if hotspot.Triggered && applied < 0 {
				applied = 0
				action = "blocked"
				reason = "hotspot activo: no se permite reducir ventilacion."
			}
		case "freq":
			if parsed.Unit != "%" {
				applied = 0
				action = "blocked"
				reason = "freq debe expresarse como porcentaje (%)."
				break
			}
			clamped := efficiency.Clamp(applied, -8, 4)
			if clamped != applied {
				applied = clamped
				action = "clamped"
				reason = "freq limitada al rango seguro [-8%, +4%]."
			}
			if (hotspot.Triggered || features.ThermalBand == "warning" || features.ThermalBand == "hotspot") && applied > 0 {
				applied = 0
				action = "blocked"
				reason = "banda termica no segura para subir frecuencia."
			}
		case "volt":
			if parsed.Unit != "mV" {
				applied = 0
				action = "blocked"
				reason = "volt debe expresarse como milivoltios (mV)."
				break
			}
			clamped := efficiency.Clamp(applied, -20, 12)
			if clamped != applied {
				applied = clamped
				action = "clamped"
				reason = "volt limitado al rango seguro [-20mV, +12mV]."
			}
			if (hotspot.Triggered || features.ThermalBand == "warning" || features.ThermalBand == "hotspot") && applied > 0 {
				applied = 0
				action = "blocked"
				reason = "riesgo termico elevado: se bloquea aumento de voltaje."
			}
			if features.CompensatedJTH > baseline.NominalJTH*1.15 && applied > 0 {
				applied = 0
				action = "blocked"
				reason = "eficiencia degradada: no se permite subir voltaje."
			}
		default:
			applied = 0
			action = "blocked"
			reason = "parametro no soportado por guardrails."
		}

		appliedDelta := formatDelta(applied, parsed.Unit)
		rec.RequestedDelta = requestedRaw
		rec.SuggestedDelta = appliedDelta
		if action == "allow" && requestedRaw == appliedDelta {
			rec.RequestedDelta = ""
		}

		safeRecommendations = append(safeRecommendations, rec)
		decisions = append(decisions, GuardrailDecision{
			Parameter:      rec.Parameter,
			RequestedDelta: requestedRaw,
			AppliedDelta:   appliedDelta,
			Action:         action,
			Reason:         reason,
		})
	}

	return safeRecommendations, decisions
}

func estimateImpact(
	features FeatureVector,
	baseline efficiency.Baseline,
	recommendations []Recommendation,
) ImpactEstimate {
	before := ImpactSnapshot{
		HashrateTHs:    features.AvgHashrateTHs,
		PowerWatts:     features.AvgPowerWatts,
		TempCelsius:    features.AvgTempCelsius,
		FanRPM:         features.AvgFanRPM,
		CompensatedJTH: features.CompensatedJTH,
	}

	after := before
	for _, rec := range recommendations {
		d, err := parseDelta(rec.SuggestedDelta)
		if err != nil {
			continue
		}

		switch strings.ToLower(strings.TrimSpace(rec.Parameter)) {
		case "fan":
			after.FanRPM *= 1 + (d.Value / 100)
			after.TempCelsius += -0.14 * d.Value
			after.PowerWatts *= 1 + (0.02 * d.Value / 100)
		case "freq":
			after.HashrateTHs *= 1 + (0.82 * d.Value / 100)
			after.PowerWatts *= 1 + (0.68 * d.Value / 100)
			after.TempCelsius += 0.09 * d.Value
		case "volt":
			ratio := d.Value / 700.0
			after.PowerWatts *= 1 + (0.70 * ratio)
			after.HashrateTHs *= 1 + (0.18 * ratio)
			after.TempCelsius += 10 * ratio
		}
	}

	after.HashrateTHs = math.Max(after.HashrateTHs, 1)
	after.PowerWatts = math.Max(after.PowerWatts, 1)
	after.TempCelsius = math.Max(after.TempCelsius, -20)
	after.FanRPM = math.Max(after.FanRPM, 0)
	after.CompensatedJTH = efficiency.CompensateJTH(
		efficiency.ComputeJTH(after.PowerWatts, after.HashrateTHs),
		features.AmbientCelsius,
		baseline,
	)

	confidence := efficiency.Clamp(0.45+float64(features.Samples)*0.03, 0.45, 0.92)
	if features.ThermalBand == "hotspot" {
		confidence = efficiency.Clamp(confidence-0.08, 0.40, 0.92)
	}

	return ImpactEstimate{
		Before: before,
		After:  after,
		Delta: ImpactDelta{
			HashrateTHsPct:    efficiency.DeltaPct(after.HashrateTHs, before.HashrateTHs),
			PowerWattsPct:     efficiency.DeltaPct(after.PowerWatts, before.PowerWatts),
			TempCelsiusDelta:  after.TempCelsius - before.TempCelsius,
			FanRPMPct:         efficiency.DeltaPct(after.FanRPM, before.FanRPM),
			CompensatedJTHPct: efficiency.DeltaPct(after.CompensatedJTH, before.CompensatedJTH),
		},
		Confidence: math.Round(confidence*100) / 100,
		Assumptions: []string{
			"Estimacion heuristica basada en elasticidades historicas.",
			"No incorpora cambios de firmware ni mantenimiento fisico.",
			"Requiere validacion post-cambio en ventana de 30-60 minutos.",
		},
	}
}

type parsedDelta struct {
	Value float64
	Unit  string
}

func parseDelta(raw string) (parsedDelta, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return parsedDelta{}, fmt.Errorf("empty delta")
	}

	lower := strings.ToLower(value)
	switch {
	case strings.HasSuffix(lower, "%"):
		number := strings.TrimSpace(value[:len(value)-1])
		parsed, err := strconv.ParseFloat(number, 64)
		if err != nil {
			return parsedDelta{}, fmt.Errorf("invalid percent delta")
		}
		return parsedDelta{Value: parsed, Unit: "%"}, nil
	case strings.HasSuffix(lower, "mv"):
		number := strings.TrimSpace(value[:len(value)-2])
		parsed, err := strconv.ParseFloat(number, 64)
		if err != nil {
			return parsedDelta{}, fmt.Errorf("invalid mv delta")
		}
		return parsedDelta{Value: parsed, Unit: "mV"}, nil
	default:
		return parsedDelta{}, fmt.Errorf("unsupported delta unit")
	}
}

func zeroDeltaForParameter(parameter string) string {
	switch parameter {
	case "volt":
		return "0mV"
	default:
		return "0%"
	}
}

func formatDelta(value float64, unit string) string {
	if math.Abs(value) < 1e-9 {
		return "0" + unit
	}

	formatted := fmt.Sprintf("%+.2f", value)
	formatted = strings.TrimSuffix(formatted, "00")
	formatted = strings.TrimSuffix(formatted, "0")
	formatted = strings.TrimSuffix(formatted, ".")
	return formatted + unit
}

func firstBucket(points []storage.MinerSeriesPoint) time.Time {
	if len(points) == 0 {
		return time.Time{}
	}
	return points[0].Bucket
}

func lastBucket(points []storage.MinerSeriesPoint) time.Time {
	if len(points) == 0 {
		return time.Time{}
	}
	return points[len(points)-1].Bucket
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

func computeDropPct(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	segment := len(values) / 3
	if segment < 1 {
		segment = 1
	}
	firstAvg := mean(values[:segment])
	lastAvg := mean(values[len(values)-segment:])
	if firstAvg <= 0 {
		return 0
	}
	return (firstAvg - lastAvg) / firstAvg
}

func linearTrendPerHour(points []storage.MinerSeriesPoint, values []float64) float64 {
	if len(points) != len(values) || len(values) < 2 {
		return 0
	}

	x0 := points[0].Bucket
	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumXX := 0.0
	n := float64(len(points))

	for idx, point := range points {
		x := point.Bucket.Sub(x0).Hours()
		y := values[idx]
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}

	denominator := (n * sumXX) - (sumX * sumX)
	if denominator == 0 {
		return 0
	}

	return ((n * sumXY) - (sumX * sumY)) / denominator
}

func SummaryLine(report Report) string {
	return fmt.Sprintf(
		"miner=%s model=%s severity=%s score=%.1f hotspot=%t hash_degradation=%t",
		report.MinerID,
		report.MinerModel,
		report.SeverityLabel,
		report.SeverityScore,
		report.Hotspot.Triggered,
		report.HashDegradation.Triggered,
	)
}

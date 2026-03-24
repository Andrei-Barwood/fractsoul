package alerts

import (
	"fmt"
	"strings"
	"time"

	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/telemetry"
)

type RuleContext struct {
	Model              string
	NominalHashrateTHs float64
	NominalPowerWatts  float64
}

type Rule interface {
	ID() string
	Name() string
	Evaluate(event telemetry.IngestRequest, ctx RuleContext) (EvaluatedAlert, bool)
}

type Engine struct {
	rules []Rule
}

func NewEngine(rules []Rule) *Engine {
	cloned := make([]Rule, 0, len(rules))
	for _, rule := range rules {
		if rule == nil {
			continue
		}
		cloned = append(cloned, rule)
	}
	return &Engine{rules: cloned}
}

func DefaultRules() []Rule {
	return []Rule{
		overheatRule{warningTemp: 90, criticalTemp: 95},
		powerSpikeRule{warningMultiplier: 1.2, criticalMultiplier: 1.35},
		hashrateDropRule{warningRatio: 0.7, criticalRatio: 0.5},
	}
}

func (e *Engine) Evaluate(event telemetry.IngestRequest) []EvaluatedAlert {
	if e == nil {
		return nil
	}

	ctx := buildRuleContext(event)
	alerts := make([]EvaluatedAlert, 0, len(e.rules))
	for _, rule := range e.rules {
		candidate, ok := rule.Evaluate(event, ctx)
		if !ok {
			continue
		}
		candidate.SiteID = event.SiteID
		candidate.RackID = event.RackID
		candidate.MinerID = event.MinerID
		candidate.EventID = event.EventID
		candidate.ObservedAt = normalizeObservedAt(candidate.ObservedAt, event.Timestamp)
		candidate.MinerModel = ctx.Model
		candidate.Firmware = event.FirmwareVersion
		if candidate.Details == nil {
			candidate.Details = map[string]any{}
		}
		candidate.Details["model"] = ctx.Model
		alerts = append(alerts, candidate)
	}

	return alerts
}

func normalizeObservedAt(candidate time.Time, fallback time.Time) time.Time {
	if !candidate.IsZero() {
		return candidate.UTC()
	}
	if !fallback.IsZero() {
		return fallback.UTC()
	}
	return time.Now().UTC()
}

type modelProfile struct {
	hashrateTHs float64
	powerWatts  float64
}

var defaultModelProfile = modelProfile{hashrateTHs: 120, powerWatts: 3200}

var knownProfiles = map[string]modelProfile{
	"S19XP": {hashrateTHs: 141, powerWatts: 3010},
	"S21":   {hashrateTHs: 200, powerWatts: 3550},
	"M50":   {hashrateTHs: 120, powerWatts: 3300},
}

func buildRuleContext(event telemetry.IngestRequest) RuleContext {
	model := extractModel(event.Tags)
	profile, ok := knownProfiles[model]
	if !ok {
		profile = defaultModelProfile
	}

	return RuleContext{
		Model:              model,
		NominalHashrateTHs: profile.hashrateTHs,
		NominalPowerWatts:  profile.powerWatts,
	}
}

func extractModel(tags map[string]string) string {
	if len(tags) == 0 {
		return "UNKNOWN"
	}
	candidates := []string{
		tags["asic_model"],
		tags["miner_model"],
		tags["model"],
	}
	for _, candidate := range candidates {
		value := strings.ToUpper(strings.TrimSpace(candidate))
		if value != "" {
			return value
		}
	}
	return "UNKNOWN"
}

type overheatRule struct {
	warningTemp  float64
	criticalTemp float64
}

func (r overheatRule) ID() string { return "overheat" }

func (r overheatRule) Name() string { return "Sobretemperatura" }

func (r overheatRule) Evaluate(event telemetry.IngestRequest, _ RuleContext) (EvaluatedAlert, bool) {
	temp := event.Metrics.TempCelsius
	if temp < r.warningTemp {
		return EvaluatedAlert{}, false
	}

	severity := SeverityWarning
	if temp >= r.criticalTemp || event.Metrics.Status == telemetry.StatusCritical {
		severity = SeverityCritical
	}

	threshold := r.warningTemp
	if severity == SeverityCritical {
		threshold = r.criticalTemp
	}

	return EvaluatedAlert{
		RuleID:      r.ID(),
		RuleName:    r.Name(),
		Severity:    severity,
		Message:     fmt.Sprintf("Temperatura alta detectada: %.2f°C", temp),
		MetricName:  "temp_celsius",
		MetricValue: temp,
		Threshold:   threshold,
		Details: map[string]any{
			"status": event.Metrics.Status,
		},
	}, true
}

type powerSpikeRule struct {
	warningMultiplier  float64
	criticalMultiplier float64
}

func (r powerSpikeRule) ID() string { return "power_spike" }

func (r powerSpikeRule) Name() string { return "Pico de Consumo" }

func (r powerSpikeRule) Evaluate(event telemetry.IngestRequest, ctx RuleContext) (EvaluatedAlert, bool) {
	base := ctx.NominalPowerWatts
	if base <= 0 {
		base = defaultModelProfile.powerWatts
	}

	power := event.Metrics.PowerWatts
	warningThreshold := base * r.warningMultiplier
	if power < warningThreshold {
		return EvaluatedAlert{}, false
	}

	criticalThreshold := base * r.criticalMultiplier
	severity := SeverityWarning
	threshold := warningThreshold
	if power >= criticalThreshold {
		severity = SeverityCritical
		threshold = criticalThreshold
	}

	return EvaluatedAlert{
		RuleID:      r.ID(),
		RuleName:    r.Name(),
		Severity:    severity,
		Message:     fmt.Sprintf("Consumo fuera de banda: %.2fW", power),
		MetricName:  "power_watts",
		MetricValue: power,
		Threshold:   threshold,
		Details: map[string]any{
			"nominal_power_watts": base,
		},
	}, true
}

type hashrateDropRule struct {
	warningRatio  float64
	criticalRatio float64
}

func (r hashrateDropRule) ID() string { return "hashrate_drop" }

func (r hashrateDropRule) Name() string { return "Caida de Hashrate" }

func (r hashrateDropRule) Evaluate(event telemetry.IngestRequest, ctx RuleContext) (EvaluatedAlert, bool) {
	base := ctx.NominalHashrateTHs
	if base <= 0 {
		base = defaultModelProfile.hashrateTHs
	}

	hashrate := event.Metrics.HashrateTHs
	warningThreshold := base * r.warningRatio
	if hashrate >= warningThreshold {
		return EvaluatedAlert{}, false
	}

	criticalThreshold := base * r.criticalRatio
	severity := SeverityWarning
	threshold := warningThreshold
	if hashrate <= criticalThreshold || event.Metrics.Status == telemetry.StatusCritical {
		severity = SeverityCritical
		threshold = criticalThreshold
	}

	return EvaluatedAlert{
		RuleID:      r.ID(),
		RuleName:    r.Name(),
		Severity:    severity,
		Message:     fmt.Sprintf("Hashrate por debajo de esperado: %.2f TH/s", hashrate),
		MetricName:  "hashrate_ths",
		MetricValue: hashrate,
		Threshold:   threshold,
		Details: map[string]any{
			"nominal_hashrate_ths": base,
		},
	}, true
}

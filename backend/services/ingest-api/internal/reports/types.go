package reports

import (
	"fmt"
	"time"
)

type GlobalMetrics struct {
	Samples          int64   `json:"samples"`
	ActiveMiners     int64   `json:"active_miners"`
	AvgHashrateTHs   float64 `json:"avg_hashrate_ths"`
	AvgPowerWatts    float64 `json:"avg_power_watts"`
	AvgTempCelsius   float64 `json:"avg_temp_celsius"`
	AvgEfficiencyJTH float64 `json:"avg_efficiency_jth"`
	CriticalEvents   int64   `json:"critical_events"`
	WarningEvents    int64   `json:"warning_events"`
}

type SiteMetrics struct {
	SiteID           string  `json:"site_id"`
	Samples          int64   `json:"samples"`
	ActiveMiners     int64   `json:"active_miners"`
	AvgHashrateTHs   float64 `json:"avg_hashrate_ths"`
	AvgPowerWatts    float64 `json:"avg_power_watts"`
	AvgTempCelsius   float64 `json:"avg_temp_celsius"`
	AvgEfficiencyJTH float64 `json:"avg_efficiency_jth"`
	CriticalEvents   int64   `json:"critical_events"`
	WarningEvents    int64   `json:"warning_events"`
}

type AlertRuleCount struct {
	RuleID string `json:"rule_id"`
	Count  int64  `json:"count"`
}

type AlertMetrics struct {
	Total    int64            `json:"total"`
	Critical int64            `json:"critical"`
	Warning  int64            `json:"warning"`
	TopRules []AlertRuleCount `json:"top_rules"`
}

type ChangeMetrics struct {
	Applied    int64 `json:"applied"`
	RolledBack int64 `json:"rolled_back"`
}

type Hotspot struct {
	SiteID           string  `json:"site_id"`
	RackID           string  `json:"rack_id"`
	MinerID          string  `json:"miner_id"`
	MaxTempCelsius   float64 `json:"max_temp_celsius"`
	AvgTempCelsius   float64 `json:"avg_temp_celsius"`
	AvgHashrateTHs   float64 `json:"avg_hashrate_ths"`
	AvgEfficiencyJTH float64 `json:"avg_efficiency_jth"`
}

type DailyMetrics struct {
	Global   GlobalMetrics `json:"global"`
	Sites    []SiteMetrics `json:"sites"`
	Alerts   AlertMetrics  `json:"alerts"`
	Changes  ChangeMetrics `json:"changes"`
	Hotspots []Hotspot     `json:"hotspots"`
}

type Report struct {
	ReportID           string        `json:"report_id,omitempty"`
	ReportDate         string        `json:"report_date"`
	Timezone           string        `json:"timezone"`
	WindowFrom         time.Time     `json:"window_from"`
	WindowTo           time.Time     `json:"window_to"`
	GeneratedAt        time.Time     `json:"generated_at"`
	Global             GlobalMetrics `json:"global"`
	Sites              []SiteMetrics `json:"sites"`
	Alerts             AlertMetrics  `json:"alerts"`
	Changes            ChangeMetrics `json:"changes"`
	Hotspots           []Hotspot     `json:"hotspots"`
	ExecutiveSummary   []string      `json:"executive_summary"`
	OperationalActions []string      `json:"operational_actions"`
}

func BuildReport(
	reportDate time.Time,
	location *time.Location,
	windowFrom time.Time,
	windowTo time.Time,
	metrics DailyMetrics,
	generatedAt time.Time,
) Report {
	localDate := reportDate.In(location).Format("2006-01-02")

	executive := []string{
		fmt.Sprintf(
			"Se procesaron %d eventos con %d equipos activos en la ventana.",
			metrics.Global.Samples,
			metrics.Global.ActiveMiners,
		),
		thermalExecutiveLine(metrics.Global.AvgTempCelsius, metrics.Global.CriticalEvents),
		fmt.Sprintf(
			"Alertas registradas: total %d, criticas %d, rollbacks %d.",
			metrics.Alerts.Total,
			metrics.Alerts.Critical,
			metrics.Changes.RolledBack,
		),
	}

	actions := []string{
		"Validar outliers de temperatura en top hotspots antes del siguiente turno.",
		"Revisar reglas con mayor volumen de alertas para ajustar umbrales por sitio.",
		"Confirmar efectividad de cambios aplicados vs reportes del dia siguiente.",
	}

	if metrics.Alerts.Critical == 0 {
		actions[0] = "Mantener monitoreo preventivo; no se detectaron alertas criticas en la ventana."
	}
	if metrics.Changes.Applied == 0 {
		actions[2] = "No hubo cambios operativos aplicados; mantener baseline y preparar ventana de prueba controlada."
	}

	return Report{
		ReportDate:         localDate,
		Timezone:           location.String(),
		WindowFrom:         windowFrom,
		WindowTo:           windowTo,
		GeneratedAt:        generatedAt,
		Global:             metrics.Global,
		Sites:              metrics.Sites,
		Alerts:             metrics.Alerts,
		Changes:            metrics.Changes,
		Hotspots:           metrics.Hotspots,
		ExecutiveSummary:   executive,
		OperationalActions: actions,
	}
}

func thermalExecutiveLine(avgTemp float64, criticalEvents int64) string {
	if criticalEvents > 0 {
		return fmt.Sprintf(
			"Temperatura media %.1f C con eventos criticos detectados; mantener foco termico.",
			avgTemp,
		)
	}
	return fmt.Sprintf("Temperatura media %.1f C sin eventos criticos de telemetria.", avgTemp)
}

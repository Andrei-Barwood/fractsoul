package reports

import (
	"fmt"
	"strings"
)

func RenderExecutiveOperationalMarkdown(report Report) string {
	var builder strings.Builder

	builder.WriteString("# Reporte Diario Ejecutivo-Operativo\n\n")
	builder.WriteString(fmt.Sprintf("- Fecha de reporte: `%s`\n", report.ReportDate))
	builder.WriteString(fmt.Sprintf("- Timezone: `%s`\n", report.Timezone))
	builder.WriteString(fmt.Sprintf("- Ventana UTC: `%s` -> `%s`\n", report.WindowFrom.Format("2006-01-02 15:04:05"), report.WindowTo.Format("2006-01-02 15:04:05")))
	builder.WriteString(fmt.Sprintf("- Generado: `%s`\n\n", report.GeneratedAt.Format("2006-01-02 15:04:05 MST")))

	builder.WriteString("## Resumen Ejecutivo\n")
	for _, line := range report.ExecutiveSummary {
		builder.WriteString(fmt.Sprintf("- %s\n", line))
	}
	builder.WriteString("\n")

	builder.WriteString("## KPIs Globales\n")
	builder.WriteString("| KPI | Valor |\n")
	builder.WriteString("| --- | ---: |\n")
	builder.WriteString(fmt.Sprintf("| Eventos procesados | %d |\n", report.Global.Samples))
	builder.WriteString(fmt.Sprintf("| Equipos activos | %d |\n", report.Global.ActiveMiners))
	builder.WriteString(fmt.Sprintf("| Hashrate promedio (TH/s) | %.2f |\n", report.Global.AvgHashrateTHs))
	builder.WriteString(fmt.Sprintf("| Potencia promedio (W) | %.2f |\n", report.Global.AvgPowerWatts))
	builder.WriteString(fmt.Sprintf("| Temperatura promedio (C) | %.2f |\n", report.Global.AvgTempCelsius))
	builder.WriteString(fmt.Sprintf("| Eficiencia promedio (J/TH) | %.2f |\n", report.Global.AvgEfficiencyJTH))
	builder.WriteString(fmt.Sprintf("| Eventos telemetria criticos | %d |\n", report.Global.CriticalEvents))
	builder.WriteString(fmt.Sprintf("| Eventos telemetria warning | %d |\n\n", report.Global.WarningEvents))

	builder.WriteString("## Desempeño por Sitio\n")
	if len(report.Sites) == 0 {
		builder.WriteString("- Sin datos por sitio en la ventana.\n\n")
	} else {
		builder.WriteString("| Sitio | Mineros activos | Hashrate avg (TH/s) | Power avg (W) | Temp avg (C) | Eficiencia avg (J/TH) | Criticos |\n")
		builder.WriteString("| --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")
		for _, site := range report.Sites {
			builder.WriteString(fmt.Sprintf(
				"| %s | %d | %.2f | %.2f | %.2f | %.2f | %d |\n",
				site.SiteID,
				site.ActiveMiners,
				site.AvgHashrateTHs,
				site.AvgPowerWatts,
				site.AvgTempCelsius,
				site.AvgEfficiencyJTH,
				site.CriticalEvents,
			))
		}
		builder.WriteString("\n")
	}

	builder.WriteString("## Alertas y Anomalías\n")
	builder.WriteString(fmt.Sprintf("- Total alertas: `%d`\n", report.Alerts.Total))
	builder.WriteString(fmt.Sprintf("- Alertas criticas: `%d`\n", report.Alerts.Critical))
	builder.WriteString(fmt.Sprintf("- Alertas warning: `%d`\n", report.Alerts.Warning))
	if len(report.Alerts.TopRules) > 0 {
		builder.WriteString("- Top reglas:\n")
		for _, rule := range report.Alerts.TopRules {
			builder.WriteString(fmt.Sprintf("  - `%s`: %d\n", rule.RuleID, rule.Count))
		}
	}
	builder.WriteString("\n")

	builder.WriteString("## Cambios Operativos\n")
	builder.WriteString(fmt.Sprintf("- Cambios aplicados: `%d`\n", report.Changes.Applied))
	builder.WriteString(fmt.Sprintf("- Rollbacks lógicos: `%d`\n\n", report.Changes.RolledBack))

	builder.WriteString("## Top Hotspots\n")
	if len(report.Hotspots) == 0 {
		builder.WriteString("- Sin hotspots relevantes en la ventana.\n\n")
	} else {
		builder.WriteString("| Miner | Sitio | Rack | Max Temp (C) | Avg Temp (C) | Avg Hashrate (TH/s) |\n")
		builder.WriteString("| --- | --- | --- | ---: | ---: | ---: |\n")
		for _, hotspot := range report.Hotspots {
			builder.WriteString(fmt.Sprintf(
				"| %s | %s | %s | %.2f | %.2f | %.2f |\n",
				hotspot.MinerID,
				hotspot.SiteID,
				hotspot.RackID,
				hotspot.MaxTempCelsius,
				hotspot.AvgTempCelsius,
				hotspot.AvgHashrateTHs,
			))
		}
		builder.WriteString("\n")
	}

	builder.WriteString("## Plan de Accion (24h)\n")
	for idx, action := range report.OperationalActions {
		builder.WriteString(fmt.Sprintf("%d. %s\n", idx+1, action))
	}
	builder.WriteString("\n")

	return builder.String()
}

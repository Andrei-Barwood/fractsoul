# D86 - Benchmark pre/post (J/TH, alert latency)

Fecha: 2026-03-25

## Entrega
- Script benchmark comparativo baseline vs hardened:
  - `scripts/benchmark_pre_post.sh`
- Metricas incluidas por perfil:
  - eventos insertados,
  - ingest rate (events/s),
  - promedio J/TH,
  - latencia de alerta promedio y p95 (`alerts.updated_at - telemetry_readings.ingested_at`).
- Reporte markdown generado:
  - `docs/operations/D86_benchmark_pre_post.md`

## Criterio de aceptacion
Existe comparativa reproducible pre/post hardening con resultados cuantitativos y reporte versionable.

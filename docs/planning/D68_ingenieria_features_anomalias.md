# D68 - Ingenieria de features para anomalias

Fecha: 2026-03-24

## Entrega
- `internal/anomaly` calcula features por ventana:
  - promedio/max de temperatura,
  - tendencia de hashrate/power/temp,
  - `hashrate_drop_pct`,
  - `compensated_jth`,
  - banda termica.

## Criterio de aceptacion
Features quedan serializadas en el reporte de analisis por maquina.

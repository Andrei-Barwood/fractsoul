# D76 - Estimador de impacto pre/post cambio

Fecha: 2026-03-25

## Entrega
- El reporte de anomalias agrega `impact_estimate` con:
  - snapshot `before` (hashrate/power/temp/fan/JTH),
  - snapshot `after` estimado tras recomendaciones,
  - `delta` porcentual/absoluto,
  - `confidence` y supuestos del modelo.
- Se usa una aproximacion heuristica y trazable para evaluar impacto esperado antes de aplicar cambios.

## Criterio de aceptacion
El endpoint de anomalias entrega estimacion pre/post util para priorizar cambios operativos.

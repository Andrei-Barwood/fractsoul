# D75 - Recomendador v1 (freq/volt/fan)

Fecha: 2026-03-24

## Entrega
- Recomendador en `internal/anomaly` que produce acciones por parametro:
  - `fan`,
  - `freq`,
  - `volt`.
- Recomendaciones contextualizadas por tipo de anomalia detectada.

## Criterio de aceptacion
El endpoint de analisis devuelve un bloque `recommendations` util para accion operativa inmediata.

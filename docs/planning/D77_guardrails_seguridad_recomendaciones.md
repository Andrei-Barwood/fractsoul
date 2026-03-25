# D77 - Guardrails de seguridad para recomendaciones

Fecha: 2026-03-25

## Entrega
- Se incorporan guardrails por parametro (`fan`, `freq`, `volt`) con acciones:
  - `allow`,
  - `clamped`,
  - `blocked`.
- Las recomendaciones inseguras se corrigen automaticamente y se registra trazabilidad en `guardrails`.
- Se preserva `requested_delta` cuando el valor original es ajustado por seguridad.

## Criterio de aceptacion
El reporte de anomalias no expone ajustes fuera de limites operativos y explica cada intervencion de guardrail.

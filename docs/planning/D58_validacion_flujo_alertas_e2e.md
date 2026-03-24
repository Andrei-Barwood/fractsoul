# D58 - Validacion E2E de alertas

Fecha: 2026-03-24

## Objetivo
Validar el flujo completo: `ingest -> processor -> reglas -> DB alerts -> notificaciones`.

## Pruebas implementadas
- Integracion:
  - `tests/integration/alerts_pipeline_test.go`
  - verifica persistencia de alerta y supresion por duplicado.
- E2E:
  - `tests/e2e/alerts_flow_test.go`
  - inyecta eventos deterministas y valida `occurrences` + `status=suppressed`.
- Script de validacion desde raiz:
  - `scripts/e2e_alerts_flow.sh`.

## Criterio de salida
- Alertas quedan registradas en tabla `alerts`.
- Detecciones repetidas dentro de ventana elevan `occurrences` sin ruido de re-notificacion.
- Resultado de entrega se audita en `alert_notifications`.

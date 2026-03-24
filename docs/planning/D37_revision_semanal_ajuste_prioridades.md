# D37 - Revision semanal + ajuste de prioridades

Fecha: 2026-03-24

## Estado del tramo D31-D37
- Objetivo S2 y criterios definidos (D31).
- Simulador con perfiles ASIC y scheduler configurable (D32-D33).
- Ingesta real sobre JetStream durable (D34).
- Validacion HTTP estricta y errores consistentes (D35).
- Retry + DLQ basica implementados en consumer (D36).

## Evidencia tecnica
- `go test ./...` en modulo `backend/services/ingest-api`: OK
- `make e2e` (tag `e2e`): OK
  - `TestIngestPublishesToNATS`: PASS
  - `TestSimulatorPipelineToDBAndReadAPI`: PASS

## Ajuste de prioridades para siguiente tramo
1. D38: optimizar hypertables/indices en Timescale.
2. D39: politicas de retencion.
3. D40: agregaciones continuas minuto/hora.
4. D41-D42: endpoints por sitio/rack y por maquina/rango temporal.
5. D43: prueba de performance con 100 ASICs y reporte de limites.

## Riesgos vigentes
- Falta instrumentacion avanzada (metricas/latencias de cola y DLQ).
- Sin seguridad baseline aun (API keys/rate limit), planificada para siguiente iteracion.

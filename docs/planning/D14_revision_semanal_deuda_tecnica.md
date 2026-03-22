# D14 - Revision semanal y deuda tecnica

Fecha: 2026-03-22

## Deuda tecnica actual

1. Persistencia real no implementada:
- El endpoint es mock y aun no publica a NATS ni persiste en TimescaleDB.

2. Validacion de contrato parcial:
- Se valida por tags de Gin; falta validacion formal contra JSON Schema en runtime/tests.

3. Seguridad baseline pendiente:
- Falta autenticacion para endpoint de ingesta.
- Falta rate limiting por origen/sitio.

4. Observabilidad incompleta:
- Hay logs estructurados, pero faltan metricas Prometheus y trazas OTel.

5. CI minima:
- Ejecuta lint y tests, pero aun sin cobertura minima obligatoria ni escaneo SAST.

## Propuesta de pago de deuda (S2)

1. Implementar publisher NATS JetStream con retry/backoff y DLQ.
2. Agregar consumer de telemetria + persistencia en TimescaleDB.
3. Instrumentar metricas de ingesta (`throughput`, `latency`, `error_rate`).
4. Incorporar autenticacion por API key y controles de abuso.
5. Agregar contract tests y gates de cobertura en CI.

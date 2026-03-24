# D30 - Retro S1 + plan detallado S2

Fecha: 2026-03-24

## Retro S1

### Lo que funciono
- Avance rapido de arquitectura base a pipeline operativo.
- Contratos y normalizacion de IDs alineados con modelo de datos.
- Simulador util para pruebas funcionales y de carga inicial.

### Lo que falta robustecer
- Validacion runtime contra JSON schema (ademas de tags Gin).
- JetStream durable consumer, retry/backoff y DLQ.
- Observabilidad avanzada (Prometheus/OTel) y seguridad baseline.

## Plan S2 (detalle corto)
1. Migrar publicacion/consumo a JetStream durable con retry policy.
2. Instrumentar metricas de servicio (`throughput`, `latency`, `error_rate`).
3. Agregar autenticacion API key + rate limit por sitio.
4. Incorporar contract tests y gate de cobertura CI.
5. Preparar dashboard operativo inicial sobre API de lectura.

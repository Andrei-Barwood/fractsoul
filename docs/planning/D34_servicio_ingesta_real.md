# D34 - Implementar servicio de ingesta real

Fecha: 2026-03-24

## Resultado
La API de ingesta queda integrada a JetStream con stream administrado por aplicacion y consumo durable.

## Implementacion tecnica
- Publicador NATS migrado a JetStream:
  - archivo: `backend/services/ingest-api/internal/telemetry/nats_publisher.go`
  - inicializa stream (`TELEMETRY_STREAM`) y asegura subjects requeridos.
- Arranque de aplicacion actualizado:
  - archivo: `backend/services/ingest-api/internal/app/run.go`
  - configura publisher con `subject` y `dlq subject`.
  - instancia consumer con config durable/retry.
- Configuracion extendida:
  - archivo: `backend/services/ingest-api/internal/app/config.go`
  - variables nuevas:
    - `TELEMETRY_STREAM`
    - `TELEMETRY_DLQ_SUBJECT`
    - `TELEMETRY_CONSUMER_DURABLE`
    - `PROCESSOR_MAX_DELIVER`
    - `PROCESSOR_RETRY_DELAY`
    - `INGEST_MAX_BODY_BYTES`

## Operacion local
- `docker-compose.yml` actualizado con defaults S2.

## Criterio de salida
Ingesta deja de depender de pub/sub efimero y opera sobre stream durable con configuracion explicita.

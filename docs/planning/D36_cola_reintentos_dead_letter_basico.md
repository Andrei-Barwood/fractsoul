# D36 - Cola de reintentos y dead-letter basico

Fecha: 2026-03-24

## Resultado
El processor de telemetria incorpora politica de reintento con backoff y derivacion a DLQ en falla terminal.

## Implementacion
- Archivo: `backend/services/ingest-api/internal/processor/consumer.go`
- Caracteristicas:
  - suscripcion durable en JetStream (`Durable`, `ManualAck`, `MaxDeliver`)
  - reintento con `NakWithDelay(PROCESSOR_RETRY_DELAY)` ante falla de persistencia
  - publicacion a `TELEMETRY_DLQ_SUBJECT` cuando se alcanza `PROCESSOR_MAX_DELIVER`
  - ack final del mensaje terminal para evitar loop de reprocesamiento
  - payload DLQ enriquecido con metadata (`subject`, `num_delivered`, `failure_reason`, `event_id`)

## Parametros operativos
- `PROCESSOR_MAX_DELIVER` (default `5`)
- `PROCESSOR_RETRY_DELAY` (default `2s`)
- `TELEMETRY_DLQ_SUBJECT` (default `telemetry.dlq.v1`)

## Criterio de salida
Fallas transitorias se reintentan automaticamente y fallas persistentes quedan trazables en DLQ.

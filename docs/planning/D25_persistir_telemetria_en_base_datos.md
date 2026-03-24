# D25 - Persistir telemetria en base de datos

Fecha: 2026-03-24

## Resultado
Persistencia implementada sobre Timescale/Postgres:
- `internal/processor/consumer.go`
- `internal/storage/postgres.go`

## Flujo
1. Ingest API publica evento en NATS.
2. Consumer suscrito al subject persiste en `telemetry_readings`.
3. Upsert automatico de entidades (`sites`, `racks`, `miners`).

## Criterio de salida
- Insercion idempotente por `event_id + ts`.
- Trazabilidad de payload original en `raw_payload`.

# D42 - Endpoint por maquina y rango temporal

Fecha: 2026-03-24

## Resultado
Se agrego endpoint de serie temporal por ASIC con resolucion configurable.

## Endpoint
`GET /v1/telemetry/miners/:miner_id/timeseries`

## Parametros
- `from` (RFC3339, opcional)
- `to` (RFC3339, opcional)
- `resolution` (`minute|hour`, default `minute`)
- `limit` (max 10000)

## Respuesta
Devuelve buckets agregados con:
- `samples`
- `avg_hashrate_ths`
- `avg_power_watts`
- `avg_temp_celsius`
- `max_temp_celsius`
- `avg_fan_rpm`
- `avg_efficiency_jth`
- `critical_events`

## Implementacion
- Consulta preferente sobre continuous aggregates (`telemetry_agg_minute`/`telemetry_agg_hour`).
- Fallback automatico a `telemetry_readings` si la vista aun no existe o no tiene datos materializados.

## Archivos
- `backend/services/ingest-api/internal/httpapi/router.go`
- `backend/services/ingest-api/internal/httpapi/telemetry_read_handler.go`
- `backend/services/ingest-api/internal/storage/repository.go`
- `backend/services/ingest-api/internal/storage/postgres.go`
- `docs/contracts/telemetry_miner_timeseries_response_v1.example.json`

## Criterio de salida
Operaciones pueden consultar tendencias por maquina en ventana temporal y resolucion ajustable.

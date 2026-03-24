# D40 - Crear agregaciones continuas por minuto/hora

Fecha: 2026-03-24

## Resultado
Se crearon continuous aggregates para lectura eficiente de series temporales por equipo.

## Vistas continuas
- `telemetry_agg_minute`
- `telemetry_agg_hour`

## Campos agregados
- `samples`
- `avg_hashrate_ths`
- `avg_power_watts`
- `avg_temp_celsius`
- `max_temp_celsius`
- `avg_fan_rpm`
- `avg_efficiency_jth`
- `critical_events` (status `critical` u `offline`)

## Politicas
- Refresh minuto:
  - `start_offset: 6 hours`
  - `end_offset: 1 minute`
  - `schedule_interval: 1 minute`
- Refresh hora:
  - `start_offset: 30 days`
  - `end_offset: 1 hour`
  - `schedule_interval: 15 minutes`

## Archivos
- `infra/db/migrations/0002_timescale_optimizations_s2.sql`
- `backend/services/ingest-api/internal/storage/postgres.go` (consulta con fallback a tabla cruda)

## Criterio de salida
Series por maquina en rango temporal consultables por bucket (`minute|hour`) sin depender de full scans sobre datos crudos.

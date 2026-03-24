# D17 - Disenar modelo logico Timescale/Postgres

Fecha: 2026-03-24

## Modelo logico v1

### Tablas maestras
- `sites`
  - PK: `site_id`
- `racks`
  - PK: `rack_id`
  - FK: `site_id -> sites.site_id`
- `miners`
  - PK: `miner_id`
  - FK: `site_id -> sites.site_id`
  - FK: `rack_id -> racks.rack_id`

### Tabla de series temporales
- `telemetry_readings` (hypertable Timescale por `ts`)
  - claves operativas: `site_id`, `rack_id`, `miner_id`
  - metricas tipadas: `hashrate_ths`, `power_watts`, `temp_celsius`, `fan_rpm`, `efficiency_jth`, `status`, `load_pct`
  - metadata: `tags`, `raw_payload`, `ingested_at`
  - idempotencia: `UNIQUE(event_id, ts)`

## Indices principales
- `site_id, ts DESC`
- `rack_id, ts DESC`
- `miner_id, ts DESC`
- `status, ts DESC`

## Consultas objetivo cubiertas
- Ultimas lecturas por sitio/rack/miner.
- Agregados por ventana temporal (promedios y `p95` de temperatura).
- Soporte para dashboard operativo en casi tiempo real.

## Artefactos
- Migraciones SQL iniciales en `infra/db/migrations/`.
- Esquema de bootstrap local actualizado en `infra/docker/timescaledb/init/`.

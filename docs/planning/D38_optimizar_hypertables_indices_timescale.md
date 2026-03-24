# D38 - Optimizar hypertables/indices en Timescale

Fecha: 2026-03-24

## Resultado
Se aplico una optimizacion de almacenamiento y consulta sobre `telemetry_readings`.

## Cambios implementados
- Chunk interval ajustado a `1 day`.
- Compresion habilitada en hypertable:
  - `compress_orderby = ts DESC`
  - `compress_segmentby = site_id,rack_id,miner_id`
- Politica de compresion agregada a partir de `7 days`.
- Nuevos indices para consultas S2:
  - `(site_id, rack_id, ts DESC)`
  - `(miner_id, status, ts DESC)`
  - `BRIN(ts)` para scans por rango temporal amplio.

## Archivos
- `infra/db/migrations/0002_timescale_optimizations_s2.sql`
- `infra/docker/timescaledb/init/002_s2_optimizations.sql`

## Criterio de salida
Consultas por sitio/rack y por maquina+rango tienen soporte de indice y mejor comportamiento para volumen creciente.

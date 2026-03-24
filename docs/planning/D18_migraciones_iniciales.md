# D18 - Crear migraciones iniciales

Fecha: 2026-03-24

## Resultado
Se implementa baseline SQL-first versionado:
- `infra/db/migrations/0001_initial_schema.sql`
- `scripts/migrate_timescaledb.sh`
- `scripts/bootstrap_timescaledb.sh` (wrapper de migraciones)

## Cobertura
- Extensiones y tablas maestras (`sites`, `racks`, `miners`).
- Hypertable `telemetry_readings` con indices operativos.
- Vista `telemetry_latest` para consultas de ultima lectura por equipo.
- Tabla `schema_migrations` para control incremental.

## Criterio de salida
- Migraciones aplicables en orden, idempotentes y trazables por version.

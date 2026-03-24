# Infra Docker

Recursos de contenedores para desarrollo local.

El compose principal esta en `/docker-compose.yml` en la raiz del monorepo.

## TimescaleDB bootstrap

Scripts de inicializacion:
- `infra/docker/timescaledb/init/001_init_schema.sql`
- `infra/db/migrations/0001_initial_schema.sql`
- `infra/db/seeds/001_synthetic_fleet.sql`

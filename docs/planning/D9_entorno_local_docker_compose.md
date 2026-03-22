# D9 - Entorno local con Docker Compose

Fecha: 2026-03-22

## Objetivo
Levantar stack local minimo para iterar el MVP.

## Implementacion
Archivo principal: `docker-compose.yml`.

Servicios:
- `ingest-api` (Go/Gin)
- `timescaledb` (PostgreSQL + TimescaleDB)
- `nats` (JetStream habilitado)

## Notas
- Se agregaron healthchecks por servicio.
- Docker Desktop se instalo via Homebrew (`docker-desktop`) y `docker`/`docker compose` quedaron operativos.
- Se monto init SQL para TimescaleDB en `infra/docker/timescaledb/init/001_init_schema.sql`.

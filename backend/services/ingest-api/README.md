# Ingest API

API mock para ingesta de telemetria del MVP.

## Ejecutar local

```bash
go run ./cmd/api
```

Variables:
- `APP_PORT` (default `8080`)
- `GIN_MODE` (default `release`)
- `LOG_LEVEL` (default `info`)
- `NATS_URL` (default `nats://localhost:4222`)
- `TELEMETRY_SUBJECT` (default `telemetry.raw.v1`)
- `DATABASE_URL` (ej: `postgres://postgres:postgres@localhost:5432/mining?sslmode=disable`)
- `TELEMETRY_PROCESSOR_ENABLED` (default `true`)

## Endpoints
- `GET /healthz`
- `POST /v1/telemetry/ingest`
- `GET /v1/telemetry/readings`
- `GET /v1/telemetry/summary`

Ejemplos:

```bash
curl "http://localhost:8080/v1/telemetry/readings?site_id=site-cl-01&limit=5"
curl "http://localhost:8080/v1/telemetry/summary?window_minutes=60"
```

## Validacion y errores
Se aplica validacion de payload y contrato de error uniforme con `request_id`.

## Prueba E2E

Con compose levantado:

```bash
make e2e
```

E2E full stack desde raiz del repo:

```bash
./scripts/e2e_simulator_db_api.sh
```

## Simulador ASIC

Ejecutar simulador de 100 equipos:

```bash
make simulate
```

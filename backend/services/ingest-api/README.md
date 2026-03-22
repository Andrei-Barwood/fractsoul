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

## Endpoints
- `GET /healthz`
- `POST /v1/telemetry/ingest`

## Validacion y errores
Se aplica validacion de payload y contrato de error uniforme con `request_id`.

## Prueba E2E

Con compose levantado:

```bash
make e2e
```

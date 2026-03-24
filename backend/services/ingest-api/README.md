# Ingest API

Servicio de ingesta de telemetria del MVP (HTTP -> JetStream -> processor -> TimescaleDB).

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
- `TELEMETRY_STREAM` (default `TELEMETRY`)
- `TELEMETRY_DLQ_SUBJECT` (default `telemetry.dlq.v1`)
- `TELEMETRY_CONSUMER_DURABLE` (default `telemetry-processor-v1`)
- `PROCESSOR_MAX_DELIVER` (default `5`)
- `PROCESSOR_RETRY_DELAY` (default `2s`)
- `INGEST_MAX_BODY_BYTES` (default `1048576`)
- `DATABASE_URL` (ej: `postgres://postgres:postgres@localhost:5432/mining?sslmode=disable`)
- `TELEMETRY_PROCESSOR_ENABLED` (default `true`)

## Endpoints
- `GET /healthz`
- `POST /v1/telemetry/ingest`
- `GET /v1/telemetry/readings`
- `GET /v1/telemetry/summary`
- `GET /v1/telemetry/sites/:site_id/racks/:rack_id/readings`
- `GET /v1/telemetry/miners/:miner_id/timeseries`

Ejemplos:

```bash
curl "http://localhost:8080/v1/telemetry/readings?site_id=site-cl-01&limit=5"
curl "http://localhost:8080/v1/telemetry/summary?window_minutes=60"
curl "http://localhost:8080/v1/telemetry/sites/site-cl-01/racks/rack-cl-01-01/readings?status=warning&limit=20"
curl "http://localhost:8080/v1/telemetry/miners/asic-000001/timeseries?resolution=minute&from=2026-03-24T00:00:00Z&to=2026-03-24T12:00:00Z"
```

## Validacion y errores
Se aplica:
- parseo JSON estricto (`DisallowUnknownFields`)
- limite de body (`INGEST_MAX_BODY_BYTES`)
- validacion de contrato (`binding tags`)
- contrato de error uniforme con `request_id`

## Prueba E2E

Con compose levantado:

```bash
make e2e
```

Prueba de performance con 100 ASICs:

```bash
make perf
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

Flags relevantes del simulador:
- `-profile-mode` (`mixed|s19xp|s21|m50`)
- `-schedule` (`burst|staggered`)
- `-schedule-jitter` (ej. `250ms`)

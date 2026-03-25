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
- `API_AUTH_ENABLED` (default `false`)
- `API_KEY_HEADER` (default `X-API-Key`)
- `API_KEYS` (lista separada por comas)
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
- `ALERTS_ENABLED` (default `true`)
- `ALERT_SUPPRESS_WINDOW` (default `10m`)
- `ALERT_NOTIFY_TIMEOUT` (default `3s`)
- `ALERT_NOTIFY_RETRIES` (default `3`)
- `ALERT_NOTIFY_BACKOFF` (default `500ms`)
- `ALERT_QUEUE_SIZE` (default `256`)
- `ALERT_WORKER_COUNT` (default `2`)
- `ALERT_WEBHOOK_ENABLED` (default `false`)
- `ALERT_WEBHOOK_URL`
- `ALERT_WEBHOOK_HEADER` (default `Authorization`)
- `ALERT_WEBHOOK_TOKEN`
- `ALERT_EMAIL_ENABLED` (default `false`)
- `ALERT_SMTP_ADDR` (ej: `localhost:1025`)
- `ALERT_SMTP_USERNAME`
- `ALERT_SMTP_PASSWORD`
- `ALERT_EMAIL_FROM` (default `alerts@fractsoul.local`)
- `ALERT_EMAIL_TO` (lista separada por comas)
- `ALERT_EMAIL_SUBJECT_PREFIX` (default `[Fractsoul Alert]`)
- `REPORT_MODE` (`once|daemon`, default `once`)
- `REPORT_DATE` (opcional `YYYY-MM-DD` para modo `once`)
- `REPORT_TIMEZONE` (default `UTC`)
- `REPORT_SCHEDULE` (default `08:00`, usado en modo `daemon`)
- `REPORT_RUN_ON_STARTUP` (default `true`)
- `REPORT_OUTPUT_DIR` (opcional, guarda markdown generado)

## Endpoints
- `GET /healthz`
- `POST /v1/telemetry/ingest`
- `GET /v1/telemetry/readings`
- `GET /v1/telemetry/summary`
- `GET /v1/telemetry/sites/:site_id/racks/:rack_id/readings`
- `GET /v1/telemetry/miners/:miner_id/timeseries`
- `GET /v1/efficiency/miners`
- `GET /v1/efficiency/racks`
- `GET /v1/efficiency/sites`
- `GET /v1/anomalies/miners/:miner_id/analyze`
- `POST /v1/anomalies/miners/:miner_id/changes/apply`
- `POST /v1/anomalies/changes/:change_id/rollback`
- `GET /v1/anomalies/changes`
- `GET /dashboard/` (dashboard v0 embebido)

Ejemplos:

```bash
curl "http://localhost:8080/v1/telemetry/readings?site_id=site-cl-01&model=s21&limit=5"
curl "http://localhost:8080/v1/telemetry/summary?window_minutes=60&model=s21"
curl "http://localhost:8080/v1/telemetry/sites/site-cl-01/racks/rack-cl-01-01/readings?status=warning&model=s21&limit=20"
curl "http://localhost:8080/v1/telemetry/miners/asic-000001/timeseries?resolution=minute&from=2026-03-24T00:00:00Z&to=2026-03-24T12:00:00Z"
curl "http://localhost:8080/v1/efficiency/miners?window_minutes=120&limit=20"
curl "http://localhost:8080/v1/efficiency/racks?site_id=site-cl-01&window_minutes=120"
curl "http://localhost:8080/v1/efficiency/sites?window_minutes=120"
curl "http://localhost:8080/v1/anomalies/miners/asic-000001/analyze?resolution=minute&from=2026-03-24T00:00:00Z&to=2026-03-24T12:00:00Z"
curl -X POST "http://localhost:8080/v1/anomalies/miners/asic-000001/changes/apply?resolution=minute&limit=120" \
  -H "Content-Type: application/json" \
  -d '{"reason":"operator dry-run","requested_by":"ops@fractsoul.local"}'
curl "http://localhost:8080/v1/anomalies/changes?miner_id=asic-000001&status=applied&limit=20"
curl -X POST "http://localhost:8080/v1/anomalies/changes/<change_id>/rollback" \
  -H "Content-Type: application/json" \
  -d '{"reason":"rollback post-check","requested_by":"ops@fractsoul.local"}'
# Si API auth esta habilitada:
curl -H "X-API-Key: local-dev-key" "http://localhost:8080/v1/telemetry/readings?limit=5"
```

Dashboard v0:

```bash
open http://localhost:8080/dashboard/
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

Prueba de integracion API/DB:

```bash
make integration
```

Prueba de performance con 100 ASICs:

```bash
make perf
```

Prueba E2E de alertas (dedupe/supresion):

```bash
make alerts-e2e
```

Reporte diario (una ejecucion):

```bash
make report-once
```

Reporte diario en modo daemon:

```bash
make report-daemon
```

E2E full stack desde raiz del repo:

```bash
./scripts/e2e_simulator_db_api.sh
./scripts/e2e_alerts_flow.sh
./scripts/demo_s2_fallas_simuladas.sh
./scripts/generate_daily_report.sh
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
- `-api-key` (si auth esta habilitada)

Tags adicionales emitidos por simulador para analitica:
- `ambient_temp_c`
- `freq_mhz`
- `volt_mv`

## OpenAPI

Especificacion minima publicada en:
- `docs/openapi/ingest_api_v1.yaml`

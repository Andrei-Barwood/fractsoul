# Fractsoul MVP Monorepo

Monorepo base para el MVP de operacion de granjas de Bitcoin mining.

## Estructura

- `backend/services/ingest-api`: servicio de ingesta + lectura de telemetria (Go + Gin + NATS + Postgres).
- `frontend/apps/dashboard`: placeholder de UI operativa.
- `infra/docker`: recursos de contenedores para desarrollo local.
- `docs/planning`: documentos de ejecucion D1-D37 y ADRs.
- `docs/contracts`: contratos JSON/schema.
- `docs/engineering`: convenciones tecnicas.

## Quickstart local

1. Levantar servicios:

```bash
docker compose up --build
```

2. Probar healthcheck:

```bash
curl http://localhost:8080/healthz
```

3. Probar endpoint de ingesta:

```bash
curl -X POST http://localhost:8080/v1/telemetry/ingest \
  -H 'Content-Type: application/json' \
  -d @docs/contracts/telemetry_event_v1.example.json
```

4. Aplicar schema TimescaleDB (si el volumen ya existia):

```bash
./scripts/bootstrap_timescaledb.sh
```

5. Cargar seed sintetico (100 equipos):

```bash
./scripts/seed_synthetic_data.sh
```

6. Ejecutar prueba E2E HTTP -> NATS:

```bash
cd backend/services/ingest-api
make e2e
```

7. Ejecutar simulador ASIC (100 equipos):

```bash
cd backend/services/ingest-api
make simulate
```

8. Verificacion E2E completa (simulador -> DB -> API):

```bash
./scripts/e2e_simulator_db_api.sh
```

## CI

La pipeline minima corre en `.github/workflows/ci.yml` e incluye:

- lint (`gofmt` + `go vet`)
- tests (`go test ./...`)

## Documentacion de planning

Ver [docs/planning/README.md](docs/planning/README.md).

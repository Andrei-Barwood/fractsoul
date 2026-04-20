# Fractsoul MVP Monorepo

Monorepo base para el MVP de operacion de granjas de Bitcoin mining.

## Estructura

- `backend/services/ingest-api`: servicio de ingesta + lectura de telemetria (Go + Gin + NATS + Postgres).
- `backend/services/energy-orchestrator`: servicio de presupuesto de potencia, snapshots y validacion de dispatch (Go + Gin + Postgres + NATS).
- `frontend/apps/dashboard`: placeholder de UI operativa.
- `infra/docker`: recursos de contenedores para desarrollo local.
- `docs/planning`: documentos de ejecucion D1-D104 y ADRs.
- `docs/operations`: evidencias operativas (backup/restore, resiliencia, benchmark, demo final).
- `docs/contracts`: contratos JSON/schema.
- `docs/engineering`: convenciones tecnicas.

## Quickstart local

1. Levantar servicios:

```bash
# Opcional: copiar base de variables
cp .env.example .env

docker compose up --build
```

2. Probar healthcheck:

```bash
curl http://localhost:8080/healthz
```

Métricas de observabilidad:

```bash
curl -s http://localhost:8080/metrics | head -40
```

Dashboard operativo v0:

```bash
open http://localhost:8080/dashboard/
```

Budget operativo inicial del `energy-orchestrator`:

```bash
curl "http://localhost:8081/v1/energy/sites/site-cl-01/budget?include_context=true"
curl "http://localhost:8081/v1/energy/sites/site-cl-02/operations?include_context=true"
curl "http://localhost:8081/v1/energy/sites/site-cl-02/replay/historical?day=$(date -u +%F)"
open http://localhost:8081/dashboard/energy/
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

5. Cargar seed sintetico y energetico base:

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

9. Prueba de performance (100 ASICs):

```bash
./scripts/perf_100_asic.sh
```

10. Prueba de integracion API/DB:

```bash
./scripts/integration_api_db.sh
```

11. Validacion E2E de alertas (reglas + dedupe):

```bash
./scripts/e2e_alerts_flow.sh
```

12. Demo S2 con fallas simuladas:

```bash
./scripts/demo_s2_fallas_simuladas.sh
```

13. Consultar eficiencia/anomalias (S3):

```bash
curl "http://localhost:8080/v1/efficiency/miners?window_minutes=120&limit=5"
curl "http://localhost:8080/v1/anomalies/miners/asic-000001/analyze?resolution=minute&limit=120"
```

14. Generar reporte diario ejecutivo-operativo (S3):

```bash
./scripts/generate_daily_report.sh
```

15. Hardening/resiliencia/benchmark (S3 cierre):

```bash
./scripts/test_backup_restore.sh
./scripts/test_resilience_restart_failover.sh
./scripts/benchmark_pre_post.sh
```

16. Demo final S3 (5 minutos):

```bash
./scripts/demo_s3_final_5min.sh
```

17. Validar presupuesto, snapshots y dispatch energetico inicial (S4 bootstrap):

```bash
curl "http://localhost:8081/v1/energy/overview"
curl "http://localhost:8081/v1/energy/sites/site-cl-01/budget?include_context=true"
curl "http://localhost:8081/v1/energy/sites/site-cl-02/operations?include_context=true"
curl "http://localhost:8081/v1/energy/sites/site-cl-02/constraints/active"
curl "http://localhost:8081/v1/energy/sites/site-cl-02/recommendations/pending"
curl "http://localhost:8081/v1/energy/sites/site-cl-02/recommendations/reviews"
curl "http://localhost:8081/v1/energy/sites/site-cl-02/actions/blocked"
curl "http://localhost:8081/v1/energy/sites/site-cl-02/explanations"
curl "http://localhost:8081/v1/energy/sites/site-cl-02/replay/historical?day=$(date -u +%F)"
curl "http://localhost:8081/v1/energy/sites/site-cl-02/pilot/shadow?day=$(date -u +%F)"
curl -X POST "http://localhost:8081/v1/energy/sites/site-cl-01/dispatch/validate" \
  -H 'Content-Type: application/json' \
  -d @docs/contracts/energy_dispatch_validate_request_v1.example.json
curl -X POST "http://localhost:8081/v1/energy/sites/site-cl-02/recommendations/reviews" \
  -H 'Content-Type: application/json' \
  -d @docs/contracts/energy_recommendation_review_request_v1.example.json
```

18. Ejecutar la prueba E2E completa del `energy-orchestrator` con un solo comando:

```bash
./scripts/e2e_energy_orchestrator.sh
```

## CI

La pipeline minima corre en `.github/workflows/ci.yml` e incluye:

- lint (`gofmt` + `go vet`)
- tests (`go test ./...`)

## Documentacion de planning

Ver [docs/planning/README.md](docs/planning/README.md).

## OpenAPI

Especificacion minima disponible en:
- [docs/openapi/ingest_api_v1.yaml](docs/openapi/ingest_api_v1.yaml)

Contratos base del `energy-orchestrator` en:
- [docs/contracts/energy_load_budget_response_v1.example.json](docs/contracts/energy_load_budget_response_v1.example.json)
- [docs/contracts/energy_dispatch_validate_request_v1.example.json](docs/contracts/energy_dispatch_validate_request_v1.example.json)
- [docs/contracts/energy_dispatch_validate_response_v1.example.json](docs/contracts/energy_dispatch_validate_response_v1.example.json)
- [docs/contracts/energy_operations_response_v1.example.json](docs/contracts/energy_operations_response_v1.example.json)
- [docs/contracts/energy_replay_historical_response_v1.example.json](docs/contracts/energy_replay_historical_response_v1.example.json)
- [docs/contracts/energy_campus_overview_response_v1.example.json](docs/contracts/energy_campus_overview_response_v1.example.json)
- [docs/contracts/energy_shadow_pilot_response_v1.example.json](docs/contracts/energy_shadow_pilot_response_v1.example.json)
- [docs/contracts/energy_recommendation_review_request_v1.example.json](docs/contracts/energy_recommendation_review_request_v1.example.json)
- [docs/contracts/energy_recommendation_review_response_v1.example.json](docs/contracts/energy_recommendation_review_response_v1.example.json)

Planning reciente del `energy-orchestrator`:
- [docs/planning/D101_energy_orchestrator_priorizacion_carga.md](docs/planning/D101_energy_orchestrator_priorizacion_carga.md)
- [docs/planning/D102_energy_orchestrator_rampas_suaves.md](docs/planning/D102_energy_orchestrator_rampas_suaves.md)
- [docs/planning/D103_energy_orchestrator_replay_historico.md](docs/planning/D103_energy_orchestrator_replay_historico.md)
- [docs/planning/D104_energy_orchestrator_endpoints_operativos.md](docs/planning/D104_energy_orchestrator_endpoints_operativos.md)
- [docs/planning/D105_energy_orchestrator_dashboard_web.md](docs/planning/D105_energy_orchestrator_dashboard_web.md)
- [docs/planning/D106_energy_orchestrator_auth_rbac_gobernanza.md](docs/planning/D106_energy_orchestrator_auth_rbac_gobernanza.md)
- [docs/planning/D107_energy_orchestrator_pruebas_operativas.md](docs/planning/D107_energy_orchestrator_pruebas_operativas.md)
- [docs/planning/D108_energy_orchestrator_piloto_sombra.md](docs/planning/D108_energy_orchestrator_piloto_sombra.md)
- [docs/planning/D109_energy_orchestrator_operacion_supervisada.md](docs/planning/D109_energy_orchestrator_operacion_supervisada.md)

# Energy Orchestrator

Servicio inicial para gobierno de potencia, snapshots operativos y validacion de dispatch del campus minero.

## Objetivos de esta primera entrega

- modelar inventario energetico del sitio,
- calcular presupuesto de potencia por sitio y rack,
- aplicar derating por ambiente y margen operativo,
- validar recomendaciones contra limites de `site`, `bus`, `feeder`, `PDU` y densidad termica,
- persistir snapshots del presupuesto calculado,
- emitir eventos canonicos a NATS/JetStream,
- enriquecer respuestas con contexto de `fractsoul`,
- operar en modo `advisory-first`.

## Objetivos del ultimo tramo (D105-D109)

- exponer dashboard web embebida para campus y sitio,
- proyectar riesgo operativo a 4 horas,
- aplicar scopes por `site_id` y `rack_id`,
- persistir reviews con doble aprobacion para acciones sensibles,
- ejecutar piloto sombra sobre dias historicos,
- dejar runbooks y checklist de paso a operacion supervisada.

## Ejecutar local

```bash
go run ./cmd/api
```

Prueba E2E de un comando:

```bash
./scripts/e2e_energy_orchestrator.sh
```

Variables:

- `APP_PORT` (default `8081`)
- `GIN_MODE` (default `release`)
- `LOG_LEVEL` (default `info`)
- `DATABASE_URL` (ej: `postgres://postgres:postgres@localhost:5432/mining?sslmode=disable`)
- `API_AUTH_ENABLED` (default `false`)
- `API_RBAC_ENABLED` (default `false`)
- `API_KEY_HEADER` (default `X-API-Key`)
- `API_KEYS`
- `API_DEFAULT_ROLE` (default `admin`)
- `API_KEY_ROLES`
- `API_KEY_PRINCIPALS`
- `API_KEY_SITE_SCOPES`
- `API_KEY_RACK_SCOPES`
- `ENERGY_DEFAULT_AT` (default `now`)
- `ENERGY_EVENTS_ENABLED` (default `true`)
- `NATS_URL` (default `nats://localhost:4222`)
- `ENERGY_STREAM` (default `ENERGY`)
- `FRACTSOUL_API_BASE_URL` (opcional, ej: `http://localhost:8080`)
- `FRACTSOUL_API_KEY` (opcional, requerido si el upstream usa auth)
- `FRACTSOUL_API_TIMEOUT` (default `5s`)
- `ENERGY_CONTEXT_RACK_LIMIT` (default `3`)
- `ENERGY_CONTEXT_WINDOW_MINUTES` (default `60`)
- `E2E_SKIP_BUILD` (solo para `scripts/e2e_energy_orchestrator.sh`)
- `ENERGY_OPERATIONAL_SITE_ID` (solo para `scripts/e2e_energy_orchestrator.sh`)
- `ENERGY_REPLAY_DAY` (solo para `scripts/e2e_energy_orchestrator.sh`)

Para secretos sensibles se soporta `<ENV>_FILE`.

## Endpoints

- `GET /healthz`
- `GET /metrics`
- `GET /dashboard/energy/`
- `GET /v1/energy/overview`
- `GET /v1/energy/sites/:site_id/budget`
- `GET /v1/energy/sites/:site_id/operations`
- `GET /v1/energy/sites/:site_id/constraints/active`
- `GET /v1/energy/sites/:site_id/recommendations/pending`
- `GET /v1/energy/sites/:site_id/recommendations/reviews`
- `GET /v1/energy/sites/:site_id/actions/blocked`
- `GET /v1/energy/sites/:site_id/explanations`
- `GET /v1/energy/sites/:site_id/replay/historical?day=YYYY-MM-DD`
- `GET /v1/energy/sites/:site_id/pilot/shadow?day=YYYY-MM-DD`
- `POST /v1/energy/sites/:site_id/dispatch/validate`
- `POST /v1/energy/sites/:site_id/recommendations/reviews`

Ejemplos:

```bash
curl "http://localhost:8081/v1/energy/sites/site-cl-01/budget?include_context=true"
curl "http://localhost:8081/v1/energy/overview"
curl "http://localhost:8081/v1/energy/sites/site-cl-02/operations?include_context=true"
curl "http://localhost:8081/v1/energy/sites/site-cl-02/constraints/active"
curl "http://localhost:8081/v1/energy/sites/site-cl-02/recommendations/pending"
curl "http://localhost:8081/v1/energy/sites/site-cl-02/recommendations/reviews"
curl "http://localhost:8081/v1/energy/sites/site-cl-02/actions/blocked"
curl "http://localhost:8081/v1/energy/sites/site-cl-02/explanations"
curl "http://localhost:8081/v1/energy/sites/site-cl-02/replay/historical?day=$(date -u +%F)"
curl "http://localhost:8081/v1/energy/sites/site-cl-02/pilot/shadow?day=$(date -u +%F)"
curl "http://localhost:8081/v1/energy/sites/site-cl-01/budget?ambient_celsius=31.5&context_rack_limit=2&context_window_minutes=120"
curl -X POST "http://localhost:8081/v1/energy/sites/site-cl-01/dispatch/validate" \
  -H "Content-Type: application/json" \
  -d @../../../docs/contracts/energy_dispatch_validate_request_v1.example.json
curl -X POST "http://localhost:8081/v1/energy/sites/site-cl-02/recommendations/reviews" \
  -H "Content-Type: application/json" \
  -d @../../../docs/contracts/energy_recommendation_review_request_v1.example.json
```

RBAC cuando `API_RBAC_ENABLED=true`:

- `viewer`: lectura de presupuesto, overview, operaciones, restricciones, recomendaciones, reviews, bloqueos, explicaciones y replay.
- `operator`: lectura + validacion de dispatch + reviews operativos.
- `admin`: igual que `operator`.

## Integracion con Fractsoul

Esta primera version usa:

- `sites`, `racks`, `telemetry_latest`,
- tablas nuevas `energy_*` y `energy_budget_snapshots`,
- telemetria reciente para carga actual y ambiente derivado.

Cuando `FRACTSOUL_API_BASE_URL` esta configurado, el servicio consulta:

- `GET /v1/efficiency/sites`
- `GET /v1/efficiency/racks`
- `GET /v1/telemetry/summary`
- `GET /v1/telemetry/sites/:site_id/racks/:rack_id/readings`
- `GET /v1/anomalies/miners/:miner_id/analyze`

Los eventos canonicos emitidos son:

- `energy.load_budget_updated.v1`
- `energy.curtailment_recommended.v1`
- `energy.dispatch_rejected.v1`

Los contratos canonicos versionados viven en `docs/contracts`.

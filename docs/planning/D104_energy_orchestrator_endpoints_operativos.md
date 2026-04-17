# D104 - Endpoints operativos y explicabilidad

Fecha: 2026-04-17

## Objetivo

Exponer la capa de decision del `energy-orchestrator` para que operadores y tooling puedan consultar no solo el presupuesto, sino tambien restricciones activas, recomendaciones pendientes, bloqueos y explicaciones legibles.

## Endpoints agregados

- `GET /v1/energy/sites/:site_id/operations`
- `GET /v1/energy/sites/:site_id/constraints/active`
- `GET /v1/energy/sites/:site_id/recommendations/pending`
- `GET /v1/energy/sites/:site_id/actions/blocked`
- `GET /v1/energy/sites/:site_id/explanations`
- `GET /v1/energy/sites/:site_id/replay/historical?day=YYYY-MM-DD`

## Diseno

- todos los endpoints operativos reutilizan el presupuesto calculado del sitio,
- la salida es legible por humanos y estable para automatizaciones,
- el modo sigue siendo `advisory-first`,
- el smoke test `./scripts/e2e_energy_orchestrator.sh` valida estos endpoints junto al flujo de dispatch.

## Criterio de aceptacion

Un operador puede inspeccionar el estado operativo y entender por que una accion fue aceptada, parcial o rechazada sin leer logs internos ni revisar SQL manualmente.

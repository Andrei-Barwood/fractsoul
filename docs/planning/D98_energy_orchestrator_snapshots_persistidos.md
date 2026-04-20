# D98 - Snapshots persistidos del Energy Orchestrator

Fecha: 2026-04-09

## Objetivo

Persistir cada calculo de presupuesto y cada validacion de dispatch como evidencia auditable del estado energetico observado y recomendado.

## Resultado

- se agrega la tabla `energy_budget_snapshots`,
- cada snapshot guarda capacidades nominales, efectivas, seguras y despacho disponible,
- se almacenan `constraint_flags`, el payload completo del presupuesto y el contexto upstream consumido desde `fractsoul`,
- el servicio persiste snapshots tanto desde `GET /budget` como desde `POST /dispatch/validate`.

## Campos clave

- `snapshot_id`
- `site_id`
- `source`
- `policy_mode`
- `calculated_at`
- `telemetry_observed_at`
- `ambient_celsius`
- `nominal_capacity_kw`
- `effective_capacity_kw`
- `reserved_capacity_kw`
- `safe_capacity_kw`
- `current_load_kw`
- `available_capacity_kw`
- `safe_dispatchable_kw`
- `constraint_flags`
- `snapshot_json`
- `upstream_context_json`

## Criterio de aceptacion

Cada respuesta operativa del servicio expone `snapshot_id` y deja una traza persistida suficiente para reconstruir la decision tomada o recomendada.

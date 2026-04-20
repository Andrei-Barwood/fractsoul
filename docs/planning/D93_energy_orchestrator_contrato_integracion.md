# D93 - Contrato de integracion con Fractsoul

Fecha: 2026-04-08

## Objetivo

Definir el contrato de integracion inicial entre `energy-orchestrator` y el ecosistema `fractsoul`.

## Entradas consumidas desde Fractsoul

### Eficiencia

- fuente: `GET /v1/efficiency/sites`
- uso:
  - estimar salud energetica agregada,
  - priorizar dispatch o curtailment,
  - contextualizar decisiones de margen.

### Alertas

- fuente objetivo: `alerts` / API de alertas
- uso:
  - identificar activos o racks con riesgo alto,
  - evitar recomendaciones sobre infraestructura inestable,
  - elevar severidad operacional de un dispatch rechazado.
- estado actual:
  - pendiente de un endpoint publico de lectura en `ingest-api`,
  - no integrado aun en esta fase para evitar acoplamiento directo a base de datos entre servicios.

### Anomalias

- fuente: `GET /v1/anomalies/miners/:miner_id/analyze`
- uso:
  - detectar hotspots o degradacion de hash que deban influir en el presupuesto de carga,
  - proteger zonas o grupos mineros termicamente sensibles.

### Estado de equipos

- fuente principal inicial:
  - `telemetry_latest`,
  - `GET /v1/telemetry/summary`,
  - `GET /v1/telemetry/readings`.
- uso:
  - carga actual por rack,
  - temperatura ambiente derivada,
  - disponibilidad operativa reciente.

## Eventos canonicos definidos

### `load_budget_updated`

- sujeto sugerido: `energy.load_budget_updated.v1`
- se emite cuando el motor recalcula un presupuesto de potencia por sitio.

### `curtailment_recommended`

- sujeto sugerido: `energy.curtailment_recommended.v1`
- se emite cuando el sistema recomienda reducir o limitar carga por seguridad o margen insuficiente.

### `dispatch_rejected`

- sujeto sugerido: `energy.dispatch_rejected.v1`
- se emite cuando una solicitud de aumento de carga es rechazada o recortada por restricciones internas.

## Contratos versionados

- `docs/contracts/energy_load_budget_response_v1.example.json`
- `docs/contracts/energy_dispatch_validate_request_v1.example.json`
- `docs/contracts/energy_dispatch_validate_response_v1.example.json`
- `docs/contracts/energy_event_load_budget_updated_v1.example.json`
- `docs/contracts/energy_event_curtailment_recommended_v1.example.json`
- `docs/contracts/energy_event_dispatch_rejected_v1.example.json`

## Criterio de aceptacion

Existe un contrato inicial claro para que el `energy-orchestrator` pueda consumir senales del ecosistema `fractsoul` y producir eventos operativos canonicos.

## Estado de implementacion al cierre D100

- integrado: `GET /v1/efficiency/sites`
- integrado: `GET /v1/efficiency/racks`
- integrado: `GET /v1/telemetry/summary`
- integrado: `GET /v1/telemetry/sites/:site_id/racks/:rack_id/readings`
- integrado: `GET /v1/anomalies/miners/:miner_id/analyze`
- pendiente: API publica de alertas para contexto operacional enriquecido

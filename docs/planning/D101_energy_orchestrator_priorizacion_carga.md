# D101 - Priorizacion de carga por criticidad

Fecha: 2026-04-17

## Objetivo

Introducir clases canonicas de criticidad para que el `energy-orchestrator` no trate toda la carga del campus como equivalente cuando necesita recomendar curtailment, bloquear acciones o priorizar dispatch.

## Clases implementadas

- `preferred_production`
- `normal_production`
- `sacrificable_load`
- `safety_blocked`

## Reglas operativas

- los incrementos de carga se asignan primero a `preferred_production`, luego a `normal_production` y finalmente a `sacrificable_load`,
- las reducciones recomendadas siguen el orden inverso, con `safety_blocked` como primer candidato a aislamiento,
- una carga `safety_blocked` no puede recibir dispatch positivo,
- la criticidad queda materializada en `energy_rack_profiles` y puede heredarse a simulaciones y replay.

## Entregables

- nuevos campos de criticidad y razon de criticidad en `energy_rack_profiles`,
- budget por rack enriquecido con `criticality_class`, `criticality_rank` y razon legible,
- motor de priorizacion aplicado en `ValidateDispatch`,
- recomendaciones pendientes ordenadas por criticidad.

## Criterio de aceptacion

El sistema puede explicar por que una carga fue favorecida, sacrificada o bloqueada sin depender del orden manual de los requests.

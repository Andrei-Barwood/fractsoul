# D99 - Emision de eventos canonicos del Energy Orchestrator

Fecha: 2026-04-09

## Objetivo

Emitir eventos canonicos reales para que otros componentes del ecosistema puedan reaccionar a recalculos de presupuesto, recomendaciones de curtailment y rechazos de dispatch.

## Resultado

- se agrega un `Publisher` abstrayendo la salida de eventos,
- se implementa `NoopPublisher` para desarrollo y pruebas locales sin broker,
- se implementa `NATSPublisher` con JetStream,
- el servicio asegura el stream `ENERGY` y publica sobre sujetos versionados.

## Sujetos

- `energy.load_budget_updated.v1`
- `energy.curtailment_recommended.v1`
- `energy.dispatch_rejected.v1`

## Encabezados emitidos

- `X-Event-Subject`
- `X-Request-ID`
- `X-Snapshot-ID`

## Regla operacional

La emision de eventos no ejecuta acciones sobre infraestructura electrica. Toda automatizacion aguas abajo debe respetar el principio `advisory-first` aprobado en `ADR-003`.

## Criterio de aceptacion

El servicio puede publicar eventos versionados y trazables sin romper la operacion cuando el broker no esta habilitado o cuando la publicacion falla de manera puntual.

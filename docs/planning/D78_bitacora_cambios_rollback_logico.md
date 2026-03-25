# D78 - Bitacora de cambios + rollback logico

Fecha: 2026-03-25

## Entrega
- Nueva tabla `recommendation_changes` para auditar cambios sugeridos/aplicados.
- Endpoints:
  - `POST /v1/anomalies/miners/:miner_id/changes/apply`,
  - `POST /v1/anomalies/changes/:change_id/rollback`,
  - `GET /v1/anomalies/changes`.
- Rollback logico:
  - no elimina historial,
  - marca el cambio original como `rolled_back`,
  - crea un nuevo registro `rollback` enlazado al original.

## Criterio de aceptacion
Existe trazabilidad completa de cambios operativos y posibilidad de revertirlos logicamente con auditoria.

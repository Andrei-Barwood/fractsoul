# D49 - Añadir filtros por sitio/rack/modelo

Fecha: 2026-03-24

## Resultado
Se habilitaron filtros por `site`, `rack` y `model` en API y dashboard.

## Cambios backend
- `ReadingsFilter` y `SummaryFilter` ahora soportan `model`.
- Persistencia de modelo desde tags (`asic_model`) hacia `miners.miner_model`.
- Endpoints con filtro `model`:
  - `/v1/telemetry/readings`
  - `/v1/telemetry/summary`
  - `/v1/telemetry/sites/:site_id/racks/:rack_id/readings`

## Cambios frontend
- Selectores de sitio, rack y modelo en dashboard.
- Reconsulta de KPIs/tabla al cambiar filtros.

## Archivos clave
- `backend/services/ingest-api/internal/storage/repository.go`
- `backend/services/ingest-api/internal/storage/postgres.go`
- `backend/services/ingest-api/internal/httpapi/telemetry_read_handler.go`
- `backend/services/ingest-api/internal/httpapi/dashboard/app.js`

## Criterio de salida
Operacion puede aislar vistas por ubicacion fisica y tipo de ASIC sin consultar datos crudos manualmente.

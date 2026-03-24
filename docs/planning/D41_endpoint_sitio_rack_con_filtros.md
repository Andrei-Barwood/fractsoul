# D41 - Endpoint por sitio/rack con filtros

Fecha: 2026-03-24

## Resultado
Se incorporo endpoint dedicado para lectura operativa por rack con filtros.

## Endpoint
`GET /v1/telemetry/sites/:site_id/racks/:rack_id/readings`

## Filtros soportados
- `status` (`ok|warning|critical|offline`)
- `from` (RFC3339)
- `to` (RFC3339)
- `limit` (max 500)
- `miner_id` (opcional, para acotar dentro del rack)

## Comportamiento
- Normaliza `site_id` y `rack_id` segun convencion canonica.
- Retorna contrato consistente con `request_id`, `count`, `items`.
- Usa indices por `site_id + rack_id + ts`.

## Archivos
- `backend/services/ingest-api/internal/httpapi/router.go`
- `backend/services/ingest-api/internal/httpapi/telemetry_read_handler.go`
- `backend/services/ingest-api/internal/storage/postgres.go`
- `docs/contracts/telemetry_rack_readings_response_v1.example.json`

## Criterio de salida
Consulta focalizada por rack disponible para operaciones con filtros de estado y ventana temporal.

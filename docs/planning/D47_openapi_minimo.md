# D47 - Publicar documentacion OpenAPI minima

Fecha: 2026-03-24

## Resultado
Se publico especificacion OpenAPI 3.0 minima de la API de ingesta/lectura.

## Archivo
- `docs/openapi/ingest_api_v1.yaml`

## Cobertura
- `GET /healthz`
- `POST /v1/telemetry/ingest`
- `GET /v1/telemetry/readings`
- `GET /v1/telemetry/summary`
- `GET /v1/telemetry/sites/{site_id}/racks/{rack_id}/readings`
- `GET /v1/telemetry/miners/{miner_id}/timeseries`

## Incluye
- Security scheme `ApiKeyAuth` en header.
- Esquemas minimos de request/response.
- Errores comunes (`400`, `401`, `413`, `422`).

## Criterio de salida
Consumidores internos cuentan con contrato OpenAPI versionado para integracion y documentacion tecnica.

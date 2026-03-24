# D26 - Exponer API de lectura de metricas

Fecha: 2026-03-24

## Resultado
Nuevos endpoints de lectura:
- `GET /v1/telemetry/readings`
- `GET /v1/telemetry/summary`

Ejemplos de contrato:
- `docs/contracts/telemetry_readings_response_v1.example.json`
- `docs/contracts/telemetry_summary_response_v1.example.json`

## Capacidades
- Filtros por `site_id`, `rack_id`, `miner_id`.
- Ventana temporal y limite de lecturas.
- Resumen agregado (promedios, p95 y max de temperatura, muestras).
- Validacion y normalizacion de IDs en query params.

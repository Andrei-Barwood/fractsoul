# D20 - Prototipo simulador ASIC (100 equipos)

Fecha: 2026-03-24

## Resultado
Simulador operativo implementado en:
- `backend/services/ingest-api/cmd/simulator/main.go`

## Capacidades
- Generacion de telemetria para 100 ASICs distribuidos en sitios/racks canonicos.
- Publicacion por HTTP a `POST /v1/telemetry/ingest`.
- Configuracion por flags (`duration`, `tick`, `miners`, `sites`, `concurrency`).
- Ejecucion directa via `make simulate`.

## Criterio de salida
- Producir carga sintetica sostenida y configurable para validar pipeline operativo.

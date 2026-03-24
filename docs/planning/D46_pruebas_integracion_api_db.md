# D46 - Escribir pruebas de integracion API/DB

Fecha: 2026-03-24

## Resultado
Se incorporo una suite dedicada de integracion para validar persistencia y lectura desde la API.

## Suite agregada
- Archivo:
  - `backend/services/ingest-api/tests/integration/api_db_test.go`
- Build tag:
  - `integration`
- Target Make:
  - `make integration`
- Script raiz:
  - `scripts/integration_api_db.sh`

## Casos cubiertos
1. Ingesta HTTP y persistencia efectiva en `telemetry_readings`.
2. Lectura por endpoint de rack con filtros y serie temporal por maquina.

## Compatibilidad con auth
- La suite soporta auth opcional via:
  - `INTEGRATION_API_KEY`
  - `INTEGRATION_API_KEY_HEADER`

## Criterio de salida
Existe evidencia automatizada de que la API publica/persiste y luego lee datos desde DB en flujo real.

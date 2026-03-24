# D27 - Verificacion E2E (simulador -> DB -> API)

Fecha: 2026-03-24

## Artefactos de verificacion
- Test E2E: `backend/services/ingest-api/tests/e2e/simulator_db_api_test.go`
- Script E2E operativo: `scripts/e2e_simulator_db_api.sh`

## Comportamiento esperado
1. Ejecutar simulador corto.
2. Confirmar crecimiento de filas en `telemetry_readings`.
3. Validar respuestas no vacias de `readings` y `summary`.

## Nota de ejecucion
- Si no existe `telemetry_readings`, el test marca `SKIP` y pide aplicar migraciones.

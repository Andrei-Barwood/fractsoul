# D84 - Prueba de backup/restore

Fecha: 2026-03-25

## Entrega
- Script de backup de TimescaleDB:
  - `scripts/backup_timescaledb.sh`
- Script de restore sobre DB destino:
  - `scripts/restore_timescaledb.sh`
- Prueba automatica de consistencia post-restore:
  - `scripts/test_backup_restore.sh`
- Evidencia de ejecucion:
  - `docs/operations/D84_backup_restore_validation.md`
- Validacion por conteo de tablas criticas:
  - `telemetry_readings`
  - `alerts`
  - `recommendation_changes`
  - `daily_reports`

## Criterio de aceptacion
Se puede ejecutar backup y restaurar en una base de prueba, verificando consistencia de conteos clave entre origen y restaurado.

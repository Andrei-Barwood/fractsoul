# D79 - Job de reporte diario automatico

Fecha: 2026-03-25

## Entrega
- Nuevo comando `cmd/reporter` con dos modos:
  - `once`: genera reporte de una fecha.
  - `daemon`: planifica ejecucion diaria en horario configurable.
- Persistencia de reportes en tabla `daily_reports`.
- Script operativo `scripts/generate_daily_report.sh`.
- Servicio `daily-reporter` en `docker-compose.yml` para ejecucion automatica.

## Criterio de aceptacion
Existe una ejecucion diaria automatizada que genera y persiste el reporte ejecutivo-operativo.

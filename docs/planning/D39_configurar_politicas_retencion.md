# D39 - Configurar politicas de retencion

Fecha: 2026-03-24

## Resultado
Se definieron politicas de retencion para datos crudos y agregados.

## Politicas aplicadas
- `telemetry_readings`: retencion de `30 days`.
- `telemetry_agg_minute`: retencion de `180 days`.
- `telemetry_agg_hour`: retencion de `730 days`.

## Justificacion
- Mantener granularidad fina reciente para diagnostico operativo.
- Preservar historia consolidada de mediano/largo plazo para analitica.
- Controlar crecimiento de almacenamiento sin perder tendencias.

## Archivos
- `infra/db/migrations/0002_timescale_optimizations_s2.sql`
- `infra/docker/timescaledb/init/002_s2_optimizations.sql`

## Criterio de salida
Politicas activas y reproducibles via migraciones para evitar crecimiento indefinido de tablas.

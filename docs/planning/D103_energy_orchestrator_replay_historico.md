# D103 - Replay historico de politicas energeticas

Fecha: 2026-04-17

## Objetivo

Poder tomar un dia real de telemetria del campus y comparar la operacion observada contra politicas alternativas de orquestacion sin tocar produccion real.

## Datos de entrada

- telemetria historica agregada por `5 minutes` desde `telemetry_readings`,
- perfiles nominales de `miners`,
- criticidad, bloqueos de seguridad y rampas desde `energy_rack_profiles`,
- alertas persistidas del dia desde `alerts`.

## Escenarios iniciales

- `observed`
- `priority_balanced`
- `protective_thermal`

## Metricas comparadas

- `avg_jth`
- `peak_power_kw`
- `max_temp_celsius`
- `estimated_alert_count`
- `observed_persisted_alerts`
- `energy_mwh`

## Criterio de aceptacion

El endpoint de replay devuelve al menos dos escenarios alternativos con deltas porcentuales comparables contra la operacion observada.

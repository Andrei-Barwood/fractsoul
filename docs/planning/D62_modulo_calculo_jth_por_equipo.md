# D62 - Implementar modulo de calculo J/TH por equipo

Fecha: 2026-03-24

## Entrega
- Nuevo modulo `internal/efficiency`:
  - `ComputeJTH(power_watts, hashrate_ths)`,
  - baseline por modelo (`S21`, `S19XP`, `M50`, fallback).
- Endpoint `GET /v1/efficiency/miners`.
- Persistencia de metricas agregadas por ventana desde `telemetry_readings`.

## Valor
Estandariza comparacion energetica entre equipos en una metrica unica y trazable.

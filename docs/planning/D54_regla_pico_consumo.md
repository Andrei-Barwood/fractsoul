# D54 - Regla de pico de consumo

Fecha: 2026-03-24

## Objetivo
Detectar consumo electrico anomalo por equipo, normalizado por perfil/modelo ASIC.

## Implementacion
- Regla `power_spike` en `internal/alerts/rules.go`.
- Referencia nominal por modelo (`S19XP`, `S21`, `M50`) con fallback.
- Umbrales:
  - `warning`: `power_watts >= nominal * 1.20`.
  - `critical`: `power_watts >= nominal * 1.35`.
- Se guarda detalle `nominal_power_watts` para trazabilidad de umbral.

## Validacion
- Cobertura unitaria en `internal/alerts/rules_test.go`.

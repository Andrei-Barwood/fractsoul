# D53 - Regla de sobretemperatura

Fecha: 2026-03-24

## Objetivo
Detectar eventos de temperatura fuera de banda por equipo ASIC.

## Implementacion
- Regla `overheat` en `internal/alerts/rules.go`.
- Umbrales:
  - `warning`: `temp_celsius >= 90`.
  - `critical`: `temp_celsius >= 95` o `status=critical`.
- Metadatos persistidos:
  - `metric_name=temp_celsius`,
  - valor observado,
  - umbral aplicado,
  - contexto operativo (sitio/rack/miner/modelo).

## Validacion
- Cobertura unitaria en `internal/alerts/rules_test.go`.

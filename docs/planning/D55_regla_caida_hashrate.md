# D55 - Regla de caida de hashrate

Fecha: 2026-03-24

## Objetivo
Detectar degradacion de rendimiento por caida de hashrate respecto del nominal esperado.

## Implementacion
- Regla `hashrate_drop` en `internal/alerts/rules.go`.
- Referencia nominal por modelo (`S19XP`, `S21`, `M50`) con fallback.
- Umbrales:
  - `warning`: `hashrate_ths < nominal * 0.70`.
  - `critical`: `hashrate_ths <= nominal * 0.50` o `status=critical`.
- Se guarda detalle `nominal_hashrate_ths` para auditoria.

## Validacion
- Cobertura unitaria en `internal/alerts/rules_test.go`.

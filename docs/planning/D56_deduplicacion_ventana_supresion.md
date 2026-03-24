# D56 - Deduplicacion y ventana de supresion

Fecha: 2026-03-24

## Objetivo
Reducir ruido operativo evitando alertas repetidas para la misma causa en la misma maquina.

## Implementacion
- Persistencia en tabla `alerts` (migracion `0003_alerts_engine_s2.sql`).
- Llave de deduplicacion:
  - `fingerprint = rule_id + "|" + miner_id`.
- Ventana configurable:
  - `ALERT_SUPPRESS_WINDOW` (default `10m`).
- Comportamiento:
  - primera deteccion: `status=open`, `should_notify=true`.
  - detecciones dentro de ventana: `status=suppressed`, incrementa `occurrences`, `should_notify=false`.
  - al expirar ventana: misma alerta vuelve a `open` y puede notificar.

## Validacion
- Pruebas integration/e2e verifican:
  - `occurrences >= 2` para evento repetido,
  - estado `suppressed` en segunda deteccion.

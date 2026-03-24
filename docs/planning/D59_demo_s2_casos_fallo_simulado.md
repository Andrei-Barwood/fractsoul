# D59 - Demo S2 con casos de fallo simulado

Fecha: 2026-03-24

## Objetivo
Ejecutar una demo tecnica reproducible de S2 que evidencie:
- deteccion de fallas por reglas,
- deduplicacion/supresion de ruido,
- trazabilidad de datos desde ingesta hasta consultas.

## Guion de demo
Script de referencia:
- `scripts/demo_s2_fallas_simuladas.sh`

Acciones incluidas en el script:
1. Aplicar migraciones.
2. Limpiar datos previos del escenario demo (`asic-920001..asic-920004`).
3. Inyectar eventos deterministas:
   - sobretemperatura duplicada (`overheat`) para validar supresion.
   - pico de consumo (`power_spike`).
   - caida de hashrate (`hashrate_drop`).
   - evento baseline sin alerta.
4. Esperar persistencia en DB.
5. Mostrar evidencia SQL y muestra de API de lectura.

## Evidencia de ejecucion (dry-run local)
Comando ejecutado:

```bash
./scripts/demo_s2_fallas_simuladas.sh
```

Snapshot de alertas observado:

| miner_id | rule_id | severity | status | occurrences | metric_value | threshold_value |
|---|---|---|---|---:|---:|---:|
| asic-920001 | overheat | critical | suppressed | 2 | 100.80 | 95.00 |
| asic-920002 | power_spike | critical | open | 1 | 5020.00 | 4792.50 |
| asic-920003 | hashrate_drop | critical | open | 1 | 62.00 | 100.00 |

Validacion de deduplicacion:
- `overheat` para `asic-920001` queda con `occurrences=2` y `status=suppressed`.

Muestra de API (`/v1/telemetry/readings`) validada para los 3 mineros del demo.

## Hallazgos
1. El flujo de reglas y persistencia responde en segundos bajo carga puntual.
2. La supresion controla el ruido para eventos repetidos del mismo origen.
3. Los casos de `power_spike` y `hashrate_drop` quedan activos (`open`) en primera deteccion, util para triage.

## Cierre de D59
- Demo S2 ejecutada y repetible.
- Evidencia tecnica documentada para soporte de presentacion interna.

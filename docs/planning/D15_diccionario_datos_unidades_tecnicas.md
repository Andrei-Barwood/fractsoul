# D15 - Definir diccionario de datos y unidades tecnicas

Fecha: 2026-03-24

## Resultado
Se define diccionario de datos v1 en:
- `docs/data/diccionario_datos_unidades_v1.md`

## Cobertura
- Entidades maestras (`site`, `rack`, `miner`).
- Campos de telemetria con tipo, unidad y rango tecnico.
- Campos agregados para API de lectura.
- Reglas de calidad y consistencia (`event_id + ts`, normalizacion de IDs, calculo de eficiencia).

## Criterio de salida
- Unidades tecnicas estandarizadas (TH/s, W, degC, RPM, J/TH, %).
- Campos listos para mapear directamente a modelo SQL y contratos API.

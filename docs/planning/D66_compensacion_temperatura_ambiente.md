# D66 - Compensacion por temperatura ambiente

Fecha: 2026-03-24

## Entrega
- Funcion `CompensateJTH(raw_jth, ambient_c, baseline)`.
- Soporte de `ambient_temp_c`/`ambient_c` desde tags de telemetria.
- Exposicion de `compensated_jth` en endpoints de eficiencia.

## Criterio de aceptacion
La eficiencia reportada reduce sesgo por variacion ambiental entre equipos y sitios.

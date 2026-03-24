# D67 - Revision semanal + calibracion de metricas

Fecha: 2026-03-24

## Revision
- Se valida consistencia entre `raw_jth` y `compensated_jth`.
- Se verifican bandas termicas frente a escenarios simulados de carga.

## Ajustes aplicados
- Baselines iniciales por modelo en `internal/efficiency`.
- Penalizacion ambiente parametrizada por modelo (factor por grado C).

## Proximo ajuste sugerido
- Parametrizar baselines por sitio para capturar diferencias de infraestructura.

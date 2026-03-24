# D65 - Modelo termico baseline por rango operativo

Fecha: 2026-03-24

## Entrega
- Baselines termicos por modelo en `internal/efficiency`:
  - rango `optimal`,
  - rango `elevated/warning`,
  - umbral `hotspot`.
- Clasificador `ClassifyThermalBand(temp, baseline)`.

## Criterio de aceptacion
Cada equipo se etiqueta en banda termica coherente con su modelo operativo.

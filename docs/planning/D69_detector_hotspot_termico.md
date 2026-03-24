# D69 - Detector de hotspot termico

Fecha: 2026-03-24

## Entrega
- Detector `hotspot_thermal`:
  - evalua `max_temp_celsius` vs umbral de modelo,
  - considera tendencia termica y fan RPM,
  - genera score 0-100 y evidencia numerica.

## Criterio de aceptacion
El detector marca `triggered=true` en escenarios de sobretemperatura sostenida.

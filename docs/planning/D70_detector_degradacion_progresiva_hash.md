# D70 - Detector de degradacion progresiva de hash

Fecha: 2026-03-24

## Entrega
- Detector `hash_degradation_progressive`:
  - analiza `hashrate_drop_pct`,
  - analiza tendencia lineal de hashrate por hora,
  - cruza con desviacion de `compensated_jth` contra baseline.

## Criterio de aceptacion
El detector identifica caidas graduales aun cuando no exista fallo abrupto.

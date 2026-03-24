# D74 - Revision semanal + tuning de umbrales

Fecha: 2026-03-24

## Hallazgos de tuning inicial
- Umbrales de hotspot y degradacion detectan fallas severas sin exceso de falsos positivos en escenarios sintéticos.
- `severity_score` refleja adecuadamente coexistencia de estres termico y degradacion de hash.

## Ajustes sugeridos para siguiente iteracion
1. Afinar `AmbientPenaltyPerDeg` por sitio.
2. Parametrizar umbrales de degradacion por modelo y antiguedad del equipo.
3. Añadir hysteresis temporal para reducir oscilaciones alrededor de borde de activacion.

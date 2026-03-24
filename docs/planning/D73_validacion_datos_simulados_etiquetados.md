# D73 - Validacion con datos simulados y etiquetados

Fecha: 2026-03-24

## Entrega
- Pruebas unitarias etiquetadas en `internal/anomaly/analyzer_test.go`:
  - caso hotspot termico,
  - caso degradacion progresiva de hash.
- Prueba de integracion API en `tests/integration/efficiency_anomaly_test.go`.

## Resultado
- Los detectores activan en escenarios esperados y generan recomendaciones.

# D88 Demo Final (5m) - Execution and Feedback

Fecha: 2026-03-25

## Comando ejecutado
```bash
./scripts/demo_s3_final_5min.sh
```

## Resultado de ejecucion
- Estado: `PASS`
- Duracion total: `4s` (dry-run local)
- Eficiencia (conteo de respuesta): `miner/rack/site = 1/1/1`
- Minero objetivo: `asic-920001` con `compensated_jth=18.390282968499733`
- Analisis de anomalia:
  - `severity=high (79)`
  - `hotspot=true`
  - `hash_degradation=false`
- Trazabilidad de cambios:
  - apply change id: `4d51ace7-b9f8-4dd1-9cea-f448a05c798c`
  - apply final status: `rolled_back`
  - rollback change id: `f8e75358-0065-4ee4-a92e-c72e58f1c869`
  - rollback transaction status: `applied`
- Reporte diario generado:
  - report id: `725e81dd-b84e-4969-a85e-3e637d7f5bd8`
  - date/tz: `2026-03-25` / `America/Santiago`
  - kpis: `samples=9725`, `alerts=193`, `applied=3`, `rolled_back=3`
- Alertas demo (`asic-920001..asic-920004`):
  - total/open/suppressed = `3/2/1`
- Snapshot de observabilidad validado en `/metrics`:
  - `fractsoul_http_requests_total`
  - `fractsoul_ingest_events_total`
  - `fractsoul_processor_events_total`
  - `fractsoul_alerts_evaluations_total`
  - `fractsoul_alerts_notifications_total`

## Feedback tecnico capturado
1. Fortalezas
- El guion es completamente repetible y cubre cadena completa: ingest -> alertas -> analitica -> cambios -> reporte -> observabilidad.
- El tiempo total permite demo de 5 minutos con margen para preguntas.
- El resumen por pasos facilita presentacion en vivo y captura de evidencia.

2. Fricciones observadas
- En la respuesta de rollback, el estado del registro de rollback aparece como `applied`; esto puede confundirse con el estado final del cambio original.
- La limpieza de demo S2 no era idempotente cuando existian `recommendation_changes`; se corrigio borrando primero esos registros.

3. Acciones propuestas para cierre S3
- Enfatizar en la narrativa de demo que el estado final relevante es el del cambio original (`rolled_back`).
- Mantener `scripts/demo_s3_final_5min.sh` como entrada unica de demo tecnica para D89 (documentacion ejecutiva) y D90 (retro final).

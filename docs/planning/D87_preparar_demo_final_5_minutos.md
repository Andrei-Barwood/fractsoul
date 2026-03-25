# D87 - Preparar demo final de 5 minutos

Fecha: 2026-03-25

## Entrega
- Script de demo final end-to-end:
  - `scripts/demo_s3_final_5min.sh`
- El guion integra en una sola corrida:
  - fallas sinteticas y alertas (D59),
  - eficiencia por equipo/rack/sitio (D62-D64),
  - analisis de anomalias + apply/rollback (D68-D78),
  - reporte ejecutivo-operativo diario (D79-D80),
  - snapshot de metricas de observabilidad (D82).
- Flujo disenado para ejecutarse en menos de 5 minutos en entorno Docker local activo.

## Checklist de demo (5 minutos)
1. Preflight de salud API/DB.
2. Inyeccion de escenario de fallas deterministas.
3. Consulta de eficiencia agregada.
4. Analisis de anomalia en minero objetivo.
5. Apply + rollback de recomendacion trazable.
6. Generacion de reporte diario y lectura de KPIs clave.
7. Snapshot de metricas Prometheus y resumen de alertas.

## Criterio de aceptacion
Existe un guion unico, repetible y ejecutable que demuestra de punta a punta las capacidades clave de S3 en formato de demo tecnica de 5 minutos.

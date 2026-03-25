# D80 - Plantilla de reporte ejecutivo-operativo

Fecha: 2026-03-25

## Entrega
- Plantilla markdown generada por `RenderExecutiveOperationalMarkdown` con secciones:
  - resumen ejecutivo,
  - KPIs globales,
  - desempeno por sitio,
  - alertas y anomalias,
  - cambios operativos y rollbacks,
  - top hotspots,
  - plan de accion 24h.
- El reporte se guarda:
  - en base de datos (`daily_reports.report_markdown`),
  - y opcionalmente en archivo (`REPORT_OUTPUT_DIR`).

## Criterio de aceptacion
El reporte diario presenta un formato estable, legible y reutilizable para audiencias ejecutivas y operativas.

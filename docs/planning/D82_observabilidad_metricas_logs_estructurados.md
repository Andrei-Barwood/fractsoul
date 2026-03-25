# D82 - Observabilidad (metricas, logs estructurados)

Fecha: 2026-03-25

## Entrega
- Exposicion de metricas Prometheus en `GET /metrics` sin autenticacion API key.
- Instrumentacion HTTP:
  - `fractsoul_http_inflight_requests`
  - `fractsoul_http_requests_total{method,path,status}`
  - `fractsoul_http_request_duration_seconds{method,path,status}`
- Instrumentacion de ingesta:
  - `fractsoul_ingest_events_total{result,reason}`
  - `fractsoul_ingest_payload_size_bytes`
- Instrumentacion de processor:
  - `fractsoul_processor_events_total{result}`
  - `fractsoul_processor_event_duration_seconds{result}`
- Instrumentacion de alertas:
  - `fractsoul_alerts_evaluations_total{rule_id,severity,status,notify,suppressed}`
  - `fractsoul_alerts_notifications_total{channel,status}`
  - `fractsoul_alerts_notification_duration_seconds{channel,status}`
- Logs estructurados con `service` y `component` en API y reporter.

## Validacion
- `make lint && go test ./...` en `backend/services/ingest-api`.
- Smoke test operativo de `GET /metrics` con servicios Docker activos.

## Criterio de aceptacion
El sistema permite monitorear salud y comportamiento operativo con metricas scrapeables y logs JSON enriquecidos por servicio/componente.

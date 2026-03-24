# D57 - Notificaciones webhook/email

Fecha: 2026-03-24

## Objetivo
Entregar alertas activas a canales externos para respuesta operativa.

## Implementacion
- Dispatcher asincrono en `internal/alerts/dispatcher.go`:
  - cola en memoria con workers,
  - timeout por intento,
  - reintentos con backoff,
  - registro de resultado.
- Canales:
  - `WebhookNotifier` (HTTP POST JSON).
  - `SMTPNotifier` (email plain text via SMTP).
- Persistencia de resultados en `alert_notifications`:
  - `sent/failed`,
  - intento,
  - destino,
  - error y `response_code`.
- Variables de entorno:
  - Webhook: `ALERT_WEBHOOK_ENABLED`, `ALERT_WEBHOOK_URL`, `ALERT_WEBHOOK_HEADER`, `ALERT_WEBHOOK_TOKEN`.
  - Email: `ALERT_EMAIL_ENABLED`, `ALERT_SMTP_ADDR`, `ALERT_EMAIL_FROM`, `ALERT_EMAIL_TO`, etc.

## Validacion
- Tests unitarios:
  - `webhook_notifier_test.go`
  - `smtp_notifier_test.go`

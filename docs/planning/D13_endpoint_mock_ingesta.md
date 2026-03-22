# D13 - Endpoint mock de ingesta

Fecha: 2026-03-22

## Endpoint
- `POST /v1/telemetry/ingest`

## Implementacion
Servicio en `backend/services/ingest-api`:
- API Gin con middleware de `request_id`.
- Validacion de payload via binding tags.
- Regla de timestamp futuro (max 5 minutos).
- Publicacion real a NATS (`telemetry.raw.v1`) con headers `X-Request-ID` y `X-Event-ID`.
- Respuesta `202 Accepted` con `queue_topic=telemetry.raw.v1`.
- Contrato de error estandar (`code`, `message`, `details`, `request_id`).

## Tests
- payload valido -> `202`
- payload invalido -> `400`
- timestamp fuera de rango -> `422`
- falla de publicacion NATS -> `503`

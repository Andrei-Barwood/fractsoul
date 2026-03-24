# D45 - Implementar autenticacion basica API

Fecha: 2026-03-24

## Resultado
Se implemento autenticacion basica por API key para todos los endpoints bajo `/v1`.

## Implementacion
- Middleware nuevo:
  - `backend/services/ingest-api/internal/httpapi/auth_middleware.go`
- Configuracion por entorno:
  - `API_AUTH_ENABLED` (default `false`)
  - `API_KEY_HEADER` (default `X-API-Key`)
  - `API_KEYS` (lista separada por comas)
- Integracion en runtime/router:
  - `backend/services/ingest-api/internal/app/config.go`
  - `backend/services/ingest-api/internal/app/run.go`
  - `backend/services/ingest-api/internal/httpapi/router.go`

## Comportamiento
- Si `API_AUTH_ENABLED=false`, la API opera sin autenticacion (compatibilidad local).
- Si `API_AUTH_ENABLED=true`, requiere header API key valido en `/v1/*`.
- `GET /healthz` permanece sin autenticacion.
- Respuestas de auth fallida:
  - `401 unauthorized` con `request_id`.

## Validacion
- Se agregaron tests en:
  - `backend/services/ingest-api/internal/httpapi/router_test.go`

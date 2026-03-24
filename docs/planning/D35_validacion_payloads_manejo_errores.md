# D35 - Validacion de payloads + manejo de errores

Fecha: 2026-03-24

## Resultado
El endpoint `POST /v1/telemetry/ingest` incorpora parseo estricto y mejor clasificacion de errores.

## Implementacion
- Archivo: `backend/services/ingest-api/internal/httpapi/telemetry_handler.go`
- Mejoras aplicadas:
  - valida `Content-Type: application/json`
  - limita tamano de body via `http.MaxBytesReader`
  - parseo JSON estricto (`DisallowUnknownFields`)
  - rechaza multiples objetos JSON en un mismo body
  - conserva validaciones de contrato existentes (tags Gin/validator)
  - mantiene `request_id` y esquema uniforme de errores

## Codigos de error agregados/reforzados
- `unsupported_media_type` (`415`)
- `payload_too_large` (`413`)
- `invalid_json` (`400`)
- `validation_error` (`400`)
- `timestamp_out_of_range` (`422`)

## Criterio de salida
El servicio falla de forma explicita y consistente frente a entradas invalidas, reduciendo ambiguedad operativa.

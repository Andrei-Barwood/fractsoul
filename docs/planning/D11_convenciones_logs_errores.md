# D11 - Convenciones de logs y manejo de errores

Fecha: 2026-03-22

## Resultado
- Documento base: `docs/engineering/logging_and_error_conventions.md`.
- Middleware implementado con:
  - `request_id` por request
  - logs estructurados JSON
  - severidad por codigo HTTP (`INFO/WARN/ERROR`)
- Contrato de error uniforme en respuestas HTTP.

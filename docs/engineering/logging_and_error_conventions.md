# D11 - Convenciones de Logs y Manejo de Errores

## Objetivo
Alinear todos los servicios del MVP a una convencion comun para diagnostico rapido y trazabilidad operativa.

## Convenciones de logs (v1)

Formato:
- JSON estructurado.
- Salida a `stdout` para recoleccion por Docker/Kubernetes.

Campos obligatorios por request:
- `timestamp` (inyectado por logger)
- `level`
- `msg`
- `request_id`
- `method`
- `path`
- `status`
- `duration_ms`
- `client_ip`

Campos recomendados por dominio:
- `event_id`
- `site_id`
- `rack_id`
- `miner_id`
- `status` (estado operativo de telemetria)

Niveles:
- `INFO`: requests exitosas y eventos esperados.
- `WARN`: errores de cliente (4xx), payload invalido, degradacion no critica.
- `ERROR`: errores 5xx, fallas de dependencia, timeouts no recuperados.

Reglas:
- Nunca loggear secretos ni credenciales.
- Evitar payloads completos en logs (solo campos clave y/o hashes).
- Mantener mensajes cortos y accionables.

## Convenciones de errores HTTP (v1)

Formato de respuesta de error:

```json
{
  "request_id": "uuid",
  "code": "validation_error",
  "message": "payload validation failed",
  "details": []
}
```

Codigos base:
- `validation_error` -> `400`
- `timestamp_out_of_range` -> `422`
- `unauthorized` -> `401`
- `forbidden` -> `403`
- `not_found` -> `404`
- `conflict` -> `409`
- `internal_error` -> `500`
- `dependency_unavailable` -> `503`

Reglas:
- `message` debe ser humano-legible y estable.
- `code` debe ser maquina-legible y estable para clientes.
- `details` debe incluir solo informacion util para debugging seguro.
- Toda respuesta de error debe incluir `request_id`.

## Estado de implementacion
- Endpoint `POST /v1/telemetry/ingest` ya responde con contrato de error estandar.
- Middleware HTTP agrega `request_id` y logging estructurado por request.

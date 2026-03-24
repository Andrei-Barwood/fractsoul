# D52 - Motor de reglas de alertas (esqueleto)

Fecha: 2026-03-24

## Objetivo
Implementar un motor de reglas desacoplado del pipeline de ingesta para evaluar eventos de telemetria y generar alertas operativas.

## Entregables
- Nuevo paquete `internal/alerts` con:
  - `Engine` para evaluar reglas.
  - `Rule` interface para extender reglas.
  - `Service` para orquestar evaluacion + persistencia.
- Integracion del motor en `processor/consumer` despues de `PersistTelemetry`.
- Configuracion de alertas en `internal/app/config.go`:
  - habilitacion,
  - ventana de supresion,
  - parametros de reintento y cola.

## Criterio de aceptacion
- Al ingresar un evento, el consumer evalua reglas sin bloquear la persistencia principal.
- Fallas del pipeline de alertas se registran en logs y no rompen el ack de telemetria.

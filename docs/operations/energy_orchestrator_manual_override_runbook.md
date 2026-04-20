# Energy Orchestrator - Runbook de Override Manual

## Cuándo usarlo

Aplicar override manual solo si ocurre alguna de estas condiciones:

- el motor reporta `missing_data_count` elevado y la recomendacion no es confiable,
- existe mantenimiento electrico no modelado todavia en inventario,
- un feeder o transformador acaba de degradarse y el estado aun no fue cargado,
- el sitio entro en condicion contractual especial con el utility.

## Pasos

1. Confirmar el `site_id`, el `snapshot_id` y la recomendacion objetivo.
2. Revisar `operations`, `constraints/active`, `actions/blocked` y `pilot/shadow`.
3. Si la accion es sensible, crear review y obtener primer aprobador.
4. Si corresponde, exigir segundo aprobador distinto.
5. Ejecutar el cambio fuera del sistema solo si el procedimiento local lo autoriza.
6. Registrar comentario explicando motivo, ventana y responsable.

## Reglas

- No usar override para saltarse un `rack_safety_blocked`.
- No ejecutar incremento de carga si un feeder o bus esta sin headroom.
- No usar override como sustituto permanente de inventario o sensores faltantes.

## Evidencia minima

- ticket o referencia externa,
- actor 1 y actor 2 cuando aplique,
- hora de ejecucion,
- motivo,
- horizonte de vigencia,
- validacion post-cambio.

# ADR-003 - Energy Orchestrator en modo advisory-first

- Estado: Aceptado
- Fecha: 2026-04-08
- Decision owner: Equipo Fractsoul S4

## Contexto
El `energy-orchestrator` gobernara decisiones de potencia y carga que tocan limites electricos, termicos y de disponibilidad operacional. En instalaciones de 20 MW a 100+ MW, una accion automatica equivocada puede causar:

- sobrecarga de feeder, barra o PDU,
- inestabilidad termica por densidad excesiva,
- perdida de produccion por curtailment mal aplicado,
- o una maniobra no compatible con mantenimiento en curso.

El ecosistema actual de `fractsoul` ya entrega telemetria, eficiencia, alertas y anomalias, pero no existe aun evidencia suficiente para permitir control cerrado de potencia desde el primer dia.

## Decision
El `energy-orchestrator` operara inicialmente en modo `advisory-first`.

Esto implica:

- calculara presupuestos de potencia y margen seguro,
- generara recomendaciones de curtailment o dispatch,
- validara restricciones electricas internas,
- emitira eventos canonicos y trazabilidad,
- pero no ejecutara control directo sobre activos electricos o ASICs sin aprobacion humana explicita.

## Reglas derivadas

- Ninguna recomendacion puede ignorar restricciones duras de capacidad, mantenimiento o seguridad.
- Toda accion sensible debe quedar asociada a `requested_by`, `approved_by` y evidencia de contexto.
- El sistema puede sugerir reduccion parcial o bloqueo, pero no debe forzar una accion cuando los datos de entrada sean incompletos o ambiguos.
- Si un input critico falta o esta degradado, la salida por defecto debe ser conservadora.
- La futura transicion a ejecucion supervisada requiere:
  - validacion multi-sitio,
  - RBAC por alcance,
  - interlocks OT/IT,
  - rollback verificable,
  - y aprobacion formal de operaciones e ingenieria electrica.

## Justificacion

- Reduce riesgo de automatizacion prematura.
- Mantiene a operaciones dentro del circuito de decision.
- Facilita calibracion de modelos y umbrales antes de cerrar el loop.
- Alinea el diseno con buenas practicas de seguridad electrica, OT y cambio controlado.

## Alternativas consideradas

- Automatizacion cerrada desde el dia 1:
  - Pros: respuesta rapida.
  - Contras: riesgo alto, baja explicabilidad y pobre gobernanza inicial.
- Solo reporting sin recomendaciones:
  - Pros: menor complejidad.
  - Contras: deja mucho valor operativo sin capturar.

## Consecuencias

Positivas:

- trazabilidad de decisiones,
- adopcion mas segura por operaciones,
- plataforma lista para evolucionar a supervision asistida.

Negativas:

- menor nivel de automatizacion inicial,
- necesidad de revisar y aprobar recomendaciones humanas durante el piloto.

## Criterios de validacion

- Todas las respuestas del servicio declaran `policy_mode = advisory-first`.
- Las recomendaciones bloqueadas incluyen razon explicable.
- Las validaciones de dispatch rechazan o recortan sugerencias que excedan limites internos.

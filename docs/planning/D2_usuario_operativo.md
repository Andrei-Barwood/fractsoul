# D2 - Usuario Operativo

## Perfiles objetivo

## 1) Site Manager (Jefe de sitio)
Responsabilidades:
- disponibilidad total del sitio,
- consumo energetico y eficiencia global,
- priorizacion de mantenimientos.

Dolores principales:
- no tener vista consolidada por sitio/rack/equipo,
- reaccion tardia ante degradacion de rendimiento,
- dificultad para justificar decisiones con datos.

Necesita del sistema:
- dashboard ejecutivo-operativo,
- alertas de severidad alta con contexto,
- reportes diarios y comparativos.

## 2) Operador de NOC / Operador de granja
Responsabilidades:
- monitoreo continuo,
- triage de alertas,
- escalamiento al tecnico.

Dolores principales:
- exceso de ruido y alertas duplicadas,
- falta de prioridad clara,
- poca trazabilidad de acciones realizadas.

Necesita del sistema:
- cola de alertas priorizada,
- filtros por sitio/rack/maquina,
- historial de eventos y cambios.

## 3) Tecnico de mantenimiento
Responsabilidades:
- diagnostico de fallas en campo,
- recuperacion de equipos degradados,
- validacion post-intervencion.

Dolores principales:
- poco contexto al recibir una alerta,
- datos fragmentados entre herramientas,
- dificultad para medir si la accion realmente mejoro algo.

Necesita del sistema:
- detalle por maquina (temperatura, consumo, hash, fans),
- causa probable y severidad,
- comparativa antes/despues de la intervencion.

## Usuario primario para v1
`Operador de NOC`, porque concentra reaccion en tiempo real y genera la mayor parte del valor operativo inmediato.

## Usuario decisor de compra
`Site Manager`, porque evalua impacto en ROI energetico, disponibilidad y escalabilidad.

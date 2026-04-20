# Fractsoul - Energy Orchestrator

## Tagline

Orden energetico para granjas de bitcoin a escala industrial.

## Version corta

`Fractsoul - Energy Orchestrator` es un producto para operar energia en campus mineros con una capa de decision mas clara que un dashboard de telemetria tradicional. No intenta fingir autonomia total. Observa, calcula, explica y gobierna recomendaciones antes de que una accion toque la planta.

Su centro es simple: transformar una topologia electrica compleja en una geometria legible. `site`, `substation`, `transformer`, `bus`, `feeder`, `PDU`, `rack`. Cada nivel aporta un limite, un margen y una forma. El orquestador toma esa figura completa y ayuda a decidir con menos ruido.

## Version comercial honesta

La mayoria de las plataformas para minería muestran datos. Algunas detectan anomalías. Muy pocas conectan esas señales con la realidad eléctrica del campus y con la disciplina operativa que exige un sitio serio.

`Fractsoul - Energy Orchestrator` nace justo en ese espacio.

Hoy ya permite:

- calcular presupuesto de potencia por sitio y por rack,
- respetar limites de `transformer`, `bus`, `feeder`, `PDU`, densidad termica y rampa,
- priorizar carga por criticidad,
- proyectar riesgo para las siguientes cuatro horas,
- ejecutar replay historico y piloto sombra,
- dejar trazabilidad de aprobacion, rechazo o postergacion,
- operar en modo `advisory-first` con doble confirmacion para acciones sensibles.

Eso significa algo muy concreto: el equipo de operaciones no recibe solo un numero o una alerta. Recibe una propuesta situada en la topologia real, con contexto, restricciones, explicacion y gobierno.

## Qué hace hoy, sin exagerar

### 1. Construye una vista energetica del campus

Modela inventario electrico y operativo desde `site` hasta `rack`. Eso permite leer la granja no solo como flota ASIC, sino como sistema de potencia.

### 2. Calcula headroom seguro

No habla de capacidad teorica solamente. Calcula capacidad segura considerando:

- margen operativo,
- derating por temperatura,
- estado degradado o mantenimiento,
- restricciones aguas arriba,
- limites de rampa,
- envelope termico por rack.

### 3. Explica por qué una accion es valida o no

Si una recomendacion se acepta, se rechaza o queda parcial, el sistema devuelve una explicacion legible. Eso baja friccion en operación y mejora auditoria.

### 4. Permite ensayar decisiones antes de aplicarlas

Con replay historico y piloto sombra, el equipo puede comparar escenarios alternativos sin tocar produccion.

### 5. Introduce gobierno real

No esta diseñado para disparar automatismos ciegos. Su enfoque actual es gobernado:

- `advisory-first`,
- RBAC,
- scopes por `site_id` y `rack_id`,
- aprobacion dual para acciones sensibles,
- timeline completo de eventos de review.

## Ventajas reales

- Reduce improvisacion operacional al llevar la decision a la estructura electrica real.
- Hace visible el margen seguro en vez de esconderlo detras de capacidad nominal.
- Ayuda a tomar decisiones mas suaves con politicas de rampa y no con saltos bruscos.
- Da una base seria para pasar de observabilidad a operacion supervisada.
- Deja trazabilidad util para equipos tecnicos, compliance interno y postmortems.
- Permite validar criterio antes de automatizar.

## Lo que no promete todavia

Ser honestos aqui tambien es parte del producto.

Hoy `Fractsoul - Energy Orchestrator` no pretende:

- reemplazar protecciones electricas fisicas,
- sustituir procedimientos locales de switching o LOTO,
- automatizar de forma desatendida una granja completa,
- resolver por si solo huecos de sensorizacion o inventario incompleto.

Su valor actual no esta en vender magia. Esta en volver mas serena la capa de decision.

## Para quién tiene sentido

- operadores de campus mineros que ya superaron la etapa de dashboards basicos,
- equipos que necesitan unir telemetria ASIC con restricciones de infraestructura,
- organizaciones que quieren avanzar hacia operacion supervisada sin saltar directo a automatizacion riesgosa,
- sitios donde energia, temperatura y estabilidad importan tanto como el hashrate.

## Diferencial de tono y producto

Este producto no busca parecer ruidoso, futurista ni sobreprometido. Su lenguaje visual y conceptual puede ser sobrio: pocos elementos, relaciones claras, una topologia que se entiende como figura. Casi como una geometria espiritual aplicada a la ingeniería: centros, bordes, equilibrio, simetría suficiente para orientar la acción, no para distraer.

No se trata de adornar la operación. Se trata de encontrar una forma más limpia de gobernarla.

## Estado actual

Estado honesto a abril de 2026:

- servicio funcional en `backend/services/energy-orchestrator`,
- dashboard web embebida,
- APIs de overview, presupuesto, operaciones, shadow pilot y reviews,
- validacion con tests y E2E Docker,
- PR listo para revisión sobre la rama `codex/energy-orchestrator-bootstrap`.

## Copys reutilizables

### Web hero

`Fractsoul - Energy Orchestrator` convierte la topologia electrica de una granja minera en una superficie de decision clara, gobernada y explicable.

### One-liner

Orquestacion energetica advisory-first para campus mineros de bitcoin.

### Deck corto

Una capa de decision para granjas de bitcoin que une telemetria ASIC, restricciones electricas y gobierno operativo sin vender autonomia ficticia.

# D91 - Charter tecnico del Energy Orchestrator

Fecha: 2026-04-08

## Objetivo

Iniciar el repositorio `energy-orchestrator` como capa de gobierno de potencia para campus de mineria Bitcoin de 20 MW a 100+ MW, conectando telemetria operativa con restricciones electricas y termicas del sitio.

## Problema que resuelve

- `fractsoul` observa mineros y eficiencia, pero no gobierna aun el presupuesto electrico del campus.
- Las decisiones de carga y curtailment siguen siendo mayormente manuales.
- No existe una capa unica que combine capacidad de transformadores, feeders, PDUs, margen operativo, temperatura ambiente y estado de activos.

## Supuestos de campus

- Sitios multi-rack y multi-sala con topologias modulares.
- Alimentacion principal mediante subestacion y transformadores dedicados.
- Distribucion interna por barras, feeders y PDUs hacia racks.
- Carga altamente flexible, pero no ilimitadamente flexible.
- Telemetria ASIC disponible con latencia operativa de segundos o minutos.
- Puede existir mantenimiento programado que reduzca capacidad disponible.

## Dominios del modelo

- `site`: campus minero completo y politica de reserva.
- `substation`: punto de interconexion y agrupacion electrica mayor.
- `transformer`: capacidad principal transformada al dominio interno del sitio.
- `bus`: barra o segmento principal de distribucion interna.
- `feeder`: circuito de alimentacion hacia zonas o bloques de carga.
- `rack`: unidad operativa donde vive la carga minera.
- `miner_group`: agrupacion logica de carga por modelo, prioridad o estrategia de despacho.

## Restricciones electricas que nunca pueden violarse

- No exceder capacidad segura de transformador considerando derating y margen operativo.
- No exceder capacidad segura de barra, feeder o PDU.
- No despachar carga hacia un activo en mantenimiento, inactivo o degradado fuera de politica.
- No recomendar densidad termica por rack o pasillo por encima del limite definido.
- No asumir datos faltantes como capacidad libre.
- No emitir accion automatica en conflicto con el modo `advisory-first`.

## Entregables del dia

- charter tecnico versionado,
- dominios base definidos,
- supuestos de campus explicitados,
- lista de restricciones inviolables acordada.

## Criterio de aceptacion

El repositorio parte con un lenguaje comun de dominio y con restricciones electricas explicitas que sirven de base a modelo, contratos y motor.

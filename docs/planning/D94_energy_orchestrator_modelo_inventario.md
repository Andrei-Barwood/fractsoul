# D94 - Modelo de inventario energetico del sitio

Fecha: 2026-04-08

## Objetivo

Modelar el inventario electrico minimo necesario para calcular presupuestos de potencia y restricciones internas.

## Tablas introducidas

- `energy_site_profiles`
- `energy_substations`
- `energy_transformers`
- `energy_buses`
- `energy_feeders`
- `energy_pdus`
- `energy_miner_groups`
- `energy_rack_profiles`
- `energy_maintenance_windows`

## Cobertura del modelo

- capacidad nominal por activo,
- margen operativo por activo,
- derating por temperatura ambiente,
- estado del activo (`active`, `degraded`, `maintenance`, `inactive`),
- relaciones topologicas basicas,
- ventanas de mantenimiento,
- limite termico por rack.

## Notas de modelado

- `sites`, `racks` y `miners` existentes se reutilizan como maestras.
- `energy_site_profiles` y `energy_rack_profiles` se inicializan con defaults para no dejar el sistema inutilizable en entornos existentes.
- `miner_group` queda modelado desde ahora aunque el motor inicial todavia no lo use a fondo para politicas avanzadas.

## Criterio de aceptacion

El inventario energetico queda persistible en SQL-first y permite al motor consultar capacidad, estado, asignacion y mantenimiento del sitio.

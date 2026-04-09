# D97 - Seeds operativos del Energy Orchestrator

Fecha: 2026-04-09

## Objetivo

Agregar un set de semillas operativas para que el `energy-orchestrator` pueda arrancar con un campus sintetico coherente en terminos de sitio, subestacion, transformadores, buses, feeders, PDUs, racks y grupos mineros.

## Resultado

- se agrega `infra/db/seeds/002_energy_orchestrator_demo.sql`,
- se reutilizan `site-cl-01` y `site-cl-02` definidos en la seed base,
- se modela un sitio de referencia de 20 MW y otro de 24 MW,
- se incorpora al menos una ventana de mantenimiento programada para validar restricciones temporales,
- `scripts/seed_synthetic_data.sh` aplica todas las seeds disponibles en orden.

## Cobertura funcional

- `energy_site_profiles`
- `energy_substations`
- `energy_transformers`
- `energy_buses`
- `energy_feeders`
- `energy_pdus`
- `energy_rack_profiles`
- `energy_miner_groups`
- `energy_maintenance_windows`

## Criterio de aceptacion

El entorno local puede poblar un inventario energetico inicial sin SQL manual adicional y el motor de presupuesto puede operar sobre datos consistentes con las claves foraneas del modelo base.

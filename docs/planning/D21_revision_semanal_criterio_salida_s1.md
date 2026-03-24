# D21 - Revision semanal + criterio de salida S1

Fecha: 2026-03-24

## Resumen de avance S1
- Base documental consolidada hasta D30.
- Ingesta HTTP con publicacion a NATS.
- Persistencia a Timescale/Postgres via consumer de telemetria.
- API de lectura para lecturas y resumen operativo.
- Simulador ASIC con variacion de carga y fallas sinteticas.

## Criterio de salida S1
1. Pipeline `simulador -> ingest -> NATS -> DB -> API` funcional.
2. Nomenclatura canonica aplicada (`site/rack/miner`).
3. Migraciones y seed ejecutables de forma reproducible.
4. Evidencia E2E disponible (tests/scripts) para validacion tecnica.

## Estado de criterio (entorno local actual)
- Implementacion: completa.
- Verificacion full-stack: depende de entorno con Docker y migraciones aplicadas.

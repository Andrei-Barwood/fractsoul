# ADR-001 - Stack Backend y Almacenamiento

- Estado: Aceptado
- Fecha: 2026-03-20
- Decision owner: Equipo MVP Bitcoin Mining

## Contexto
El MVP necesita:
- alta tasa de ingesta de telemetria,
- consultas operativas rapidas por ventana temporal,
- y una base que escale sin complejidad excesiva en etapa inicial.

## Decision
Seleccionar:
- Backend principal: `Go` (API de ingesta y lectura).
- Framework API: `Gin` (minimalista, estable, alto rendimiento).
- Almacenamiento principal: `PostgreSQL + TimescaleDB`.
- Migraciones: `goose` o equivalente SQL-first.

## Justificacion
- `Go` entrega buen rendimiento para I/O concurrente con footprint moderado.
- `TimescaleDB` simplifica analitica temporal sin salir del ecosistema PostgreSQL.
- SQL-first favorece trazabilidad, debugging y control de cambios.

## Alternativas consideradas
- `Python/FastAPI`:
  - Pros: velocidad de desarrollo.
  - Contras: puede requerir optimizacion temprana bajo carga de ingesta alta.
- `ClickHouse`:
  - Pros: analitica de alto volumen.
  - Contras: mayor complejidad operativa para etapa MVP.

## Consecuencias
Positivas:
- rendimiento suficiente para piloto realista,
- curva de crecimiento clara hacia produccion.

Negativas:
- menor velocidad inicial de prototipado comparado con Python.

## Criterios de validacion
- Ingesta sostenida estable para 100 ASICs simulados.
- Consultas por sitio/rack/maquina en tiempo operativo.
- Sin errores criticos de persistencia en pruebas E2E.

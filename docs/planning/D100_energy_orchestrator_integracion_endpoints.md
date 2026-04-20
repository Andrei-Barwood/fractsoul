# D100 - Integracion de endpoints con Fractsoul

Fecha: 2026-04-09

## Objetivo

Conectar `energy-orchestrator` con el `ingest-api` existente para enriquecer cada presupuesto y cada validacion de dispatch con senales operativas reales del campus.

## Endpoints integrados

- `GET /v1/efficiency/sites`
- `GET /v1/efficiency/racks`
- `GET /v1/telemetry/summary`
- `GET /v1/telemetry/sites/:site_id/racks/:rack_id/readings`
- `GET /v1/anomalies/miners/:miner_id/analyze`

## Contexto enriquecido expuesto

- eficiencia agregada por sitio,
- eficiencia por racks restringidos o sensibles,
- resumen telemetrico de ventana,
- anomalias asociadas a racks restringidos.

## Restricciones y decisiones de diseno

- la integracion es opcional y se activa con `FRACTSOUL_API_BASE_URL`,
- si el upstream falla, el presupuesto principal sigue respondiendo y el error se refleja como `warnings`,
- no se consume todavia una API publica de alertas porque `ingest-api` aun no expone ese endpoint,
- el consumo de eficiencia por rack se acota a los racks mas restringidos para evitar sobrecargar el plano de lectura.

## Criterio de aceptacion

El `energy-orchestrator` puede calcular presupuesto y validar dispatch con o sin contexto upstream, preservando disponibilidad del servicio y transparencia sobre las dependencias faltantes.

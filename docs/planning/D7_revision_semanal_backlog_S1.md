# D7 - Revision semanal y ajuste backlog S1

Fecha: 2026-03-22

## Resumen de semana
- Se completaron D1-D6 (objetivo, usuario, KPIs, arquitectura y ADRs).
- Se confirmo stack tecnico base: Go + Gin + TimescaleDB + NATS + Docker Compose.
- Se detecto dependencia critica: definir contrato de telemetria antes de persistencia real.

## Ajuste backlog S1

Prioridad alta (bloqueantes):
1. D8 - Monorepo y estructura base.
2. D9 - Entorno local con docker compose.
3. D10 - CI minimo (lint + tests).
4. D12 - Contrato telemetria v1.
5. D13 - Endpoint mock de ingesta.

Prioridad media:
1. D11 - Convenciones de logs y errores.
2. Integrar publicacion real a NATS.
3. Definir migraciones iniciales TimescaleDB.

Prioridad baja:
1. Scaffold de dashboard frontend.
2. Observabilidad extendida (Prometheus/Grafana/Loki).

## Riesgos identificados
- Riesgo de drift entre schema JSON y structs Go.
- Riesgo de deuda temprana si no se automatiza validacion de contrato.

## Mitigacion propuesta
- Mantener schema versionado en `docs/contracts/`.
- Agregar tests de contrato en sprint siguiente.

# D85 - Prueba de resiliencia (restart/failover simple)

Fecha: 2026-03-25

## Entrega
- Script de resiliencia operacional:
  - `scripts/test_resilience_restart_failover.sh`
- Flujo validado:
  - restart de `ingest-api`,
  - restart de `nats`,
  - restart de `timescaledb`,
  - verificacion de continuidad de ingesta y persistencia.
- Evidencia de ejecucion:
  - `docs/operations/D85_resilience_validation.md`

## Criterio de aceptacion
Tras reinicios controlados de componentes criticos, la API vuelve a estado saludable y la persistencia de telemetria continua.

# D85 Resilience Validation (Restart/Failover Simple)

Fecha: 2026-03-25

## Comando ejecutado
```bash
./scripts/test_resilience_restart_failover.sh
```

## Resultado
- Baseline `telemetry_readings`: `40506`
- Tras restart `ingest-api`: `40507`
- Tras restart `nats`: `40508`
- Tras restart `timescaledb`: `40509`

## Conclusion
El sistema recupera salud y mantiene continuidad de ingesta/persistencia despues de reinicios controlados de componentes criticos.

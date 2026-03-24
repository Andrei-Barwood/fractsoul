# D28 - Preparar demo tecnica interna S1

Fecha: 2026-03-24

## Checklist de demo
1. Levantar stack (`docker compose up --build`).
2. Aplicar migraciones (`./scripts/migrate_timescaledb.sh`).
3. Cargar seed (`./scripts/seed_synthetic_data.sh`).
4. Ejecutar simulador (`make simulate` o script E2E).
5. Mostrar consultas API de lectura y evidencia DB.

## Script recomendado
- `scripts/e2e_simulator_db_api.sh` como guion repetible de demo tecnica.

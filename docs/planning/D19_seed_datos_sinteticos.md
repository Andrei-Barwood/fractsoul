# D19 - Implementar script seed con datos sinteticos

Fecha: 2026-03-24

## Resultado
Seed sintetico implementado con flota base de 100 ASICs:
- `infra/db/seeds/001_synthetic_fleet.sql`
- `scripts/seed_synthetic_data.sh`

## Cobertura
- 2 sitios (`site-cl-01`, `site-cl-02`).
- 10 racks (5 por sitio).
- 100 mineros normalizados (`asic-000001`...`asic-000100`).
- Telemetria sintetica por minuto en ventana reciente de 30 minutos.

## Notas tecnicas
- IDs y nomenclatura canonica v1.
- Inserciones idempotentes con `ON CONFLICT`.
- Payload original sintetico persistido en `raw_payload`.

# D63 - Agregar eficiencia agregada por rack

Fecha: 2026-03-24

## Entrega
- Metodo de repositorio `ListRackEfficiency`.
- Endpoint `GET /v1/efficiency/racks`.
- Agregacion por ventana con:
  - miners activos por rack,
  - `avg_hashrate_ths`, `avg_power_watts`,
  - `raw_jth` y `compensated_jth`.

## Criterio de aceptacion
Consulta por `site_id/rack_id` devuelve estado energetico consolidado del rack.

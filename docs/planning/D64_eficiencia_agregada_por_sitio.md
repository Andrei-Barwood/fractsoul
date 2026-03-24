# D64 - Agregar eficiencia agregada por sitio

Fecha: 2026-03-24

## Entrega
- Metodo `ListSiteEfficiency`.
- Endpoint `GET /v1/efficiency/sites`.
- Agregacion por sitio con:
  - conteo de racks/miners,
  - eficiencia bruta y compensada,
  - ultimo timestamp observado.

## Criterio de aceptacion
Se puede comparar eficiencia global entre sitios en la misma ventana temporal.

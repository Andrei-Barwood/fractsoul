# D51 - Revisión semanal + priorización alertas

Fecha: 2026-03-24

## Estado al cierre del bloque B
- Dashboard operativo disponible en `/dashboard/`.
- Filtros por sitio/rack/modelo activos en API y UI.
- Vista por maquina implementada con detalle y serie temporal.
- Bloque A (auth + integration + OpenAPI) estable y reutilizado en dashboard.

## Backlog priorizado para alertas (bloque C)
1. Esqueleto de motor de reglas (D52).
2. Reglas base: sobretemperatura, pico de consumo, caida de hashrate (D53-D55).
3. Deduplicacion + ventana de supresion (D56).
4. Entrega de notificaciones webhook/email (D57).
5. Validacion de flujo completo de alertas (D58).

## Riesgos y acciones
- Riesgo: ruido operativo por alertas repetidas.
  - Accion: deduplicacion por llave (`miner_id + rule_id`) y TTL.
- Riesgo: alta latencia en envio externo.
  - Accion: cola asincrona con reintentos limitados.
- Riesgo: fatiga por umbrales fijos.
  - Accion: parametrizar umbrales por modelo/sitio.

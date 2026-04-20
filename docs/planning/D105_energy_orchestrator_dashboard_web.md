# D105 - Vista Operativa Web del Energy Orchestrator

## Objetivo

Exponer una consola web embebida en el servicio `energy-orchestrator` para que operaciones pueda ver:

- resumen por sitio,
- consumo actual vs permitido,
- margen restante,
- cantidad de racks sacrificables,
- restricciones activas,
- recomendaciones pendientes,
- riesgo proyectado para las siguientes cuatro horas.

## Decisiones

- La UI se sirve directamente desde `backend/services/energy-orchestrator/internal/httpapi/dashboard`.
- No se introduce pipeline de frontend adicional en esta fase; la consola es HTML/CSS/JS embebida.
- La vista principal se alimenta desde `GET /v1/energy/overview` y complementa el detalle con:
  - `GET /v1/energy/sites/:site_id/operations`
  - `GET /v1/energy/sites/:site_id/pilot/shadow`
  - `GET /v1/energy/sites/:site_id/recommendations/reviews`

## Resultado esperado

- Un operador puede abrir `/dashboard/energy/`.
- La consola lista los sitios visibles para el principal autenticado.
- Cada tarjeta resume carga actual, carga permitida, margen, riesgo inmediato y tarifa.
- El panel de detalle muestra la proyeccion horaria y el contexto operativo del sitio seleccionado.

## Criterios de salida

- La consola carga sin build externo.
- Funciona en desktop y mobile.
- Degrada bien si no hay API key o si un endpoint puntual falla.

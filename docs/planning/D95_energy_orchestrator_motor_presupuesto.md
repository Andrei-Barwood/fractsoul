# D95 - Primer motor de presupuesto de potencia

Fecha: 2026-04-08

## Objetivo

Implementar el primer motor de presupuesto de potencia por sitio y rack.

## Reglas base del motor

- tomar capacidad nominal y aplicar derating por temperatura,
- descontar margen operativo por activo,
- usar carga actual por rack como base de presupuesto,
- agregar restricciones de site y de caminos electricos cuando existan,
- producir `safe_dispatchable_kw` a nivel sitio y rack,
- exponer `policy_mode = advisory-first`.

## Salidas esperadas

- presupuesto consolidado del sitio,
- presupuestos por transformador, barra, feeder y PDU,
- presupuesto por rack,
- flags de restricciones activas,
- capacidad disponible segura para nuevas recomendaciones.

## Criterio de aceptacion

El servicio responde un presupuesto consistente aun cuando falten algunas capas del inventario, usando defaults conservadores y explicables.

# D102 - Politica de rampas suaves

Fecha: 2026-04-17

## Objetivo

Agregar limites de subida y bajada por intervalo para suavizar cambios de carga y evitar picos de corriente o perturbaciones innecesarias sobre feeders, PDUs y racks.

## Modelo implementado

- rampa por sitio: `ramp_up_kw_per_interval`, `ramp_down_kw_per_interval`, `ramp_interval_seconds`,
- override por rack: limites de subida y bajada propios,
- tratamiento especial para `safety_blocked`: no se permite rampa positiva y la reduccion puede ejecutarse hasta el nivel actual de carga.

## Reglas de aplicacion

- el presupuesto de sitio reduce `safe_dispatchable_kw` cuando la rampa activa es mas exigente que el headroom electrico,
- cada rack expone `ramp_up_limit_kw`, `ramp_down_limit_kw`, `up_ramp_remaining_kw` y `down_ramp_remaining_kw`,
- las validaciones de dispatch generan violaciones explicitas `site_ramp_up_limit`, `site_ramp_down_limit`, `rack_ramp_up_limit` y `rack_ramp_down_limit`.

## Criterio de aceptacion

Ninguna recomendacion positiva o negativa puede superar los limites de rampa configurados para el intervalo en curso.

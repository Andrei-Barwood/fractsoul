# D24 - Inyectar fallas sinteticas (overheat/hash drop)

Fecha: 2026-03-24

## Resultado
Motor de fallas integrado al simulador:
- `overheat`: incrementa temperatura y degrada hashrate.
- `hash_drop`: caida brusca de hashrate con ajuste de potencia.
- `offline` aleatorio de baja frecuencia.

## Implementacion
- Duracion de falla por ticks con probabilidad por equipo.
- Etiquetado de falla en `tags.fault`.
- Estados derivados: `warning`, `critical`, `offline`.

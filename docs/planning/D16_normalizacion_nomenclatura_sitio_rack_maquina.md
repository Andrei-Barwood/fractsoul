# D16 - Normalizar nomenclatura por sitio/rack/maquina

Fecha: 2026-03-24

## Convencion canonica v1
- `site_id`: `site-<country>-<site_num_2d>`
  - ejemplo: `site-cl-01`
- `rack_id`: `rack-<country>-<site_num_2d>-<rack_num_2d>`
  - ejemplo: `rack-cl-01-03`
- `miner_id`: `asic-<miner_num_6d>`
  - ejemplo: `asic-000042`

## Reglas de normalizacion
1. Lowercase y trim de espacios.
2. Reemplazo de `_` por `-`.
3. Zero-padding en segmentos numericos (`1 -> 01`, `42 -> 000042`).
4. Aceptar formatos legacy (`SITE-CL-1`, `rack-a1`, `asic-42`) y convertir a canonico.
5. Rechazar IDs sin componentes minimos requeridos.

## Implementacion
- `internal/telemetry/naming.go`.
- Aplicada en endpoint de ingesta antes de publicar evento.

## Beneficio
Permite consultas consistentes por sitio/rack/maquina y evita drift de identificadores entre ingest, DB y API.

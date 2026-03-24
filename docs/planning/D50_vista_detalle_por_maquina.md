# D50 - Añadir vista detalle por maquina

Fecha: 2026-03-24

## Resultado
Dashboard v0 incorpora panel de detalle por ASIC seleccionada.

## Capacidades
- Seleccion de maquina por `miner_id`.
- Lectura de metricas mas recientes:
  - status
  - modelo
  - hashrate
  - potencia
  - temperatura
  - fan rpm
- Serie temporal en ventana corta usando endpoint de timeseries (`minute`).

## Implementacion
- `backend/services/ingest-api/internal/httpapi/dashboard/app.js`
- `backend/services/ingest-api/internal/httpapi/dashboard/index.html`

## Fuente de datos
- `/v1/telemetry/readings?miner_id=...&limit=1`
- `/v1/telemetry/miners/:miner_id/timeseries`

## Criterio de salida
Operadores pueden investigar rapidamente comportamiento de un equipo puntual desde el dashboard.

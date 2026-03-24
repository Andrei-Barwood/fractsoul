# D48 - Crear dashboard v0 (4 metricas clave)

Fecha: 2026-03-24

## Resultado
Se implemento un dashboard operativo v0 embebido en la API:
- URL: `/dashboard/`
- Renderiza sin dependencias externas de build.

## 4 KPIs incluidos
1. Hashrate promedio (`avg_hashrate_ths`)
2. Potencia promedio (`avg_power_watts`)
3. Temperatura promedio (`avg_temp_celsius`)
4. Ratio de eventos criticos (`critical/offline`)

## Implementacion
- `backend/services/ingest-api/internal/httpapi/dashboard/index.html`
- `backend/services/ingest-api/internal/httpapi/dashboard/styles.css`
- `backend/services/ingest-api/internal/httpapi/dashboard/app.js`
- `backend/services/ingest-api/internal/httpapi/dashboard_assets.go`
- `backend/services/ingest-api/internal/httpapi/router.go`

## Criterio de salida
Equipo operativo puede abrir un tablero unico y visualizar estado actual del parque con actualizacion periodica.

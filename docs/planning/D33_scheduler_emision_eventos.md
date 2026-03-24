# D33 - Agregar scheduler de emision de eventos

Fecha: 2026-03-24

## Resultado
El simulador ahora soporta planificacion de emision por tick para distribuir carga y evitar rafagas artificiales.

## Implementacion
- Archivo actualizado: `backend/services/ingest-api/cmd/simulator/main.go`
- Modos de scheduler:
  - `burst`: emision inmediata por tick.
  - `staggered`: distribuye envios dentro de una ventana del tick.
- Jitter configurable por equipo para reducir sincronizacion:
  - `-schedule-jitter` (default `250ms`).
- Etiqueta `asic_model` agregada en `tags` para analisis posterior.

## Configuracion
Nuevos flags:
- `-schedule` (`burst|staggered`)
- `-schedule-jitter` (duracion)

## Criterio de salida
Capacidad de simular patrones de emision mas realistas, manteniendo control sobre concurrencia y latencia de envio.

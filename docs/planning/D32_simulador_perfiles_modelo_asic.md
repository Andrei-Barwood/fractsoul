# D32 - Mejorar simulador con perfiles por modelo ASIC

Fecha: 2026-03-24

## Resultado
Se incorporo un sistema de perfiles por modelo ASIC en el simulador para representar flotas heterogeneas.

## Implementacion
- Archivo nuevo: `backend/services/ingest-api/cmd/simulator/profiles.go`
- Perfil base por modelo:
  - `S19XP`
  - `S21`
  - `M50`
- Parametros por perfil:
  - rango de hashrate
  - rango de potencia
  - rango de temperatura
  - rango de fan RPM
  - firmware base
  - `failureBias`
- Modo mixto con distribucion ponderada:
  - 45% `S21`
  - 35% `S19XP`
  - 20% `M50`

## Configuracion
Nuevo flag:
- `-profile-mode` con opciones `mixed|s19xp|s21|m50`.

## Criterio de salida
Las metricas sinteticas reflejan diferencias tecnicas entre modelos y pueden parametrizarse por tipo de flota.

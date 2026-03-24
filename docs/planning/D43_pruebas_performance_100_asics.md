# D43 - Pruebas de performance con 100 ASICs

Fecha: 2026-03-24

## Resultado
Se incorporo un set de pruebas de performance ejecutable en entorno local con 100 ASICs.

## Componentes
- Test perf:
  - `backend/services/ingest-api/tests/perf/performance_100_asic_test.go`
- Target Make:
  - `make perf`
- Script raiz:
  - `scripts/perf_100_asic.sh`

## Cobertura de la prueba
1. Ejecuta simulador con 100 ASICs (`20s`, `tick=2s`).
2. Valida crecimiento de eventos persistidos.
3. Calcula throughput de ingesta (events/s).
4. Ejecuta carga concurrente sobre endpoints S2:
   - rack readings
   - miner timeseries
5. Reporta p95 y promedio de latencia por endpoint.

## Umbrales base actuales
- Ingest rate minimo: `>= 20 events/s`.
- p95 endpoint rack: `< 2s`.
- p95 endpoint miner timeseries: `< 2s`.

## Criterio de salida
Existe validacion reproducible de desempeño base para 100 ASICs y endpoints criticos de lectura.

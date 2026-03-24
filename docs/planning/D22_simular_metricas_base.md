# D22 - Simular metricas base (hashrate, power, temp, fans)

Fecha: 2026-03-24

## Resultado
Metricas base modeladas en el simulador:
- `hashrate_ths`
- `power_watts`
- `temp_celsius`
- `fan_rpm`
- `efficiency_jth`

## Implementacion
- Generacion por perfil de equipo con baseline por minero.
- Correlacion entre carga, potencia, temperatura y ventilacion.
- Clamps a rangos tecnicos del contrato v1.

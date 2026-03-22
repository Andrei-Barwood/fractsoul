# D3 - KPIs Base del MVP

## KPIs operativos

### 1) Hashrate efectivo
Definicion:
Capacidad real de minado observada por equipo, rack y sitio.

Formula:
`hashrate_efectivo = promedio(hashrate_reportado)`

Uso:
Detectar degradacion y validar impacto de ajustes.

### 2) Consumo de potencia
Definicion:
Potencia activa consumida por equipo/rack/sitio.

Formula:
`power_kw = promedio(power_watts) / 1000`

Uso:
Control de costo energetico y correlacion con rendimiento.

### 3) Temperatura operativa
Definicion:
Temperatura promedio y p95 por equipo/rack.

Formula:
`temp_p95 = percentil_95(temp_celsius)`

Uso:
Deteccion temprana de hotspots y riesgo termico.

### 4) Uptime de equipo
Definicion:
Porcentaje de tiempo disponible para minar en ventana dada.

Formula:
`uptime = tiempo_operativo / tiempo_total * 100`

Uso:
Cuantificar disponibilidad y downtime no planificado.

### 5) Eficiencia energetica (`J/TH`)
Definicion:
Energia necesaria para producir terahash.

Formula:
`J/TH = power_watts / hashrate_ths`

Uso:
KPI principal para optimizacion de costo/rendimiento.

## KPIs de servicio del software
- Latencia de deteccion de alerta critica: `< 120s`.
- Disponibilidad de plataforma: `> 99.5%`.
- Error rate API: `< 1%`.

## Metas iniciales del piloto
- Mejorar eficiencia energetica entre 5% y 10%.
- Reducir downtime no planificado en 15%.
- Disminuir tiempo medio de respuesta (MTTA) a alertas criticas.

## Cadencia de seguimiento
- Tiempo real: hashrate, power, temp, alertas.
- Diario: uptime, `J/TH`, incidentes por severidad.
- Semanal: tendencia de eficiencia y disponibilidad por sitio.

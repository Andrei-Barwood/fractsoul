# D1 - Objetivo del MVP, Alcance y No-Objetivos

## Objetivo del MVP
Construir un software operativo para granjas de Bitcoin mining que permita:
- monitoreo en tiempo real por sitio, rack y equipo,
- deteccion temprana de fallas,
- y optimizacion inicial de eficiencia energetica (`J/TH` y `hash/watt`).

El objetivo de negocio del MVP es demostrar mejora medible de operacion en un entorno piloto.

## Problema que resuelve
- Poca visibilidad unificada de salud y eficiencia de la flota.
- Reaccion tardia ante sobretemperatura, caidas de hash y eventos de consumo.
- Dificultad para priorizar ajustes operativos con impacto cuantificable.

## Alcance funcional (v1)
- Ingesta de telemetria de ASICs (hashrate, power, temp, fan, estado).
- Dashboard operativo por sitio/rack/maquina.
- Alertas basadas en reglas con severidad.
- Calculo de eficiencia (`J/TH`) por equipo y agregados.
- Reporte diario de baseline operativo.

## Alcance tecnico (v1)
- API backend para lectura de metricas y eventos.
- Base de datos de series temporales para telemetria.
- Retencion y agregaciones basicas para analisis operativo.
- Observabilidad minima del sistema (metricas y logs).

## No-objetivos (v1)
- No control directo de firmware ASIC en produccion.
- No optimizacion automatica cerrada (solo recomendaciones en esta fase).
- No despliegue multi-cloud enterprise desde el dia 1.
- No modelos avanzados de ML como dependencia para lanzamiento.

## Criterios de exito del MVP
- Detectar eventos criticos en menos de 2 minutos.
- Reducir downtime no planificado en al menos 15% en piloto.
- Mejorar eficiencia energetica entre 5% y 10% mediante recomendaciones.
- Entregar demo ejecutable con evidencia de KPIs.

# D60 - Retro S2 + plan detallado S3

Fecha: 2026-03-24

## Retro S2

### Lo que funciono
- Pipeline de ingesta y consumo estable (`HTTP -> JetStream -> Timescale`) con pruebas `integration/e2e/perf` en verde.
- Simulador mejorado por perfiles ASIC y scheduler, util para escenarios realistas.
- Capa de lectura madura para operacion (`summary`, `sitio/rack`, `maquina/rango` + dashboard v0).
- Motor de alertas operativo con 3 reglas base y deduplicacion por `rule_id + miner_id`.

### Lo que falta robustecer
- Estado de ciclo de vida de alertas (`ack`, `resolved`, comentarios de operador).
- Notificaciones externas en entorno productivo (webhook/email habilitados por defecto siguen en `false` local).
- Observabilidad profunda del subsistema de alertas (metricas de entrega, cola, retries).
- Gobierno de configuracion por sitio/modelo para umbrales y supresion.

### Riesgos vigentes
1. Fatiga de alertas en escenarios con multiples reglas activas simultaneas.
2. Dependencia de configuracion manual para activacion de canales de notificacion.
3. Falta de vista operativa dedicada a alertas en UI (hoy se valida por DB/logs).

## Resultado de S2 (resumen)
- Objetivo de sprint cumplido: pipeline confiable y demostrable bajo carga con trazabilidad completa.
- Evidencia tecnica:
  - `make integration` y `make e2e` en verde.
  - `make perf` en verde (100 ASICs).
  - demo de fallas simuladas repetible (`scripts/demo_s2_fallas_simuladas.sh`).

## Plan detallado S3 (propuesto)

### Bloque 1 - Operacion de alertas
1. Exponer endpoints de alertas (`list/detail/ack/resolve`) con filtros y paginacion.
2. Definir estado operacional (`open`, `suppressed`, `acked`, `resolved`) y transiciones auditables.
3. Agregar pruebas de regresion para ciclo de vida de alertas.

### Bloque 2 - Confiabilidad de notificaciones
1. Persistir cola/outbox de notificaciones para tolerancia a reinicios.
2. Agregar politicas de retry por canal con jitter exponencial y limites por destino.
3. Incluir dead-letter para notificaciones fallidas persistentes.

### Bloque 3 - Observabilidad y producto
1. Instrumentar metricas Prometheus/OTel para alert engine y dispatcher.
2. Implementar dashboard de alertas v1 (conteos por severidad, tasa de nuevos, MTTA).
3. Definir playbooks operativos y criterios de escalamiento.

## Criterio de salida para inicio S3
- Endpoints de alertas publicados y cubiertos por pruebas E2E.
- Notificaciones con confiabilidad mejorada (outbox + retry policy validada).
- Observabilidad minima activa con panel de salud del sistema de alertas.

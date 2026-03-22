# ADR-002 - Streaming, Observabilidad e Infraestructura

- Estado: Aceptado
- Fecha: 2026-03-20
- Decision owner: Equipo MVP Bitcoin Mining

## Contexto
El sistema requiere desacoplar ingesta y procesamiento, mantener trazabilidad operacional y desplegar rapido en entornos de desarrollo/piloto.

## Decision
Seleccionar:
- Streaming/event bus: `NATS JetStream`.
- Observabilidad:
  - metricas: `Prometheus`,
  - dashboards: `Grafana`,
  - logs: `Loki`,
  - trazas: `OpenTelemetry`.
- Infraestructura:
  - local/dev: `Docker Compose`,
  - piloto/prod inicial: `Kubernetes` (k3s o cloud-managed).

## Justificacion
- `NATS JetStream` ofrece simplicidad y baja latencia para eventos de telemetria.
- Stack `Prometheus + Grafana + Loki + OTel` es estandar, robusto y bien soportado.
- `Docker Compose` acelera iteracion local; `Kubernetes` facilita crecimiento posterior.

## Alternativas consideradas
- `Kafka`:
  - Pros: muy robusto para escala grande.
  - Contras: overhead operativo alto para MVP temprano.
- Solo logs sin trazas:
  - Pros: menor configuracion inicial.
  - Contras: peor diagnostico de latencia y cuellos de botella.

## Consecuencias
Positivas:
- mejor desacople y resiliencia entre componentes.
- visibilidad clara de salud de plataforma y pipelines.

Negativas:
- mayor esfuerzo inicial de configuracion observabilidad.
- necesidad de disciplina para nomenclatura de metricas/trazas.

## Criterios de validacion
- Pipeline E2E con eventos publicados/consumidos sin perdida en prueba controlada.
- Dashboard operativo con latencia, error rate y throughput visibles.
- Alarmas basicas del sistema habilitadas en entorno de prueba.

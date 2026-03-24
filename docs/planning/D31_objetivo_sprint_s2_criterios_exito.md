# D31 - Definir objetivo de sprint S2 y criterios de exito

Fecha: 2026-03-24

## Objetivo S2
Robustecer el pipeline operativo para pasar de un flujo "funcional" a un flujo "confiable" bajo carga sostenida:
- Simulacion mas realista por modelo ASIC.
- Ingesta HTTP endurecida con validacion estricta.
- Procesamiento durable con reintentos controlados y DLQ basica.

## Criterios de exito S2 (tramo D31-D37)
1. Simulador configurable por perfiles ASIC (`mixed|s19xp|s21|m50`).
2. Scheduler de emision configurable (`burst|staggered`) con jitter.
3. Ingesta publica eventos a JetStream con stream gestionado por la app.
4. Endpoint de ingesta rechaza payloads invalidos con errores consistentes.
5. Consumer aplica retry con backoff y deriva a DLQ en falla terminal.
6. Validacion tecnica E2E local (`make e2e`) en verde.

## KPI tecnico objetivo
- Disponibilidad local de pipeline: >= 99% durante corrida de simulacion.
- Error rate de ingesta por payload valido: <= 1%.
- Mensajes persistidos en DB para corrida de prueba: crecimiento neto > 0.

## Riesgos controlados en esta fase
- Perdida silenciosa por parseo laxo de JSON.
- Bloqueo operativo por fallas transitorias sin reintento.
- Falta de trazabilidad de mensajes terminales fallidos.

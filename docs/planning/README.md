# MVP Bitcoin Mining - Kickoff D1-D14

## Estado
- [x] D1: Objetivo del MVP, alcance y no-objetivos
- [x] D2: Usuario operativo
- [x] D3: KPIs base
- [x] D4: Arquitectura logica v1
- [x] D5: ADR stack backend y almacenamiento
- [x] D6: ADR streaming, observabilidad e infraestructura
- [x] D7: Revision semanal + ajuste backlog S1
- [x] D8: Crear monorepo y estructura de carpetas
- [x] D9: Levantar entorno local con docker compose (configurado)
- [x] D10: Configurar CI minimo (lint + tests)
- [x] D11: Definir convenciones de logs y manejo de errores
- [x] D12: Disenar contrato de telemetria v1 (JSON/schema)
- [x] D13: Implementar endpoint mock de ingesta
- [x] D14: Revision semanal + deuda tecnica

## Navegacion
- [D1 Objetivo y Alcance](./D1_objetivo_alcance.md)
- [D2 Usuario Operativo](./D2_usuario_operativo.md)
- [D3 KPIs Base](./D3_kpis_base.md)
- [D4 Arquitectura Logica v1](./D4_arquitectura_logica_v1.md)
- [ADR-001 Stack Backend y Almacenamiento](./ADR-001-stack-backend-almacenamiento.md)
- [ADR-002 Streaming, Observabilidad e Infraestructura](./ADR-002-streaming-observabilidad-infraestructura.md)
- [D7 Revision semanal y backlog S1](./D7_revision_semanal_backlog_S1.md)
- [D8 Monorepo y estructura](./D8_monorepo_estructura.md)
- [D9 Entorno local Docker Compose](./D9_entorno_local_docker_compose.md)
- [D10 CI minimo](./D10_ci_minimo_lint_tests.md)
- [D11 Convenciones logs/errores](./D11_convenciones_logs_errores.md)
- [D12 Contrato telemetria v1](./D12_contrato_telemetria_v1.md)
- [D13 Endpoint mock de ingesta](./D13_endpoint_mock_ingesta.md)
- [D14 Revision semanal y deuda tecnica](./D14_revision_semanal_deuda_tecnica.md)

## Siguiente ejecucion sugerida (S2)
- Implementar publicacion real en NATS JetStream en `ingest-api`.
- Construir consumidor + persistencia inicial en TimescaleDB.
- Agregar metricas Prometheus y trazas OTel en endpoint de ingesta.

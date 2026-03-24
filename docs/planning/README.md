# MVP Bitcoin Mining - Kickoff D1-D30

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
- [x] D15: Definir diccionario de datos y unidades tecnicas
- [x] D16: Normalizar nomenclatura por sitio/rack/maquina
- [x] D17: Disenar modelo logico Timescale/Postgres
- [x] D18: Crear migraciones iniciales
- [x] D19: Implementar seed con datos sinteticos
- [x] D20: Prototipo simulador ASIC (100 equipos)
- [x] D21: Revision semanal + criterio de salida S1
- [x] D22: Simular metricas base (hashrate, power, temp, fans)
- [x] D23: Anadir ruido termico y variacion de carga
- [x] D24: Inyectar fallas sinteticas (overheat/hash drop)
- [x] D25: Persistir telemetria en base de datos
- [x] D26: Exponer API de lectura de metricas
- [x] D27: Verificacion E2E (simulador -> DB -> API)
- [x] D28: Preparar demo tecnica interna S1
- [x] D29: Ejecutar demo + documentar hallazgos
- [x] D30: Retro S1 + plan detallado S2

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
- [D15 Diccionario de datos y unidades tecnicas](./D15_diccionario_datos_unidades_tecnicas.md)
- [D16 Normalizacion de nomenclatura](./D16_normalizacion_nomenclatura_sitio_rack_maquina.md)
- [D17 Modelo logico Timescale/Postgres](./D17_modelo_logico_timescale_postgres.md)
- [D18 Migraciones iniciales](./D18_migraciones_iniciales.md)
- [D19 Seed datos sinteticos](./D19_seed_datos_sinteticos.md)
- [D20 Simulador ASIC 100 equipos](./D20_prototipo_simulador_asic_100_equipos.md)
- [D21 Revision semanal y criterio salida S1](./D21_revision_semanal_criterio_salida_s1.md)
- [D22 Simular metricas base](./D22_simular_metricas_base.md)
- [D23 Ruido termico y variacion de carga](./D23_ruido_termico_variacion_carga.md)
- [D24 Inyeccion de fallas sinteticas](./D24_inyectar_fallas_sinteticas.md)
- [D25 Persistir telemetria en base de datos](./D25_persistir_telemetria_en_base_datos.md)
- [D26 Exponer API de lectura de metricas](./D26_exponer_api_lectura_metricas.md)
- [D27 Verificacion E2E simulador-DB-API](./D27_verificacion_e2e_simulador_db_api.md)
- [D28 Preparar demo tecnica interna S1](./D28_preparar_demo_tecnica_interna_s1.md)
- [D29 Ejecutar demo y documentar hallazgos](./D29_ejecutar_demo_documentar_hallazgos.md)
- [D30 Retro S1 y plan S2](./D30_retro_s1_plan_detallado_s2.md)

## Siguiente ejecucion sugerida (S2)
- Evolucionar publicacion/consumo a NATS JetStream durable (retry/backoff/DLQ).
- Instrumentar metricas Prometheus y trazas OTel en ingesta, consumer y read API.
- Implementar seguridad baseline (API keys + rate limiting por sitio).

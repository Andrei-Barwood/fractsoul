# D90 - Retro final + backlog priorizado para S4

Fecha: 2026-03-25

## Retro final del ciclo D1-D90

### Lo que funciono
- Enfoque incremental por bloques con evidencia tecnica por cada entrega.
- Scripts operativos reproducibles para E2E, demo, backup/restore, resiliencia y benchmark.
- Consistencia entre planning, operaciones y codigo versionado.
- Capacidad de demo en vivo con narrativa completa en menos de 5 minutos.

### Lo que se puede mejorar
- Reducir ambiguedad semantica en eventos de rollback (estado de transaccion vs estado de cambio objetivo).
- Aumentar automatizacion de pruebas de caos/resiliencia para escenarios no nominales.
- Formalizar SLOs operativos y alarmas por incumplimiento.
- Integrar reportes y alertas en una experiencia de dashboard unica (hoy aun distribuida).

### Decisiones de cierre
- Mantener `scripts/demo_s3_final_5min.sh` como script canonico de demostracion tecnica.
- Usar `docs/operations` como repositorio oficial de evidencias ejecutadas.
- Iniciar S4 con foco en robustez operativa y productizacion del flujo de alertas.

## Backlog priorizado S4

### P0 (alto impacto / iniciar en Tramo 1)
1. RBAC por alcance operativo (`site_id`, `rack_id`) y auditoria de acceso.
2. Configuracion centralizada de umbrales/supresion por sitio-modelo con versionado.
3. Outbox persistente para notificaciones + retries por canal.
4. Endpoint operativo de estado de alertas (`ack`, `resolved`, comentario, owner).

### P1 (impacto medio / Tramo 2)
1. Dashboard de alertas v1 con estados, SLA de atencion y tendencia diaria.
2. Politica de retencion/archivo para `daily_reports` y snapshots de benchmark.
3. Pruebas de resiliencia extendidas (reinicio en carga, desconexion de red simulada).
4. Hardening de secretos en despliegue (rotacion periodica y validacion en arranque).

### P2 (optimizacion / Tramo 3)
1. Recomendador v2 con feedback loop post-cambio y aprendizaje de efectividad.
2. Scoring de severidad calibrado por contexto de sitio/ambiente historico.
3. Exportes ejecutivos multiformato (markdown + csv + pptx pipeline).
4. Paquete de onboarding operativo para nuevos sitios piloto.

## Criterios de exito para cierre de S4
- Seguridad:
  - 100% endpoints operativos bajo auth activa y RBAC por alcance.
- Confiabilidad:
  - entrega de notificaciones >= 99% en ventanas de operacion.
- Operacion:
  - p95 alerta->notificacion < 2 minutos en carga nominal.
  - backlog de alertas abiertas >24h reducido al menos 30%.
- Producto:
  - dashboard v1 adoptado como fuente primaria por operaciones.

## Cierre final
- D89 y D90 completados.
- Ciclo D1-D90 formalmente cerrado en documentacion y backlog de continuidad.

# Energy Orchestrator - Checklist de Paso a Operacion Supervisada

## Antes del despliegue

- [ ] Migraciones `0009` aplicadas.
- [ ] Seeds de tarifas cargadas en ambiente local o de staging.
- [ ] `go test ./...` en `backend/services/energy-orchestrator` en verde.
- [ ] `./scripts/e2e_energy_orchestrator.sh` en verde.
- [ ] Dashboard `/dashboard/energy/` accesible.
- [ ] Variables `API_KEY_PRINCIPALS`, `API_KEY_SITE_SCOPES` y `API_KEY_RACK_SCOPES` definidas.

## Antes de habilitar reviews sensibles

- [ ] Validar que una recomendacion `isolate` quede en `pending_second_approval`.
- [ ] Validar que un segundo actor distinto pueda aprobarla.
- [ ] Validar que un rack fuera de scope reciba `403`.
- [ ] Validar timeline completo de eventos del review.

## Antes de usar piloto sombra como criterio operativo

- [ ] Correr al menos un dia historico por sitio.
- [ ] Revisar `missing_data_count`.
- [ ] Confirmar que los gaps de datos no invalidan las conclusiones del piloto.

## Rollback

- [ ] Si hay degradacion, volver a modo advisory puro.
- [ ] Mantener lectura y dashboard, pero pausar aprobaciones sensibles.
- [ ] Registrar la causa y el criterio de retorno.

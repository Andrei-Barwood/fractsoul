# D109 - Cierre Operativo y Paso a Operacion Supervisada

## Entregables de cierre

- consola web operativa por sitio,
- autenticacion y scopes por sitio/rack,
- reviews con doble aprobacion,
- piloto sombra,
- pruebas unitarias y E2E actualizadas,
- runbook de override manual,
- checklist de despliegue supervisado.

## Criterios minimos antes de habilitar accion supervisada

- `go test ./...` verde,
- `./scripts/e2e_energy_orchestrator.sh` verde,
- dashboard operativa contra datos seed,
- evento `awaiting_second_approval` visible para acciones sensibles,
- piloto sombra con al menos un dia representativo por sitio,
- runbook validado por operaciones.

## Politica de rollback

- si el servicio pierde integridad de datos, se vuelve a `advisory-first` puro,
- si faltan scopes o auth, no se ejecutan acciones sensibles,
- si la calidad de sensores cae por debajo del umbral operativo, las recomendaciones quedan solo en lectura,
- cualquier override manual debe quedar documentado en el runbook y en el timeline de reviews.

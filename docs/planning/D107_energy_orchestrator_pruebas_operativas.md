# D107 - Bateria de Pruebas Operativas

## Cobertura agregada

Se incorporan pruebas unitarias para cubrir:

- transformador degradado,
- feeder fuera de servicio,
- clima extremo,
- tarifa cara,
- error de sensor,
- piloto sombra con brechas de datos,
- enforcement de scopes por sitio y rack.

## Escenarios

### Transformador degradado

Verifica que el `site budget` quede limitado por la capacidad efectiva del transformador degradado.

### Feeder fuera de servicio

Verifica que un `dispatch increase` sobre un rack aguas abajo sea rechazado por `feeder_capacity_exceeded`.

### Clima extremo y tarifa cara

Verifica que la proyeccion de riesgo sume razones de derating y costo.

### Error de sensor

Verifica que el piloto sombra detecte brechas como `sensor_hashrate_error` y `missing_nominal_reference`.

### Scope enforcement

Verifica que:

- un principal vea solo sus sitios,
- un `site_id` fuera de scope sea rechazado,
- un `rack_id` fuera de scope sea rechazado.

## Validacion esperada

- `go test ./...` en `backend/services/energy-orchestrator` debe pasar en local y CI.
- `./scripts/e2e_energy_orchestrator.sh` debe validar el flujo HTTP/DB/NATS completo.

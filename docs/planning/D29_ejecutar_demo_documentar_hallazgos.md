# D29 - Ejecutar demo + documentar hallazgos

Fecha: 2026-03-24

## Hallazgos tecnicos
1. El pipeline de ingesta a NATS se valida en E2E existente.
2. La verificacion completa simulador->DB->API requiere esquema aplicado (`telemetry_readings`).
3. Se incorporo guardia explicita en E2E para evitar falsos negativos por entorno no migrado.

## Riesgos observados
- Entornos sin migraciones aplicadas pueden aparentar fallo funcional cuando es problema de setup.
- Dependencia de Docker activo para demo full-stack reproducible.

## Acciones de mitigacion
- Script E2E integral para estandarizar ejecucion.
- Documentacion de precondiciones en planning y README.

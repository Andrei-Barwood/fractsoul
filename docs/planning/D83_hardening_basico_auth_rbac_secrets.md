# D83 - Hardening basico (auth, RBAC, secrets)

Fecha: 2026-03-25

## Entrega
- RBAC por rol (`viewer`, `operator`, `admin`) sobre endpoints API.
- Configuracion de auth/RBAC via variables:
  - `API_AUTH_ENABLED`
  - `API_RBAC_ENABLED`
  - `API_KEYS`
  - `API_KEY_ROLES`
  - `API_DEFAULT_ROLE`
- Soporte de secretos via `<ENV>_FILE` para evitar credenciales en texto plano.
- Pruebas unitarias para:
  - lectura de secretos desde archivo,
  - parseo de mapas de roles,
  - enforcement de RBAC por endpoint.

## Criterio de aceptacion
Con auth+RBAC habilitado, cada endpoint aplica control por rol y las credenciales sensibles pueden cargarse desde archivos de secreto.

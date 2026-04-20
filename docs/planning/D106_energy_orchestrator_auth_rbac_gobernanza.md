# D106 - Auth, RBAC y Gobernanza de Recomendaciones

## Objetivo

Endurecer la superficie operativa del `energy-orchestrator` con:

- autenticacion por API key,
- RBAC por rol,
- scopes por `site_id` y `rack_id`,
- aprobacion dual para acciones sensibles,
- trazabilidad completa de aprobacion, rechazo o postergacion.

## Modelo de permisos

- `viewer`: solo lectura.
- `operator`: lectura y reviews operativos.
- `admin`: lectura y acciones equivalentes a `operator`.

Los scopes quedan parametrizados por:

- `API_KEY_PRINCIPALS`
- `API_KEY_SITE_SCOPES`
- `API_KEY_RACK_SCOPES`

## Regla de sensibilidad

Una recomendacion requiere doble aprobacion si cumple al menos una condicion:

- `action == isolate`
- `criticality_class == preferred_production`
- `abs(recommended_delta_kw) >= 25`

## Persistencia

Se agregan:

- `energy_recommendation_reviews`
- `energy_recommendation_review_events`
- `energy_tariff_windows`

## Estado operativo

- La primera aprobacion deja el review en `pending_second_approval`.
- La segunda aprobacion debe venir de un actor distinto.
- Rechazo o postergacion dejan el review finalizado con trazabilidad completa.

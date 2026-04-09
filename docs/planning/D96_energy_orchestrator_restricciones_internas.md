# D96 - Modulo de restricciones de red interna

Fecha: 2026-04-08

## Objetivo

Desarrollar un modulo que impida recomendaciones de dispatch por encima de limites de `feeder`, `PDU`, `bus`, `site` o densidad termica del rack.

## Reglas de evaluacion

- una solicitud de aumento de carga se valida por rack,
- se chequean limites del camino electrico ascendente,
- se chequea el margen del sitio,
- se chequea el limite termico del rack,
- se devuelve `accepted`, `partial` o `rejected`,
- toda decision incluye explicacion y magnitud aceptada.

## Resultado esperado

- endpoint de validacion de dispatch,
- diagnostico de la razon del rechazo,
- soporte a recorte parcial en modo asesorado.

## Criterio de aceptacion

El sistema evita recomendaciones electricamente inviables y responde con detalles operativos suficientes para que operaciones entienda el bloqueo.

# D108 - Piloto en Modo Sombra

## Objetivo

Comparar operacion observada vs politicas advisory sin ejecutar cambios sobre planta.

## Entrada

- un dia historico por `site_id`,
- telemetria agregada por rack,
- alertas observadas,
- perfiles de criticidad,
- rampas y restricciones del sitio.

## Salida

`GET /v1/energy/sites/:site_id/pilot/shadow?day=YYYY-MM-DD` devuelve:

- recomendaciones evaluadas,
- decisiones correctas,
- decisiones bloqueadas,
- decisiones que requeririan escalation,
- faltantes de datos,
- replay historico comparativo.

## Uso esperado

- ejecutar el piloto contra dias calientes o dias de tarifa alta,
- medir si las politicas habrian reducido alertas o pico de potencia,
- detectar sitios donde faltan sensores o referencias nominales.

## Regla operativa

El piloto no escribe cambios sobre infraestructura electrica ni sobre equipos ASIC. Solo produce evaluacion, trazabilidad y evidencia para calibracion.

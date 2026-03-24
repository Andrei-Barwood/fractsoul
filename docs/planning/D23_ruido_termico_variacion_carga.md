# D23 - Anadir ruido termico y variacion de carga

Fecha: 2026-03-24

## Resultado
Se agrega dinamica no deterministica sobre cada equipo:
- Ruido gaussiano sobre hashrate/power/temp/fan.
- Onda de carga por tick (`load_pct`) con desfase por equipo.
- Carga reportada en `tags.load_pct` para analisis posterior.

## Beneficio
Evita series sinteticas planas y aproxima comportamiento operativo realista.

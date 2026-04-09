# D92 - Matriz normativa y operativa del Energy Orchestrator

Fecha: 2026-04-08

## Objetivo

Separar restricciones duras, restricciones blandas, limites recomendados y datos de entrada obligatorios para el `energy-orchestrator`.

## Restricciones duras

| Categoria | Regla | Accion del sistema |
|---|---|---|
| Capacidad | Nunca exceder `safe_capacity_kw` de `site`, `transformer`, `bus`, `feeder`, `pdu` o `rack`. | Bloquear o recortar recomendacion. |
| Mantenimiento | No usar activos con ventana de mantenimiento activa o aprobada. | Bloquear recomendacion. |
| Estado de activo | No recomendar dispatch sobre activos `inactive` o `maintenance`. | Bloquear recomendacion. |
| Termica | No superar `thermal_density_limit_kw` del rack/pasillo. | Bloquear o recortar. |
| Datos | Si falta inventario critico del tramo afectado, asumir postura conservadora. | Rechazar por datos insuficientes. |
| Gobernanza | Toda salida inicial es `advisory-first`. | Sin control directo automatico. |

## Restricciones blandas

| Categoria | Regla | Accion del sistema |
|---|---|---|
| Reserva | Mantener `operating_reserve_pct` por sitio y activo. | Recortar recomendacion si es necesario. |
| Ambiente | Aplicar derating por temperatura ambiente. | Reducir capacidad efectiva. |
| Degradacion | Si un activo esta `degraded`, usar factor de capacidad conservador. | Reducir capacidad efectiva. |
| Calidad de datos | Si la ambientacion viene de telemetria parcial, usar fallback del sitio. | Continuar con warning. |

## Limites recomendados

| Categoria | Regla | Uso |
|---|---|---|
| Site reserve | 10% a 20% de reserva operativa. | KPI de estabilidad. |
| Transformer margin | 10% a 15% segun topologia y criticidad. | Diseno y operacion. |
| Feeder/PDU margin | 5% a 10% segun densidad y mantenibilidad. | Dispatch y curtailment. |
| Thermal density | Limite definido por rack/pasillo y validado con operacion termica. | Bloqueo termico. |

## Datos de entrada obligatorios

- `site_id`
- perfil energetico del sitio
- perfiles de rack
- timestamp de evaluacion
- carga actual por rack
- temperatura ambiente o fallback del sitio

## Datos de entrada muy recomendados

- transformadores versionados,
- buses, feeders y PDUs mapeados,
- ventanas de mantenimiento,
- clasificacion de `miner_group`,
- politica de criticidad y curtailment.

## Referencia normativa base

- `ISO/IEC 22237`
- `ISO 50001`
- `IEC 60364`
- `IEC 61439`
- `IEC 62443`
- `NFPA 70`, `70B`, `70E`

## Resultado

La matriz normativa-operativa del proyecto queda explicitada y el modo `advisory-first` se formaliza en `ADR-003`.

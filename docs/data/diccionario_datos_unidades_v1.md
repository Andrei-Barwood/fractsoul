# Diccionario de Datos y Unidades Tecnicas v1

Fecha: 2026-03-24

## Convenciones generales
- Tiempo en UTC (`TIMESTAMPTZ` / RFC3339).
- IDs operativos normalizados en lowercase con guiones.
- Unidades SI para potencia y temperatura.
- Campos numericos en `double precision` salvo RPM (`integer`).

## Entidades maestras

| Campo | Tipo | Unidad | Ejemplo | Descripcion |
|---|---|---|---|---|
| `site_id` | text | n/a | `site-cl-01` | Identificador canonico del sitio. |
| `site_name` | text | n/a | `Copiapo Norte` | Nombre legible del sitio. |
| `country_code` | char(2) | ISO-3166 alpha-2 | `CL` | Pais del sitio. |
| `timezone` | text | IANA TZ | `America/Santiago` | Zona horaria operativa del sitio. |
| `rack_id` | text | n/a | `rack-cl-01-03` | Identificador canonico del rack. |
| `miner_id` | text | n/a | `asic-000042` | Identificador canonico de maquina ASIC. |
| `miner_model` | text | n/a | `S21` | Modelo de equipo. |
| `firmware_version` | text | n/a | `braiins-2026.1` | Version firmware reportada. |

## Telemetria operativa

| Campo | Tipo | Unidad | Rango v1 | Descripcion |
|---|---|---|---|---|
| `ts` | timestamptz | UTC | n/a | Timestamp de la lectura emitida por equipo/gateway. |
| `event_id` | uuid | n/a | uuid v4 | Idempotencia del evento. |
| `hashrate_ths` | double precision | TH/s | `0..2000` | Capacidad efectiva de minado. |
| `power_watts` | double precision | W | `0..10000` | Potencia activa del equipo. |
| `temp_celsius` | double precision | degC | `-40..130` | Temperatura operativa reportada. |
| `fan_rpm` | integer | RPM | `0..30000` | Velocidad ventiladores. |
| `efficiency_jth` | double precision | J/TH | `0..1000` | Eficiencia energetica instantanea. |
| `status` | text enum | n/a | `ok/warning/critical/offline` | Estado operativo de la lectura. |
| `load_pct` | double precision | % | `0..150` | Carga relativa sintetica del equipo. |
| `tags` | jsonb | n/a | `{"pool":"mainnet"}` | Metadatos flexibles de contexto. |
| `raw_payload` | jsonb | n/a | objeto JSON | Payload original para auditoria. |
| `ingested_at` | timestamptz | UTC | n/a | Momento en que persiste en plataforma. |

## Agregados de lectura (API)

| Campo | Tipo | Unidad | Descripcion |
|---|---|---|---|
| `avg_hashrate_ths` | double precision | TH/s | Promedio de hashrate en ventana. |
| `avg_power_watts` | double precision | W | Promedio de potencia en ventana. |
| `avg_temp_celsius` | double precision | degC | Promedio de temperatura en ventana. |
| `p95_temp_celsius` | double precision | degC | Percentil 95 de temperatura. |
| `avg_fan_rpm` | double precision | RPM | Promedio de RPM ventiladores. |
| `avg_efficiency_jth` | double precision | J/TH | Promedio de eficiencia en ventana. |
| `samples` | bigint | muestras | Numero de lecturas agregadas. |

## Reglas de calidad de datos
- `event_id + ts` debe ser unico.
- `efficiency_jth` se recalcula en simulador como `power_watts / hashrate_ths` cuando aplica.
- IDs se normalizan a formato canonico antes de publicar y persistir.
- Cualquier lectura con `hashrate_ths = 0` y `power_watts > 0` se considera candidata a diagnostico de falla.

BEGIN;

INSERT INTO sites (site_id, site_name, country_code, timezone)
VALUES
  ('site-cl-01', 'Copiapo Norte', 'CL', 'America/Santiago'),
  ('site-cl-02', 'Calama Sur', 'CL', 'America/Santiago')
ON CONFLICT (site_id) DO UPDATE
SET site_name = EXCLUDED.site_name,
    country_code = EXCLUDED.country_code,
    timezone = EXCLUDED.timezone;

INSERT INTO racks (rack_id, site_id, rack_label, max_miners)
SELECT
  'rack-cl-01-' || lpad(gs::text, 2, '0'),
  'site-cl-01',
  'S1-R' || lpad(gs::text, 2, '0'),
  24
FROM generate_series(1, 5) AS gs
ON CONFLICT (rack_id) DO UPDATE
SET site_id = EXCLUDED.site_id,
    rack_label = EXCLUDED.rack_label,
    max_miners = EXCLUDED.max_miners;

INSERT INTO racks (rack_id, site_id, rack_label, max_miners)
SELECT
  'rack-cl-02-' || lpad(gs::text, 2, '0'),
  'site-cl-02',
  'S2-R' || lpad(gs::text, 2, '0'),
  24
FROM generate_series(1, 5) AS gs
ON CONFLICT (rack_id) DO UPDATE
SET site_id = EXCLUDED.site_id,
    rack_label = EXCLUDED.rack_label,
    max_miners = EXCLUDED.max_miners;

WITH fleet AS (
  SELECT
    id,
    CASE WHEN id <= 50 THEN 1 ELSE 2 END AS site_num,
    (((id - 1) % 50) / 10) + 1 AS rack_num
  FROM generate_series(1, 100) AS id
)
INSERT INTO miners (
  miner_id,
  site_id,
  rack_id,
  miner_model,
  firmware_version,
  nominal_hashrate_ths,
  nominal_power_watts,
  installed_at,
  is_active
)
SELECT
  'asic-' || lpad(id::text, 6, '0'),
  'site-cl-' || lpad(site_num::text, 2, '0'),
  'rack-cl-' || lpad(site_num::text, 2, '0') || '-' || lpad(rack_num::text, 2, '0'),
  CASE WHEN id % 2 = 0 THEN 'S21' ELSE 'S19XP' END,
  CASE WHEN id % 2 = 0 THEN 'braiins-2026.1' ELSE 'stock-2025.11' END,
  CASE WHEN id % 2 = 0 THEN 200 ELSE 140 END,
  CASE WHEN id % 2 = 0 THEN 3600 ELSE 3050 END,
  NOW() - ((id % 45) || ' days')::interval,
  TRUE
FROM fleet
ON CONFLICT (miner_id) DO UPDATE
SET site_id = EXCLUDED.site_id,
    rack_id = EXCLUDED.rack_id,
    miner_model = EXCLUDED.miner_model,
    firmware_version = EXCLUDED.firmware_version,
    nominal_hashrate_ths = EXCLUDED.nominal_hashrate_ths,
    nominal_power_watts = EXCLUDED.nominal_power_watts,
    is_active = EXCLUDED.is_active;

WITH synthetic AS (
  SELECT
    ts,
    m.site_id,
    m.rack_id,
    m.miner_id,
    m.firmware_version,
    m.nominal_hashrate_ths,
    m.nominal_power_watts,
    (0.85 + random() * 0.25) AS load_factor,
    (55 + random() * 32) AS temp,
    (4800 + floor(random() * 2400))::integer AS fan
  FROM miners m
  CROSS JOIN generate_series(
    NOW() - INTERVAL '30 minutes',
    NOW(),
    INTERVAL '1 minute'
  ) AS ts
  WHERE m.is_active = TRUE
),
materialized AS (
  SELECT
    ts,
    (
      substring(md5(miner_id || ts::text), 1, 8) || '-' ||
      substring(md5(miner_id || ts::text), 9, 4) || '-' ||
      substring(md5(miner_id || ts::text), 13, 4) || '-' ||
      substring(md5(miner_id || ts::text), 17, 4) || '-' ||
      substring(md5(miner_id || ts::text), 21, 12)
    )::uuid AS event_id,
    site_id,
    rack_id,
    miner_id,
    firmware_version,
    round((nominal_hashrate_ths * load_factor)::numeric, 3) AS hashrate_ths,
    round((nominal_power_watts * (0.88 + random() * 0.20))::numeric, 3) AS power_watts,
    round(temp::numeric, 3) AS temp_celsius,
    fan AS fan_rpm,
    round((load_factor * 100)::numeric, 3) AS load_pct
  FROM synthetic
)
INSERT INTO telemetry_readings (
  ts,
  event_id,
  site_id,
  rack_id,
  miner_id,
  firmware_version,
  hashrate_ths,
  power_watts,
  temp_celsius,
  fan_rpm,
  efficiency_jth,
  status,
  load_pct,
  tags,
  raw_payload
)
SELECT
  ts,
  event_id,
  site_id,
  rack_id,
  miner_id,
  firmware_version,
  hashrate_ths,
  power_watts,
  temp_celsius,
  fan_rpm,
  CASE WHEN hashrate_ths > 0 THEN round((power_watts / hashrate_ths)::numeric, 3) ELSE 0 END,
  CASE
    WHEN temp_celsius >= 95 OR hashrate_ths <= 0 THEN 'critical'
    WHEN temp_celsius >= 85 THEN 'warning'
    ELSE 'ok'
  END,
  load_pct,
  jsonb_build_object('source', 'seed', 'fleet_size', 100),
  jsonb_build_object(
    'metrics', jsonb_build_object(
      'hashrate_ths', hashrate_ths,
      'power_watts', power_watts,
      'temp_celsius', temp_celsius,
      'fan_rpm', fan_rpm,
      'efficiency_jth', CASE WHEN hashrate_ths > 0 THEN round((power_watts / hashrate_ths)::numeric, 3) ELSE 0 END,
      'status', CASE
        WHEN temp_celsius >= 95 OR hashrate_ths <= 0 THEN 'critical'
        WHEN temp_celsius >= 85 THEN 'warning'
        ELSE 'ok'
      END
    ),
    'tags', jsonb_build_object('source', 'seed', 'fleet_size', 100)
  )
FROM materialized
ON CONFLICT (event_id, ts) DO NOTHING;

COMMIT;

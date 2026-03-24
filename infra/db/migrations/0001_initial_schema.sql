CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS sites (
  site_id TEXT PRIMARY KEY,
  site_name TEXT NOT NULL,
  country_code CHAR(2) NOT NULL,
  timezone TEXT NOT NULL DEFAULT 'UTC',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS racks (
  rack_id TEXT PRIMARY KEY,
  site_id TEXT NOT NULL REFERENCES sites(site_id) ON DELETE CASCADE,
  rack_label TEXT,
  max_miners INTEGER NOT NULL DEFAULT 24 CHECK (max_miners > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_racks_site
  ON racks (site_id);

CREATE TABLE IF NOT EXISTS miners (
  miner_id TEXT PRIMARY KEY,
  site_id TEXT NOT NULL REFERENCES sites(site_id) ON DELETE RESTRICT,
  rack_id TEXT NOT NULL REFERENCES racks(rack_id) ON DELETE RESTRICT,
  miner_model TEXT NOT NULL DEFAULT 'unknown',
  firmware_version TEXT,
  nominal_hashrate_ths DOUBLE PRECISION NOT NULL DEFAULT 100 CHECK (nominal_hashrate_ths > 0),
  nominal_power_watts DOUBLE PRECISION NOT NULL DEFAULT 3000 CHECK (nominal_power_watts > 0),
  installed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_miners_site_rack
  ON miners (site_id, rack_id);

CREATE TABLE IF NOT EXISTS telemetry_readings (
  ts TIMESTAMPTZ NOT NULL,
  event_id UUID NOT NULL,
  site_id TEXT NOT NULL REFERENCES sites(site_id),
  rack_id TEXT NOT NULL REFERENCES racks(rack_id),
  miner_id TEXT NOT NULL REFERENCES miners(miner_id),
  firmware_version TEXT,
  hashrate_ths DOUBLE PRECISION NOT NULL CHECK (hashrate_ths >= 0 AND hashrate_ths <= 2000),
  power_watts DOUBLE PRECISION NOT NULL CHECK (power_watts >= 0 AND power_watts <= 10000),
  temp_celsius DOUBLE PRECISION NOT NULL CHECK (temp_celsius >= -40 AND temp_celsius <= 130),
  fan_rpm INTEGER NOT NULL CHECK (fan_rpm >= 0 AND fan_rpm <= 30000),
  efficiency_jth DOUBLE PRECISION NOT NULL CHECK (efficiency_jth >= 0 AND efficiency_jth <= 1000),
  status TEXT NOT NULL CHECK (status IN ('ok', 'warning', 'critical', 'offline')),
  load_pct DOUBLE PRECISION NOT NULL DEFAULT 100 CHECK (load_pct >= 0 AND load_pct <= 150),
  tags JSONB NOT NULL DEFAULT '{}'::JSONB,
  raw_payload JSONB NOT NULL,
  ingested_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

SELECT create_hypertable('telemetry_readings', by_range('ts'), if_not_exists => TRUE);

CREATE UNIQUE INDEX IF NOT EXISTS idx_telemetry_readings_event_id_ts
  ON telemetry_readings (event_id, ts);

CREATE INDEX IF NOT EXISTS idx_telemetry_readings_site_ts
  ON telemetry_readings (site_id, ts DESC);

CREATE INDEX IF NOT EXISTS idx_telemetry_readings_rack_ts
  ON telemetry_readings (rack_id, ts DESC);

CREATE INDEX IF NOT EXISTS idx_telemetry_readings_miner_ts
  ON telemetry_readings (miner_id, ts DESC);

CREATE INDEX IF NOT EXISTS idx_telemetry_readings_status_ts
  ON telemetry_readings (status, ts DESC);

CREATE OR REPLACE VIEW telemetry_latest AS
SELECT DISTINCT ON (miner_id)
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
  ingested_at
FROM telemetry_readings
ORDER BY miner_id, ts DESC;

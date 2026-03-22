CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS telemetry_raw (
  ts TIMESTAMPTZ NOT NULL,
  event_id UUID NOT NULL,
  site_id TEXT NOT NULL,
  rack_id TEXT NOT NULL,
  miner_id TEXT NOT NULL,
  firmware_version TEXT,
  metrics JSONB NOT NULL,
  tags JSONB NOT NULL DEFAULT '{}'::JSONB,
  raw_payload JSONB NOT NULL,
  ingested_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

SELECT create_hypertable('telemetry_raw', by_range('ts'), if_not_exists => TRUE);

CREATE UNIQUE INDEX IF NOT EXISTS idx_telemetry_raw_event_id_ts
  ON telemetry_raw (event_id, ts);

CREATE INDEX IF NOT EXISTS idx_telemetry_raw_site_ts
  ON telemetry_raw (site_id, ts DESC);

CREATE INDEX IF NOT EXISTS idx_telemetry_raw_rack_ts
  ON telemetry_raw (rack_id, ts DESC);

CREATE INDEX IF NOT EXISTS idx_telemetry_raw_miner_ts
  ON telemetry_raw (miner_id, ts DESC);

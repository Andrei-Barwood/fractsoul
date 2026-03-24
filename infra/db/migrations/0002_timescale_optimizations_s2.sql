-- D38: Hypertable and index optimization
SELECT set_chunk_time_interval('telemetry_readings', INTERVAL '1 day');

ALTER TABLE telemetry_readings
SET (
  timescaledb.compress,
  timescaledb.compress_orderby = 'ts DESC, event_id',
  timescaledb.compress_segmentby = 'site_id,rack_id,miner_id'
);

SELECT add_compression_policy('telemetry_readings', INTERVAL '7 days', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS idx_telemetry_readings_site_rack_ts
  ON telemetry_readings (site_id, rack_id, ts DESC);

CREATE INDEX IF NOT EXISTS idx_telemetry_readings_miner_status_ts
  ON telemetry_readings (miner_id, status, ts DESC);

CREATE INDEX IF NOT EXISTS idx_telemetry_readings_ts_brin
  ON telemetry_readings USING BRIN (ts);

-- D40: Continuous aggregates by minute/hour
CREATE MATERIALIZED VIEW IF NOT EXISTS telemetry_agg_minute
WITH (timescaledb.continuous) AS
SELECT
  time_bucket(INTERVAL '1 minute', ts) AS bucket,
  site_id,
  rack_id,
  miner_id,
  COUNT(*)::bigint AS samples,
  COALESCE(AVG(hashrate_ths), 0) AS avg_hashrate_ths,
  COALESCE(AVG(power_watts), 0) AS avg_power_watts,
  COALESCE(AVG(temp_celsius), 0) AS avg_temp_celsius,
  COALESCE(MAX(temp_celsius), 0) AS max_temp_celsius,
  COALESCE(AVG(fan_rpm), 0) AS avg_fan_rpm,
  COALESCE(AVG(efficiency_jth), 0) AS avg_efficiency_jth,
  COUNT(*) FILTER (WHERE status IN ('critical', 'offline'))::bigint AS critical_events
FROM telemetry_readings
GROUP BY bucket, site_id, rack_id, miner_id
WITH NO DATA;

CREATE MATERIALIZED VIEW IF NOT EXISTS telemetry_agg_hour
WITH (timescaledb.continuous) AS
SELECT
  time_bucket(INTERVAL '1 hour', ts) AS bucket,
  site_id,
  rack_id,
  miner_id,
  COUNT(*)::bigint AS samples,
  COALESCE(AVG(hashrate_ths), 0) AS avg_hashrate_ths,
  COALESCE(AVG(power_watts), 0) AS avg_power_watts,
  COALESCE(AVG(temp_celsius), 0) AS avg_temp_celsius,
  COALESCE(MAX(temp_celsius), 0) AS max_temp_celsius,
  COALESCE(AVG(fan_rpm), 0) AS avg_fan_rpm,
  COALESCE(AVG(efficiency_jth), 0) AS avg_efficiency_jth,
  COUNT(*) FILTER (WHERE status IN ('critical', 'offline'))::bigint AS critical_events
FROM telemetry_readings
GROUP BY bucket, site_id, rack_id, miner_id
WITH NO DATA;

CREATE INDEX IF NOT EXISTS idx_telemetry_agg_minute_miner_bucket
  ON telemetry_agg_minute (miner_id, bucket DESC);

CREATE INDEX IF NOT EXISTS idx_telemetry_agg_minute_site_rack_bucket
  ON telemetry_agg_minute (site_id, rack_id, bucket DESC);

CREATE INDEX IF NOT EXISTS idx_telemetry_agg_hour_miner_bucket
  ON telemetry_agg_hour (miner_id, bucket DESC);

CREATE INDEX IF NOT EXISTS idx_telemetry_agg_hour_site_rack_bucket
  ON telemetry_agg_hour (site_id, rack_id, bucket DESC);

SELECT add_continuous_aggregate_policy(
  'telemetry_agg_minute',
  start_offset => INTERVAL '6 hours',
  end_offset => INTERVAL '1 minute',
  schedule_interval => INTERVAL '1 minute',
  if_not_exists => TRUE
);

SELECT add_continuous_aggregate_policy(
  'telemetry_agg_hour',
  start_offset => INTERVAL '30 days',
  end_offset => INTERVAL '1 hour',
  schedule_interval => INTERVAL '15 minutes',
  if_not_exists => TRUE
);

-- D39: Retention policies
SELECT add_retention_policy('telemetry_readings', INTERVAL '30 days', if_not_exists => TRUE);
SELECT add_retention_policy('telemetry_agg_minute', INTERVAL '180 days', if_not_exists => TRUE);
SELECT add_retention_policy('telemetry_agg_hour', INTERVAL '730 days', if_not_exists => TRUE);

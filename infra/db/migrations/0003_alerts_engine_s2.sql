-- D52-D58: Alert engine tables (rules, dedupe/suppression, notifications)

CREATE TABLE IF NOT EXISTS alerts (
  alert_id TEXT PRIMARY KEY,
  rule_id TEXT NOT NULL,
  rule_name TEXT NOT NULL,
  severity TEXT NOT NULL CHECK (severity IN ('warning', 'critical')),
  status TEXT NOT NULL CHECK (status IN ('open', 'suppressed', 'resolved')),
  message TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  dedupe_key TEXT NOT NULL,
  site_id TEXT NOT NULL REFERENCES sites(site_id),
  rack_id TEXT NOT NULL REFERENCES racks(rack_id),
  miner_id TEXT NOT NULL REFERENCES miners(miner_id),
  event_id UUID NOT NULL,
  miner_model TEXT NOT NULL DEFAULT 'unknown',
  firmware_version TEXT,
  metric_name TEXT NOT NULL,
  metric_value DOUBLE PRECISION NOT NULL,
  threshold_value DOUBLE PRECISION NOT NULL,
  first_seen_at TIMESTAMPTZ NOT NULL,
  last_seen_at TIMESTAMPTZ NOT NULL,
  suppression_until TIMESTAMPTZ NOT NULL,
  occurrences INTEGER NOT NULL DEFAULT 1 CHECK (occurrences > 0),
  notify_count INTEGER NOT NULL DEFAULT 0 CHECK (notify_count >= 0),
  last_notified_at TIMESTAMPTZ,
  details JSONB NOT NULL DEFAULT '{}'::JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alerts_fingerprint_last_seen
  ON alerts (fingerprint, last_seen_at DESC);

CREATE INDEX IF NOT EXISTS idx_alerts_status_severity_last_seen
  ON alerts (status, severity, last_seen_at DESC);

CREATE INDEX IF NOT EXISTS idx_alerts_site_rack_miner_last_seen
  ON alerts (site_id, rack_id, miner_id, last_seen_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_alerts_unique_active_fingerprint
  ON alerts (fingerprint)
  WHERE status IN ('open', 'suppressed');

CREATE TABLE IF NOT EXISTS alert_notifications (
  notification_id TEXT PRIMARY KEY,
  alert_id TEXT NOT NULL REFERENCES alerts(alert_id) ON DELETE CASCADE,
  channel TEXT NOT NULL CHECK (channel IN ('webhook', 'email')),
  destination TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('sent', 'failed')),
  attempt INTEGER NOT NULL CHECK (attempt > 0),
  error_message TEXT,
  response_code INTEGER,
  payload JSONB NOT NULL DEFAULT '{}'::JSONB,
  sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alert_notifications_alert_sent_at
  ON alert_notifications (alert_id, sent_at DESC);

CREATE INDEX IF NOT EXISTS idx_alert_notifications_channel_status_sent_at
  ON alert_notifications (channel, status, sent_at DESC);

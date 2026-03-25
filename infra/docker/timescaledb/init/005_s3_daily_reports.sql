-- D79-D80: Daily executive-operational reports

CREATE TABLE IF NOT EXISTS daily_reports (
  report_id TEXT PRIMARY KEY,
  report_date DATE NOT NULL,
  timezone TEXT NOT NULL,
  window_from TIMESTAMPTZ NOT NULL,
  window_to TIMESTAMPTZ NOT NULL,
  generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  report_json JSONB NOT NULL,
  report_markdown TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_daily_reports_date_tz UNIQUE (report_date, timezone)
);

CREATE INDEX IF NOT EXISTS idx_daily_reports_generated_at
  ON daily_reports (generated_at DESC);

CREATE INDEX IF NOT EXISTS idx_daily_reports_report_date
  ON daily_reports (report_date DESC, timezone);

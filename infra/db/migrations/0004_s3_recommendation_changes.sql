-- D78: Recommendation change log with logical rollback

CREATE TABLE IF NOT EXISTS recommendation_changes (
  change_id TEXT PRIMARY KEY,
  parent_change_id TEXT REFERENCES recommendation_changes(change_id) ON DELETE SET NULL,
  superseded_by_change_id TEXT REFERENCES recommendation_changes(change_id) ON DELETE SET NULL,
  site_id TEXT NOT NULL REFERENCES sites(site_id),
  rack_id TEXT NOT NULL REFERENCES racks(rack_id),
  miner_id TEXT NOT NULL REFERENCES miners(miner_id),
  operation TEXT NOT NULL CHECK (operation IN ('apply', 'rollback')),
  status TEXT NOT NULL CHECK (status IN ('applied', 'rolled_back')),
  reason TEXT NOT NULL,
  requested_by TEXT NOT NULL DEFAULT 'system',
  summary TEXT NOT NULL,
  source_report JSONB NOT NULL DEFAULT '{}'::JSONB,
  recommendations JSONB NOT NULL DEFAULT '[]'::JSONB,
  impact_estimate JSONB NOT NULL DEFAULT '{}'::JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  rolled_back_at TIMESTAMPTZ,
  CONSTRAINT chk_recommendation_changes_parent
    CHECK (
      (operation = 'apply' AND parent_change_id IS NULL)
      OR (operation = 'rollback' AND parent_change_id IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_recommendation_changes_miner_created
  ON recommendation_changes (miner_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_recommendation_changes_status_created
  ON recommendation_changes (status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_recommendation_changes_parent
  ON recommendation_changes (parent_change_id)
  WHERE parent_change_id IS NOT NULL;

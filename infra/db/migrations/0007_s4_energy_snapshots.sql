-- D97-D100: Energy snapshots and operational seeding support

CREATE TABLE IF NOT EXISTS energy_budget_snapshots (
  snapshot_id TEXT PRIMARY KEY,
  site_id TEXT NOT NULL REFERENCES sites(site_id) ON DELETE CASCADE,
  source TEXT NOT NULL CHECK (source IN ('budget_endpoint', 'dispatch_validate', 'system')),
  policy_mode TEXT NOT NULL,
  calculated_at TIMESTAMPTZ NOT NULL,
  telemetry_observed_at TIMESTAMPTZ NULL,
  ambient_celsius DOUBLE PRECISION NOT NULL,
  nominal_capacity_kw DOUBLE PRECISION NOT NULL,
  effective_capacity_kw DOUBLE PRECISION NOT NULL,
  reserved_capacity_kw DOUBLE PRECISION NOT NULL,
  safe_capacity_kw DOUBLE PRECISION NOT NULL,
  current_load_kw DOUBLE PRECISION NOT NULL,
  available_capacity_kw DOUBLE PRECISION NOT NULL,
  safe_dispatchable_kw DOUBLE PRECISION NOT NULL,
  constraint_flags JSONB NOT NULL DEFAULT '[]'::JSONB,
  snapshot_json JSONB NOT NULL,
  upstream_context_json JSONB NOT NULL DEFAULT 'null'::JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_energy_budget_snapshots_site_calculated_at
  ON energy_budget_snapshots (site_id, calculated_at DESC);

CREATE INDEX IF NOT EXISTS idx_energy_budget_snapshots_source_created_at
  ON energy_budget_snapshots (source, created_at DESC);

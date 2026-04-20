-- D105-D109: Campus overview, tariff awareness and recommendation governance

CREATE TABLE IF NOT EXISTS energy_tariff_windows (
  window_id TEXT PRIMARY KEY,
  site_id TEXT NOT NULL REFERENCES sites(site_id) ON DELETE CASCADE,
  tariff_code TEXT NOT NULL,
  price_usd_per_mwh DOUBLE PRECISION NOT NULL CHECK (price_usd_per_mwh >= 0),
  effective_from TIMESTAMPTZ NOT NULL,
  effective_to TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_energy_tariff_window_order CHECK (effective_to > effective_from)
);

CREATE INDEX IF NOT EXISTS idx_energy_tariff_windows_site_window
  ON energy_tariff_windows (site_id, effective_from, effective_to);

CREATE TABLE IF NOT EXISTS energy_recommendation_reviews (
  review_id TEXT PRIMARY KEY,
  site_id TEXT NOT NULL REFERENCES sites(site_id) ON DELETE CASCADE,
  snapshot_id TEXT REFERENCES energy_budget_snapshots(snapshot_id) ON DELETE SET NULL,
  recommendation_id TEXT NOT NULL,
  rack_id TEXT REFERENCES racks(rack_id) ON DELETE SET NULL,
  action TEXT NOT NULL,
  criticality_class TEXT NOT NULL
    CHECK (criticality_class IN ('normal_production', 'preferred_production', 'sacrificable_load', 'safety_blocked')),
  requested_delta_kw DOUBLE PRECISION NOT NULL DEFAULT 0,
  recommended_delta_kw DOUBLE PRECISION NOT NULL DEFAULT 0,
  reason TEXT NOT NULL DEFAULT '',
  decision TEXT NOT NULL
    CHECK (decision IN ('approve', 'reject', 'postpone')),
  status TEXT NOT NULL
    CHECK (status IN ('pending_second_approval', 'approved', 'rejected', 'postponed')),
  sensitivity TEXT NOT NULL
    CHECK (sensitivity IN ('standard', 'high')),
  requires_dual_confirmation BOOLEAN NOT NULL DEFAULT FALSE,
  requested_by TEXT NOT NULL,
  requested_by_role TEXT NOT NULL
    CHECK (requested_by_role IN ('viewer', 'operator', 'admin')),
  first_approved_by TEXT,
  second_approved_by TEXT,
  rejected_by TEXT,
  postponed_by TEXT,
  postponed_until TIMESTAMPTZ,
  comment TEXT NOT NULL DEFAULT '',
  final_decision_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_energy_recommendation_review UNIQUE (site_id, recommendation_id)
);

CREATE INDEX IF NOT EXISTS idx_energy_recommendation_reviews_site_status
  ON energy_recommendation_reviews (site_id, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_energy_recommendation_reviews_site_rack
  ON energy_recommendation_reviews (site_id, rack_id);

CREATE TABLE IF NOT EXISTS energy_recommendation_review_events (
  event_id TEXT PRIMARY KEY,
  review_id TEXT NOT NULL REFERENCES energy_recommendation_reviews(review_id) ON DELETE CASCADE,
  site_id TEXT NOT NULL REFERENCES sites(site_id) ON DELETE CASCADE,
  rack_id TEXT REFERENCES racks(rack_id) ON DELETE SET NULL,
  actor_id TEXT NOT NULL,
  actor_role TEXT NOT NULL
    CHECK (actor_role IN ('viewer', 'operator', 'admin')),
  event_type TEXT NOT NULL,
  decision TEXT
    CHECK (decision IS NULL OR decision IN ('approve', 'reject', 'postpone')),
  comment TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_energy_recommendation_review_events_review
  ON energy_recommendation_review_events (review_id, created_at ASC);

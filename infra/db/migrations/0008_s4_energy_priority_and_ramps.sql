-- D101-D104: Priority classes, ramp policies and safety locks for energy orchestration

ALTER TABLE energy_site_profiles
  ADD COLUMN IF NOT EXISTS ramp_up_kw_per_interval DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (ramp_up_kw_per_interval >= 0),
  ADD COLUMN IF NOT EXISTS ramp_down_kw_per_interval DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (ramp_down_kw_per_interval >= 0),
  ADD COLUMN IF NOT EXISTS ramp_interval_seconds INTEGER NOT NULL DEFAULT 300 CHECK (ramp_interval_seconds > 0 AND ramp_interval_seconds <= 3600);

ALTER TABLE energy_miner_groups
  ADD COLUMN IF NOT EXISTS criticality_class TEXT NOT NULL DEFAULT 'normal_production'
    CHECK (criticality_class IN ('normal_production', 'preferred_production', 'sacrificable_load', 'safety_blocked'));

ALTER TABLE energy_rack_profiles
  ADD COLUMN IF NOT EXISTS criticality_class TEXT NOT NULL DEFAULT 'normal_production'
    CHECK (criticality_class IN ('normal_production', 'preferred_production', 'sacrificable_load', 'safety_blocked')),
  ADD COLUMN IF NOT EXISTS criticality_reason TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS safety_locked BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS safety_lock_reason TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS ramp_up_kw_per_interval DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (ramp_up_kw_per_interval >= 0),
  ADD COLUMN IF NOT EXISTS ramp_down_kw_per_interval DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (ramp_down_kw_per_interval >= 0);

CREATE INDEX IF NOT EXISTS idx_energy_rack_profiles_site_criticality
  ON energy_rack_profiles (site_id, criticality_class);

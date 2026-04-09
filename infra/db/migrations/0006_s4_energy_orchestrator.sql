-- D91-D96: Energy orchestrator foundations

CREATE TABLE IF NOT EXISTS energy_site_profiles (
  site_id TEXT PRIMARY KEY REFERENCES sites(site_id) ON DELETE CASCADE,
  campus_name TEXT NOT NULL DEFAULT 'Unnamed campus',
  target_capacity_mw DOUBLE PRECISION NOT NULL CHECK (target_capacity_mw > 0 AND target_capacity_mw <= 1000),
  operating_reserve_pct DOUBLE PRECISION NOT NULL DEFAULT 15 CHECK (operating_reserve_pct >= 0 AND operating_reserve_pct <= 60),
  ambient_reference_c DOUBLE PRECISION NOT NULL DEFAULT 25 CHECK (ambient_reference_c >= -40 AND ambient_reference_c <= 80),
  ambient_derate_start_c DOUBLE PRECISION NOT NULL DEFAULT 30 CHECK (ambient_derate_start_c >= -40 AND ambient_derate_start_c <= 120),
  ambient_derate_pct_per_deg DOUBLE PRECISION NOT NULL DEFAULT 0.50 CHECK (ambient_derate_pct_per_deg >= 0 AND ambient_derate_pct_per_deg <= 10),
  advisory_mode TEXT NOT NULL DEFAULT 'advisory-first' CHECK (advisory_mode IN ('advisory-first')),
  notes TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS energy_substations (
  substation_id TEXT PRIMARY KEY,
  site_id TEXT NOT NULL REFERENCES sites(site_id) ON DELETE CASCADE,
  substation_name TEXT NOT NULL,
  voltage_level_kv DOUBLE PRECISION NOT NULL CHECK (voltage_level_kv > 0),
  redundancy_mode TEXT NOT NULL DEFAULT 'n' CHECK (redundancy_mode IN ('n', 'n+1', '2n')),
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'degraded', 'maintenance', 'inactive')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_energy_substations_site
  ON energy_substations (site_id);

CREATE TABLE IF NOT EXISTS energy_transformers (
  transformer_id TEXT PRIMARY KEY,
  site_id TEXT NOT NULL REFERENCES sites(site_id) ON DELETE CASCADE,
  substation_id TEXT NOT NULL REFERENCES energy_substations(substation_id) ON DELETE CASCADE,
  transformer_name TEXT NOT NULL,
  nominal_capacity_kw DOUBLE PRECISION NOT NULL CHECK (nominal_capacity_kw > 0),
  operating_margin_pct DOUBLE PRECISION NOT NULL DEFAULT 12 CHECK (operating_margin_pct >= 0 AND operating_margin_pct <= 60),
  ambient_derate_start_c DOUBLE PRECISION NOT NULL DEFAULT 30 CHECK (ambient_derate_start_c >= -40 AND ambient_derate_start_c <= 120),
  ambient_derate_pct_per_deg DOUBLE PRECISION NOT NULL DEFAULT 0.60 CHECK (ambient_derate_pct_per_deg >= 0 AND ambient_derate_pct_per_deg <= 10),
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'degraded', 'maintenance', 'inactive')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_energy_transformers_site_substation
  ON energy_transformers (site_id, substation_id);

CREATE TABLE IF NOT EXISTS energy_buses (
  bus_id TEXT PRIMARY KEY,
  site_id TEXT NOT NULL REFERENCES sites(site_id) ON DELETE CASCADE,
  substation_id TEXT NOT NULL REFERENCES energy_substations(substation_id) ON DELETE CASCADE,
  transformer_id TEXT REFERENCES energy_transformers(transformer_id) ON DELETE SET NULL,
  bus_name TEXT NOT NULL,
  nominal_capacity_kw DOUBLE PRECISION NOT NULL CHECK (nominal_capacity_kw > 0),
  operating_margin_pct DOUBLE PRECISION NOT NULL DEFAULT 10 CHECK (operating_margin_pct >= 0 AND operating_margin_pct <= 60),
  ambient_derate_start_c DOUBLE PRECISION NOT NULL DEFAULT 35 CHECK (ambient_derate_start_c >= -40 AND ambient_derate_start_c <= 120),
  ambient_derate_pct_per_deg DOUBLE PRECISION NOT NULL DEFAULT 0.35 CHECK (ambient_derate_pct_per_deg >= 0 AND ambient_derate_pct_per_deg <= 10),
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'degraded', 'maintenance', 'inactive')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_energy_buses_site
  ON energy_buses (site_id);

CREATE INDEX IF NOT EXISTS idx_energy_buses_transformer
  ON energy_buses (transformer_id);

CREATE TABLE IF NOT EXISTS energy_feeders (
  feeder_id TEXT PRIMARY KEY,
  site_id TEXT NOT NULL REFERENCES sites(site_id) ON DELETE CASCADE,
  bus_id TEXT NOT NULL REFERENCES energy_buses(bus_id) ON DELETE CASCADE,
  feeder_name TEXT NOT NULL,
  nominal_capacity_kw DOUBLE PRECISION NOT NULL CHECK (nominal_capacity_kw > 0),
  operating_margin_pct DOUBLE PRECISION NOT NULL DEFAULT 8 CHECK (operating_margin_pct >= 0 AND operating_margin_pct <= 60),
  ambient_derate_start_c DOUBLE PRECISION NOT NULL DEFAULT 35 CHECK (ambient_derate_start_c >= -40 AND ambient_derate_start_c <= 120),
  ambient_derate_pct_per_deg DOUBLE PRECISION NOT NULL DEFAULT 0.25 CHECK (ambient_derate_pct_per_deg >= 0 AND ambient_derate_pct_per_deg <= 10),
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'degraded', 'maintenance', 'inactive')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_energy_feeders_site_bus
  ON energy_feeders (site_id, bus_id);

CREATE TABLE IF NOT EXISTS energy_pdus (
  pdu_id TEXT PRIMARY KEY,
  site_id TEXT NOT NULL REFERENCES sites(site_id) ON DELETE CASCADE,
  feeder_id TEXT NOT NULL REFERENCES energy_feeders(feeder_id) ON DELETE CASCADE,
  pdu_name TEXT NOT NULL,
  nominal_capacity_kw DOUBLE PRECISION NOT NULL CHECK (nominal_capacity_kw > 0),
  operating_margin_pct DOUBLE PRECISION NOT NULL DEFAULT 5 CHECK (operating_margin_pct >= 0 AND operating_margin_pct <= 60),
  ambient_derate_start_c DOUBLE PRECISION NOT NULL DEFAULT 40 CHECK (ambient_derate_start_c >= -40 AND ambient_derate_start_c <= 120),
  ambient_derate_pct_per_deg DOUBLE PRECISION NOT NULL DEFAULT 0.20 CHECK (ambient_derate_pct_per_deg >= 0 AND ambient_derate_pct_per_deg <= 10),
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'degraded', 'maintenance', 'inactive')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_energy_pdus_site_feeder
  ON energy_pdus (site_id, feeder_id);

CREATE TABLE IF NOT EXISTS energy_miner_groups (
  miner_group_id TEXT PRIMARY KEY,
  site_id TEXT NOT NULL REFERENCES sites(site_id) ON DELETE CASCADE,
  rack_id TEXT REFERENCES racks(rack_id) ON DELETE SET NULL,
  group_name TEXT NOT NULL,
  miner_model TEXT NOT NULL DEFAULT 'unknown',
  priority_class TEXT NOT NULL DEFAULT 'standard' CHECK (priority_class IN ('critical', 'preferred', 'standard', 'sacrificable')),
  target_miners INTEGER NOT NULL DEFAULT 0 CHECK (target_miners >= 0),
  nominal_group_kw DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (nominal_group_kw >= 0),
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'degraded', 'maintenance', 'inactive')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_energy_miner_groups_site
  ON energy_miner_groups (site_id);

CREATE TABLE IF NOT EXISTS energy_rack_profiles (
  rack_id TEXT PRIMARY KEY REFERENCES racks(rack_id) ON DELETE CASCADE,
  site_id TEXT NOT NULL REFERENCES sites(site_id) ON DELETE CASCADE,
  bus_id TEXT REFERENCES energy_buses(bus_id) ON DELETE SET NULL,
  feeder_id TEXT REFERENCES energy_feeders(feeder_id) ON DELETE SET NULL,
  pdu_id TEXT REFERENCES energy_pdus(pdu_id) ON DELETE SET NULL,
  nominal_capacity_kw DOUBLE PRECISION NOT NULL DEFAULT 120 CHECK (nominal_capacity_kw > 0),
  operating_margin_pct DOUBLE PRECISION NOT NULL DEFAULT 10 CHECK (operating_margin_pct >= 0 AND operating_margin_pct <= 60),
  thermal_density_limit_kw DOUBLE PRECISION NOT NULL DEFAULT 140 CHECK (thermal_density_limit_kw > 0),
  aisle_zone TEXT NOT NULL DEFAULT 'unknown',
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'degraded', 'maintenance', 'inactive')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_energy_rack_profiles_site
  ON energy_rack_profiles (site_id);

CREATE TABLE IF NOT EXISTS energy_maintenance_windows (
  window_id TEXT PRIMARY KEY,
  site_id TEXT NOT NULL REFERENCES sites(site_id) ON DELETE CASCADE,
  asset_type TEXT NOT NULL CHECK (asset_type IN ('site', 'substation', 'transformer', 'bus', 'feeder', 'pdu', 'rack', 'miner_group')),
  asset_id TEXT NOT NULL,
  window_from TIMESTAMPTZ NOT NULL,
  window_to TIMESTAMPTZ NOT NULL,
  status TEXT NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled', 'approved', 'active', 'completed', 'cancelled')),
  reason TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_energy_maintenance_window_order CHECK (window_to > window_from)
);

CREATE INDEX IF NOT EXISTS idx_energy_maintenance_windows_site_asset
  ON energy_maintenance_windows (site_id, asset_type, asset_id, window_from, window_to);

INSERT INTO energy_site_profiles (
  site_id,
  campus_name,
  target_capacity_mw,
  operating_reserve_pct,
  ambient_reference_c,
  ambient_derate_start_c,
  ambient_derate_pct_per_deg,
  advisory_mode
)
SELECT
  site_id,
  site_name,
  20,
  15,
  25,
  30,
  0.50,
  'advisory-first'
FROM sites
ON CONFLICT (site_id) DO NOTHING;

INSERT INTO energy_rack_profiles (
  rack_id,
  site_id,
  nominal_capacity_kw,
  operating_margin_pct,
  thermal_density_limit_kw,
  aisle_zone,
  status
)
SELECT
  rack_id,
  site_id,
  120,
  10,
  140,
  'unknown',
  'active'
FROM racks
ON CONFLICT (rack_id) DO NOTHING;

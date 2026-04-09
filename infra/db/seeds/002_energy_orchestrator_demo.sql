BEGIN;

INSERT INTO energy_site_profiles (
  site_id,
  campus_name,
  target_capacity_mw,
  operating_reserve_pct,
  ambient_reference_c,
  ambient_derate_start_c,
  ambient_derate_pct_per_deg,
  advisory_mode,
  notes
)
VALUES
  ('site-cl-01', 'Copiapo Norte Campus', 20, 15, 25, 30, 0.50, 'advisory-first', 'Synthetic 20 MW site profile for energy-orchestrator demo.'),
  ('site-cl-02', 'Calama Sur Campus', 24, 15, 25, 30, 0.50, 'advisory-first', 'Synthetic 24 MW site profile for energy-orchestrator demo.')
ON CONFLICT (site_id) DO UPDATE
SET campus_name = EXCLUDED.campus_name,
    target_capacity_mw = EXCLUDED.target_capacity_mw,
    operating_reserve_pct = EXCLUDED.operating_reserve_pct,
    ambient_reference_c = EXCLUDED.ambient_reference_c,
    ambient_derate_start_c = EXCLUDED.ambient_derate_start_c,
    ambient_derate_pct_per_deg = EXCLUDED.ambient_derate_pct_per_deg,
    advisory_mode = EXCLUDED.advisory_mode,
    notes = EXCLUDED.notes,
    updated_at = NOW();

INSERT INTO energy_substations (
  substation_id,
  site_id,
  substation_name,
  voltage_level_kv,
  redundancy_mode,
  status
)
VALUES
  ('sub-cl-01-a', 'site-cl-01', 'Substation CL01-A', 13.8, 'n+1', 'active'),
  ('sub-cl-02-a', 'site-cl-02', 'Substation CL02-A', 13.8, 'n+1', 'active')
ON CONFLICT (substation_id) DO UPDATE
SET site_id = EXCLUDED.site_id,
    substation_name = EXCLUDED.substation_name,
    voltage_level_kv = EXCLUDED.voltage_level_kv,
    redundancy_mode = EXCLUDED.redundancy_mode,
    status = EXCLUDED.status,
    updated_at = NOW();

INSERT INTO energy_transformers (
  transformer_id,
  site_id,
  substation_id,
  transformer_name,
  nominal_capacity_kw,
  operating_margin_pct,
  ambient_derate_start_c,
  ambient_derate_pct_per_deg,
  status
)
VALUES
  ('tx-cl-01-a', 'site-cl-01', 'sub-cl-01-a', 'TX CL01-A', 12500, 12, 30, 0.60, 'active'),
  ('tx-cl-01-b', 'site-cl-01', 'sub-cl-01-a', 'TX CL01-B', 12500, 12, 30, 0.60, 'active'),
  ('tx-cl-02-a', 'site-cl-02', 'sub-cl-02-a', 'TX CL02-A', 15000, 12, 30, 0.60, 'active'),
  ('tx-cl-02-b', 'site-cl-02', 'sub-cl-02-a', 'TX CL02-B', 15000, 12, 30, 0.60, 'active')
ON CONFLICT (transformer_id) DO UPDATE
SET site_id = EXCLUDED.site_id,
    substation_id = EXCLUDED.substation_id,
    transformer_name = EXCLUDED.transformer_name,
    nominal_capacity_kw = EXCLUDED.nominal_capacity_kw,
    operating_margin_pct = EXCLUDED.operating_margin_pct,
    ambient_derate_start_c = EXCLUDED.ambient_derate_start_c,
    ambient_derate_pct_per_deg = EXCLUDED.ambient_derate_pct_per_deg,
    status = EXCLUDED.status,
    updated_at = NOW();

INSERT INTO energy_buses (
  bus_id,
  site_id,
  substation_id,
  transformer_id,
  bus_name,
  nominal_capacity_kw,
  operating_margin_pct,
  ambient_derate_start_c,
  ambient_derate_pct_per_deg,
  status
)
VALUES
  ('bus-cl-01-a', 'site-cl-01', 'sub-cl-01-a', 'tx-cl-01-a', 'Bus CL01-A', 6000, 10, 35, 0.35, 'active'),
  ('bus-cl-01-b', 'site-cl-01', 'sub-cl-01-a', 'tx-cl-01-b', 'Bus CL01-B', 6000, 10, 35, 0.35, 'active'),
  ('bus-cl-02-a', 'site-cl-02', 'sub-cl-02-a', 'tx-cl-02-a', 'Bus CL02-A', 7500, 10, 35, 0.35, 'active'),
  ('bus-cl-02-b', 'site-cl-02', 'sub-cl-02-a', 'tx-cl-02-b', 'Bus CL02-B', 7500, 10, 35, 0.35, 'active')
ON CONFLICT (bus_id) DO UPDATE
SET site_id = EXCLUDED.site_id,
    substation_id = EXCLUDED.substation_id,
    transformer_id = EXCLUDED.transformer_id,
    bus_name = EXCLUDED.bus_name,
    nominal_capacity_kw = EXCLUDED.nominal_capacity_kw,
    operating_margin_pct = EXCLUDED.operating_margin_pct,
    ambient_derate_start_c = EXCLUDED.ambient_derate_start_c,
    ambient_derate_pct_per_deg = EXCLUDED.ambient_derate_pct_per_deg,
    status = EXCLUDED.status,
    updated_at = NOW();

INSERT INTO energy_feeders (
  feeder_id,
  site_id,
  bus_id,
  feeder_name,
  nominal_capacity_kw,
  operating_margin_pct,
  ambient_derate_start_c,
  ambient_derate_pct_per_deg,
  status
)
VALUES
  ('feeder-cl-01-01', 'site-cl-01', 'bus-cl-01-a', 'Feeder CL01-01', 600, 8, 35, 0.25, 'active'),
  ('feeder-cl-01-02', 'site-cl-01', 'bus-cl-01-a', 'Feeder CL01-02', 600, 8, 35, 0.25, 'active'),
  ('feeder-cl-01-03', 'site-cl-01', 'bus-cl-01-a', 'Feeder CL01-03', 600, 8, 35, 0.25, 'active'),
  ('feeder-cl-01-04', 'site-cl-01', 'bus-cl-01-b', 'Feeder CL01-04', 600, 8, 35, 0.25, 'active'),
  ('feeder-cl-01-05', 'site-cl-01', 'bus-cl-01-b', 'Feeder CL01-05', 600, 8, 35, 0.25, 'active'),
  ('feeder-cl-02-01', 'site-cl-02', 'bus-cl-02-a', 'Feeder CL02-01', 650, 8, 35, 0.25, 'active'),
  ('feeder-cl-02-02', 'site-cl-02', 'bus-cl-02-a', 'Feeder CL02-02', 650, 8, 35, 0.25, 'active'),
  ('feeder-cl-02-03', 'site-cl-02', 'bus-cl-02-a', 'Feeder CL02-03', 650, 8, 35, 0.25, 'active'),
  ('feeder-cl-02-04', 'site-cl-02', 'bus-cl-02-b', 'Feeder CL02-04', 650, 8, 35, 0.25, 'active'),
  ('feeder-cl-02-05', 'site-cl-02', 'bus-cl-02-b', 'Feeder CL02-05', 650, 8, 35, 0.25, 'active')
ON CONFLICT (feeder_id) DO UPDATE
SET site_id = EXCLUDED.site_id,
    bus_id = EXCLUDED.bus_id,
    feeder_name = EXCLUDED.feeder_name,
    nominal_capacity_kw = EXCLUDED.nominal_capacity_kw,
    operating_margin_pct = EXCLUDED.operating_margin_pct,
    ambient_derate_start_c = EXCLUDED.ambient_derate_start_c,
    ambient_derate_pct_per_deg = EXCLUDED.ambient_derate_pct_per_deg,
    status = EXCLUDED.status,
    updated_at = NOW();

INSERT INTO energy_pdus (
  pdu_id,
  site_id,
  feeder_id,
  pdu_name,
  nominal_capacity_kw,
  operating_margin_pct,
  ambient_derate_start_c,
  ambient_derate_pct_per_deg,
  status
)
VALUES
  ('pdu-cl-01-01', 'site-cl-01', 'feeder-cl-01-01', 'PDU CL01-01', 180, 5, 40, 0.20, 'active'),
  ('pdu-cl-01-02', 'site-cl-01', 'feeder-cl-01-02', 'PDU CL01-02', 180, 5, 40, 0.20, 'active'),
  ('pdu-cl-01-03', 'site-cl-01', 'feeder-cl-01-03', 'PDU CL01-03', 180, 5, 40, 0.20, 'active'),
  ('pdu-cl-01-04', 'site-cl-01', 'feeder-cl-01-04', 'PDU CL01-04', 180, 5, 40, 0.20, 'active'),
  ('pdu-cl-01-05', 'site-cl-01', 'feeder-cl-01-05', 'PDU CL01-05', 180, 5, 40, 0.20, 'active'),
  ('pdu-cl-02-01', 'site-cl-02', 'feeder-cl-02-01', 'PDU CL02-01', 180, 5, 40, 0.20, 'active'),
  ('pdu-cl-02-02', 'site-cl-02', 'feeder-cl-02-02', 'PDU CL02-02', 180, 5, 40, 0.20, 'active'),
  ('pdu-cl-02-03', 'site-cl-02', 'feeder-cl-02-03', 'PDU CL02-03', 180, 5, 40, 0.20, 'active'),
  ('pdu-cl-02-04', 'site-cl-02', 'feeder-cl-02-04', 'PDU CL02-04', 180, 5, 40, 0.20, 'active'),
  ('pdu-cl-02-05', 'site-cl-02', 'feeder-cl-02-05', 'PDU CL02-05', 180, 5, 40, 0.20, 'active')
ON CONFLICT (pdu_id) DO UPDATE
SET site_id = EXCLUDED.site_id,
    feeder_id = EXCLUDED.feeder_id,
    pdu_name = EXCLUDED.pdu_name,
    nominal_capacity_kw = EXCLUDED.nominal_capacity_kw,
    operating_margin_pct = EXCLUDED.operating_margin_pct,
    ambient_derate_start_c = EXCLUDED.ambient_derate_start_c,
    ambient_derate_pct_per_deg = EXCLUDED.ambient_derate_pct_per_deg,
    status = EXCLUDED.status,
    updated_at = NOW();

INSERT INTO energy_rack_profiles (
  rack_id,
  site_id,
  bus_id,
  feeder_id,
  pdu_id,
  nominal_capacity_kw,
  operating_margin_pct,
  thermal_density_limit_kw,
  aisle_zone,
  status
)
VALUES
  ('rack-cl-01-01', 'site-cl-01', 'bus-cl-01-a', 'feeder-cl-01-01', 'pdu-cl-01-01', 120, 10, 140, 'aisle-a', 'active'),
  ('rack-cl-01-02', 'site-cl-01', 'bus-cl-01-a', 'feeder-cl-01-02', 'pdu-cl-01-02', 120, 10, 140, 'aisle-a', 'active'),
  ('rack-cl-01-03', 'site-cl-01', 'bus-cl-01-a', 'feeder-cl-01-03', 'pdu-cl-01-03', 120, 10, 140, 'aisle-a', 'active'),
  ('rack-cl-01-04', 'site-cl-01', 'bus-cl-01-b', 'feeder-cl-01-04', 'pdu-cl-01-04', 120, 10, 140, 'aisle-b', 'active'),
  ('rack-cl-01-05', 'site-cl-01', 'bus-cl-01-b', 'feeder-cl-01-05', 'pdu-cl-01-05', 120, 10, 140, 'aisle-b', 'active'),
  ('rack-cl-02-01', 'site-cl-02', 'bus-cl-02-a', 'feeder-cl-02-01', 'pdu-cl-02-01', 120, 10, 140, 'aisle-a', 'active'),
  ('rack-cl-02-02', 'site-cl-02', 'bus-cl-02-a', 'feeder-cl-02-02', 'pdu-cl-02-02', 120, 10, 140, 'aisle-a', 'active'),
  ('rack-cl-02-03', 'site-cl-02', 'bus-cl-02-a', 'feeder-cl-02-03', 'pdu-cl-02-03', 120, 10, 140, 'aisle-a', 'active'),
  ('rack-cl-02-04', 'site-cl-02', 'bus-cl-02-b', 'feeder-cl-02-04', 'pdu-cl-02-04', 120, 10, 140, 'aisle-b', 'active'),
  ('rack-cl-02-05', 'site-cl-02', 'bus-cl-02-b', 'feeder-cl-02-05', 'pdu-cl-02-05', 120, 10, 140, 'aisle-b', 'active')
ON CONFLICT (rack_id) DO UPDATE
SET site_id = EXCLUDED.site_id,
    bus_id = EXCLUDED.bus_id,
    feeder_id = EXCLUDED.feeder_id,
    pdu_id = EXCLUDED.pdu_id,
    nominal_capacity_kw = EXCLUDED.nominal_capacity_kw,
    operating_margin_pct = EXCLUDED.operating_margin_pct,
    thermal_density_limit_kw = EXCLUDED.thermal_density_limit_kw,
    aisle_zone = EXCLUDED.aisle_zone,
    status = EXCLUDED.status,
    updated_at = NOW();

INSERT INTO energy_miner_groups (
  miner_group_id,
  site_id,
  rack_id,
  group_name,
  miner_model,
  priority_class,
  target_miners,
  nominal_group_kw,
  status
)
VALUES
  ('group-cl-01-s21', 'site-cl-01', NULL, 'CL01 S21 Preferred', 'S21', 'preferred', 25, 90, 'active'),
  ('group-cl-01-s19xp', 'site-cl-01', NULL, 'CL01 S19XP Standard', 'S19XP', 'standard', 25, 76, 'active'),
  ('group-cl-02-s21', 'site-cl-02', NULL, 'CL02 S21 Preferred', 'S21', 'preferred', 25, 90, 'active'),
  ('group-cl-02-s19xp', 'site-cl-02', NULL, 'CL02 S19XP Standard', 'S19XP', 'standard', 25, 76, 'active')
ON CONFLICT (miner_group_id) DO UPDATE
SET site_id = EXCLUDED.site_id,
    rack_id = EXCLUDED.rack_id,
    group_name = EXCLUDED.group_name,
    miner_model = EXCLUDED.miner_model,
    priority_class = EXCLUDED.priority_class,
    target_miners = EXCLUDED.target_miners,
    nominal_group_kw = EXCLUDED.nominal_group_kw,
    status = EXCLUDED.status,
    updated_at = NOW();

INSERT INTO energy_maintenance_windows (
  window_id,
  site_id,
  asset_type,
  asset_id,
  window_from,
  window_to,
  status,
  reason
)
VALUES
  (
    'mw-energy-demo-cl01-feeder04',
    'site-cl-01',
    'feeder',
    'feeder-cl-01-04',
    NOW() + INTERVAL '2 days',
    NOW() + INTERVAL '2 days 4 hours',
    'scheduled',
    'Synthetic maintenance window for energy-orchestrator demo.'
  )
ON CONFLICT (window_id) DO UPDATE
SET site_id = EXCLUDED.site_id,
    asset_type = EXCLUDED.asset_type,
    asset_id = EXCLUDED.asset_id,
    window_from = EXCLUDED.window_from,
    window_to = EXCLUDED.window_to,
    status = EXCLUDED.status,
    reason = EXCLUDED.reason,
    updated_at = NOW();

COMMIT;

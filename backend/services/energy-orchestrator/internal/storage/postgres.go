package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/orchestrator"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(ctx context.Context, databaseURL string) (*PostgresRepository, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &PostgresRepository{pool: pool}, nil
}

func (r *PostgresRepository) Close() {
	if r.pool != nil {
		r.pool.Close()
	}
}

func (r *PostgresRepository) CreateBudgetSnapshot(ctx context.Context, input BudgetSnapshotCreateInput) (BudgetSnapshot, error) {
	snapshotJSON, err := json.Marshal(input.SnapshotPayload)
	if err != nil {
		return BudgetSnapshot{}, fmt.Errorf("marshal snapshot payload: %w", err)
	}

	upstreamContextJSON, err := json.Marshal(input.UpstreamContext)
	if err != nil {
		return BudgetSnapshot{}, fmt.Errorf("marshal upstream context: %w", err)
	}

	snapshot := BudgetSnapshot{
		SnapshotID:   uuid.NewString(),
		SiteID:       input.SiteID,
		Source:       input.Source,
		CalculatedAt: input.CalculatedAt.UTC(),
	}

	err = r.pool.QueryRow(ctx, `
INSERT INTO energy_budget_snapshots (
  snapshot_id,
  site_id,
  source,
  policy_mode,
  calculated_at,
  telemetry_observed_at,
  ambient_celsius,
  nominal_capacity_kw,
  effective_capacity_kw,
  reserved_capacity_kw,
  safe_capacity_kw,
  current_load_kw,
  available_capacity_kw,
  safe_dispatchable_kw,
  constraint_flags,
  snapshot_json,
  upstream_context_json
)
VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
)
RETURNING created_at
`,
		snapshot.SnapshotID,
		input.SiteID,
		input.Source,
		input.PolicyMode,
		input.CalculatedAt.UTC(),
		input.TelemetryObservedAt,
		input.AmbientCelsius,
		input.NominalCapacityKW,
		input.EffectiveCapacityKW,
		input.ReservedCapacityKW,
		input.SafeCapacityKW,
		input.CurrentLoadKW,
		input.AvailableCapacityKW,
		input.SafeDispatchableKW,
		snapshotConstraintFlags(input.ConstraintFlags),
		snapshotJSON,
		upstreamContextJSON,
	).Scan(&snapshot.CreatedAt)
	if err != nil {
		return BudgetSnapshot{}, fmt.Errorf("insert energy budget snapshot: %w", err)
	}

	return snapshot, nil
}

func (r *PostgresRepository) LoadBudgetInput(ctx context.Context, siteID string, at time.Time) (orchestrator.BudgetInput, error) {
	input := orchestrator.BudgetInput{
		At:                at.UTC(),
		CurrentRackLoadKW: map[string]float64{},
	}

	err := r.pool.QueryRow(ctx, `
SELECT
  esp.site_id,
  esp.campus_name,
  esp.target_capacity_mw,
  esp.operating_reserve_pct,
  esp.ambient_reference_c,
  esp.ambient_derate_start_c,
  esp.ambient_derate_pct_per_deg,
  esp.ramp_up_kw_per_interval,
  esp.ramp_down_kw_per_interval,
  esp.ramp_interval_seconds,
  esp.advisory_mode
FROM energy_site_profiles esp
WHERE esp.site_id = $1
`, siteID).Scan(
		&input.Site.SiteID,
		&input.Site.CampusName,
		&input.Site.TargetCapacityMW,
		&input.Site.OperatingReservePct,
		&input.Site.AmbientReferenceC,
		&input.Site.AmbientDerateStartC,
		&input.Site.AmbientDeratePctPerDeg,
		&input.Site.RampUpKWPerInterval,
		&input.Site.RampDownKWPerInterval,
		&input.Site.RampIntervalSeconds,
		&input.Site.AdvisoryMode,
	)
	if err != nil {
		return orchestrator.BudgetInput{}, fmt.Errorf("load energy site profile: %w", err)
	}

	input.AmbientCelsius = input.Site.AmbientReferenceC

	if err := r.loadAmbient(ctx, siteID, &input); err != nil {
		return orchestrator.BudgetInput{}, err
	}
	if err := r.loadSubstations(ctx, siteID, at, &input); err != nil {
		return orchestrator.BudgetInput{}, err
	}
	if err := r.loadAssets(ctx, siteID, at, &input.Transformers, "transformer", `
SELECT
  transformer_id,
  substation_id,
  transformer_name,
  nominal_capacity_kw,
  operating_margin_pct,
  ambient_derate_start_c,
  ambient_derate_pct_per_deg,
  status,
  EXISTS (
    SELECT 1
    FROM energy_maintenance_windows mw
    WHERE mw.site_id = t.site_id
      AND mw.asset_type = 'transformer'
      AND mw.asset_id = t.transformer_id
      AND mw.status IN ('scheduled', 'approved', 'active')
      AND $2 >= mw.window_from
      AND $2 <= mw.window_to
  ) AS maintenance_active
FROM energy_transformers t
WHERE t.site_id = $1
ORDER BY transformer_id
`); err != nil {
		return orchestrator.BudgetInput{}, err
	}
	if err := r.loadAssets(ctx, siteID, at, &input.Buses, "bus", `
SELECT
  bus_id,
  COALESCE(transformer_id, substation_id),
  bus_name,
  nominal_capacity_kw,
  operating_margin_pct,
  ambient_derate_start_c,
  ambient_derate_pct_per_deg,
  status,
  EXISTS (
    SELECT 1
    FROM energy_maintenance_windows mw
    WHERE mw.site_id = b.site_id
      AND mw.asset_type = 'bus'
      AND mw.asset_id = b.bus_id
      AND mw.status IN ('scheduled', 'approved', 'active')
      AND $2 >= mw.window_from
      AND $2 <= mw.window_to
  ) AS maintenance_active
FROM energy_buses b
WHERE b.site_id = $1
ORDER BY bus_id
`); err != nil {
		return orchestrator.BudgetInput{}, err
	}
	if err := r.loadAssets(ctx, siteID, at, &input.Feeders, "feeder", `
SELECT
  feeder_id,
  bus_id,
  feeder_name,
  nominal_capacity_kw,
  operating_margin_pct,
  ambient_derate_start_c,
  ambient_derate_pct_per_deg,
  status,
  EXISTS (
    SELECT 1
    FROM energy_maintenance_windows mw
    WHERE mw.site_id = f.site_id
      AND mw.asset_type = 'feeder'
      AND mw.asset_id = f.feeder_id
      AND mw.status IN ('scheduled', 'approved', 'active')
      AND $2 >= mw.window_from
      AND $2 <= mw.window_to
  ) AS maintenance_active
FROM energy_feeders f
WHERE f.site_id = $1
ORDER BY feeder_id
`); err != nil {
		return orchestrator.BudgetInput{}, err
	}
	if err := r.loadAssets(ctx, siteID, at, &input.PDUs, "pdu", `
SELECT
  pdu_id,
  feeder_id,
  pdu_name,
  nominal_capacity_kw,
  operating_margin_pct,
  ambient_derate_start_c,
  ambient_derate_pct_per_deg,
  status,
  EXISTS (
    SELECT 1
    FROM energy_maintenance_windows mw
    WHERE mw.site_id = p.site_id
      AND mw.asset_type = 'pdu'
      AND mw.asset_id = p.pdu_id
      AND mw.status IN ('scheduled', 'approved', 'active')
      AND $2 >= mw.window_from
      AND $2 <= mw.window_to
  ) AS maintenance_active
FROM energy_pdus p
WHERE p.site_id = $1
ORDER BY pdu_id
`); err != nil {
		return orchestrator.BudgetInput{}, err
	}
	if err := r.loadRackProfiles(ctx, siteID, at, &input); err != nil {
		return orchestrator.BudgetInput{}, err
	}
	if err := r.loadRackLoads(ctx, siteID, &input); err != nil {
		return orchestrator.BudgetInput{}, err
	}

	return input, nil
}

func (r *PostgresRepository) loadAmbient(ctx context.Context, siteID string, input *orchestrator.BudgetInput) error {
	var ambient *float64
	var observedAt *time.Time

	if err := r.pool.QueryRow(ctx, `
SELECT
  AVG(
    CASE
      WHEN NULLIF(tags->>'ambient_temp_c', '') ~ '^-?[0-9]+(\.[0-9]+)?$'
        THEN (tags->>'ambient_temp_c')::DOUBLE PRECISION
      ELSE NULL
    END
  ),
  MAX(ts)
FROM telemetry_latest
WHERE site_id = $1
`, siteID).Scan(&ambient, &observedAt); err != nil {
		return fmt.Errorf("load ambient telemetry: %w", err)
	}

	if ambient != nil {
		input.AmbientCelsius = *ambient
	}
	input.TelemetryObservedAt = observedAt

	return nil
}

func snapshotConstraintFlags(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return values
}

func (r *PostgresRepository) loadSubstations(ctx context.Context, siteID string, at time.Time, input *orchestrator.BudgetInput) error {
	rows, err := r.pool.Query(ctx, `
SELECT
  s.substation_id,
  s.site_id,
  s.substation_name,
  s.voltage_level_kv,
  s.redundancy_mode,
  s.status,
  EXISTS (
    SELECT 1
    FROM energy_maintenance_windows mw
    WHERE mw.site_id = s.site_id
      AND mw.asset_type = 'substation'
      AND mw.asset_id = s.substation_id
      AND mw.status IN ('scheduled', 'approved', 'active')
      AND $2 >= mw.window_from
      AND $2 <= mw.window_to
  ) AS maintenance_active
FROM energy_substations s
WHERE s.site_id = $1
ORDER BY s.substation_id
`, siteID, at.UTC())
	if err != nil {
		return fmt.Errorf("query substations: %w", err)
	}
	defer rows.Close()

	substations := make([]orchestrator.Substation, 0)
	for rows.Next() {
		var item orchestrator.Substation
		var status string
		if err := rows.Scan(
			&item.SubstationID,
			&item.SiteID,
			&item.SubstationName,
			&item.VoltageLevelKV,
			&item.RedundancyMode,
			&status,
			&item.MaintenanceActive,
		); err != nil {
			return fmt.Errorf("scan substation: %w", err)
		}
		item.Status = orchestrator.AssetStatus(status)
		substations = append(substations, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate substations: %w", err)
	}

	input.Substations = substations
	return nil
}

func (r *PostgresRepository) loadAssets(ctx context.Context, siteID string, at time.Time, target *[]orchestrator.CapacityAsset, kind string, query string) error {
	rows, err := r.pool.Query(ctx, query, siteID, at.UTC())
	if err != nil {
		return fmt.Errorf("query %s assets: %w", kind, err)
	}
	defer rows.Close()

	items := make([]orchestrator.CapacityAsset, 0)
	for rows.Next() {
		var item orchestrator.CapacityAsset
		var status string
		if err := rows.Scan(
			&item.ID,
			&item.ParentID,
			&item.Name,
			&item.NominalCapacityKW,
			&item.OperatingMarginPct,
			&item.AmbientDerateStartC,
			&item.AmbientDeratePctPerDeg,
			&status,
			&item.MaintenanceActive,
		); err != nil {
			return fmt.Errorf("scan %s asset: %w", kind, err)
		}
		item.Kind = orchestrator.AssetKind(kind)
		item.SiteID = siteID
		item.Status = orchestrator.AssetStatus(status)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate %s assets: %w", kind, err)
	}

	*target = items
	return nil
}

func (r *PostgresRepository) loadRackProfiles(ctx context.Context, siteID string, at time.Time, input *orchestrator.BudgetInput) error {
	rows, err := r.pool.Query(ctx, `
SELECT
  rp.rack_id,
  rp.site_id,
  COALESCE(rp.bus_id, ''),
  COALESCE(rp.feeder_id, ''),
  COALESCE(rp.pdu_id, ''),
  rp.nominal_capacity_kw,
  rp.operating_margin_pct,
  rp.thermal_density_limit_kw,
  rp.aisle_zone,
  rp.criticality_class,
  rp.criticality_reason,
  rp.safety_locked,
  COALESCE(rp.safety_lock_reason, ''),
  rp.ramp_up_kw_per_interval,
  rp.ramp_down_kw_per_interval,
  rp.status,
  EXISTS (
    SELECT 1
    FROM energy_maintenance_windows mw
    WHERE mw.site_id = rp.site_id
      AND mw.asset_type = 'rack'
      AND mw.asset_id = rp.rack_id
      AND mw.status IN ('scheduled', 'approved', 'active')
      AND $2 >= mw.window_from
      AND $2 <= mw.window_to
  ) AS maintenance_active
FROM energy_rack_profiles rp
WHERE rp.site_id = $1
ORDER BY rp.rack_id
`, siteID, at.UTC())
	if err != nil {
		return fmt.Errorf("query rack profiles: %w", err)
	}
	defer rows.Close()

	racks := make([]orchestrator.RackProfile, 0)
	for rows.Next() {
		var item orchestrator.RackProfile
		var status string
		if err := rows.Scan(
			&item.RackID,
			&item.SiteID,
			&item.BusID,
			&item.FeederID,
			&item.PDUID,
			&item.NominalCapacityKW,
			&item.OperatingMarginPct,
			&item.ThermalDensityLimitKW,
			&item.AisleZone,
			&item.CriticalityClass,
			&item.CriticalityReason,
			&item.SafetyLocked,
			&item.SafetyLockReason,
			&item.RampUpKWPerInterval,
			&item.RampDownKWPerInterval,
			&status,
			&item.MaintenanceActive,
		); err != nil {
			return fmt.Errorf("scan rack profile: %w", err)
		}
		item.Status = orchestrator.AssetStatus(status)
		racks = append(racks, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate rack profiles: %w", err)
	}

	input.Racks = racks
	return nil
}

func (r *PostgresRepository) loadRackLoads(ctx context.Context, siteID string, input *orchestrator.BudgetInput) error {
	rows, err := r.pool.Query(ctx, `
SELECT rack_id, COALESCE(SUM(power_watts), 0) / 1000.0 AS current_load_kw
FROM telemetry_latest
WHERE site_id = $1
GROUP BY rack_id
`, siteID)
	if err != nil {
		return fmt.Errorf("query rack loads: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var rackID string
		var currentLoadKW float64
		if err := rows.Scan(&rackID, &currentLoadKW); err != nil {
			return fmt.Errorf("scan rack load: %w", err)
		}
		input.CurrentRackLoadKW[rackID] = currentLoadKW
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate rack loads: %w", err)
	}

	return nil
}

func (r *PostgresRepository) LoadHistoricalReplayInput(ctx context.Context, siteID string, day time.Time) (orchestrator.HistoricalReplayInput, error) {
	input := orchestrator.HistoricalReplayInput{
		Day: day.UTC(),
	}

	budgetInput, err := r.LoadBudgetInput(ctx, siteID, day.UTC())
	if err != nil {
		return orchestrator.HistoricalReplayInput{}, err
	}
	input.Site = budgetInput.Site

	start := time.Date(day.UTC().Year(), day.UTC().Month(), day.UTC().Day(), 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	rows, err := r.pool.Query(ctx, `
WITH rack_readings AS (
  SELECT
    time_bucket(INTERVAL '5 minutes', tr.ts) AS bucket,
    tr.site_id,
    tr.rack_id,
    tr.miner_id,
    AVG(tr.hashrate_ths) AS avg_hashrate_ths,
    AVG(tr.power_watts) AS avg_power_watts,
    AVG(tr.temp_celsius) AS avg_temp_celsius,
    MAX(tr.temp_celsius) AS max_temp_celsius,
    AVG(tr.efficiency_jth) AS avg_efficiency_jth,
    AVG(
      CASE
        WHEN NULLIF(tr.tags->>'ambient_temp_c', '') ~ '^-?[0-9]+(\.[0-9]+)?$'
          THEN (tr.tags->>'ambient_temp_c')::DOUBLE PRECISION
        ELSE NULL
      END
    ) AS avg_ambient_celsius
  FROM telemetry_readings tr
  WHERE tr.site_id = $1
    AND tr.ts >= $2
    AND tr.ts < $3
  GROUP BY 1, 2, 3, 4
)
SELECT
  rr.bucket,
  rr.rack_id,
  COALESCE(MIN(m.miner_model), 'unknown') AS miner_model,
  COALESCE(rp.criticality_class, 'normal_production') AS criticality_class,
  COALESCE(SUM(rr.avg_power_watts) / 1000.0, 0) AS current_load_kw,
  COALESCE(SUM(rr.avg_hashrate_ths), 0) AS avg_hashrate_ths,
  COALESCE(SUM(rr.avg_power_watts), 0) AS avg_power_watts,
  COALESCE(AVG(rr.avg_temp_celsius), 0) AS avg_temp_celsius,
  COALESCE(MAX(rr.max_temp_celsius), 0) AS max_temp_celsius,
  COALESCE(AVG(COALESCE(rr.avg_ambient_celsius, esp.ambient_reference_c)), esp.ambient_reference_c) AS avg_ambient_celsius,
  COALESCE(SUM(rr.avg_power_watts) / NULLIF(SUM(rr.avg_hashrate_ths), 0), 0) AS avg_efficiency_jth,
  COALESCE(SUM(m.nominal_hashrate_ths), 0) AS nominal_hashrate_ths,
  COALESCE(SUM(m.nominal_power_watts), 0) AS nominal_power_watts,
  CASE
    WHEN rp.ramp_up_kw_per_interval > 0 THEN rp.ramp_up_kw_per_interval
    ELSE esp.ramp_up_kw_per_interval
  END AS ramp_up_kw_per_interval,
  CASE
    WHEN rp.ramp_down_kw_per_interval > 0 THEN rp.ramp_down_kw_per_interval
    ELSE esp.ramp_down_kw_per_interval
  END AS ramp_down_kw_per_interval,
  (rp.safety_locked OR rp.criticality_class = 'safety_blocked') AS safety_locked
FROM rack_readings rr
JOIN miners m
  ON m.miner_id = rr.miner_id
JOIN energy_rack_profiles rp
  ON rp.rack_id = rr.rack_id
JOIN energy_site_profiles esp
  ON esp.site_id = rr.site_id
GROUP BY
  rr.bucket,
  rr.rack_id,
  rp.criticality_class,
  rp.ramp_up_kw_per_interval,
  rp.ramp_down_kw_per_interval,
  rp.safety_locked,
  esp.ramp_up_kw_per_interval,
  esp.ramp_down_kw_per_interval,
  esp.ambient_reference_c
ORDER BY rr.bucket, rr.rack_id
`, siteID, start, end)
	if err != nil {
		return orchestrator.HistoricalReplayInput{}, fmt.Errorf("query historical replay input: %w", err)
	}
	defer rows.Close()

	points := make([]orchestrator.HistoricalRackPoint, 0)
	for rows.Next() {
		var item orchestrator.HistoricalRackPoint
		if err := rows.Scan(
			&item.Bucket,
			&item.RackID,
			&item.MinerModel,
			&item.CriticalityClass,
			&item.CurrentLoadKW,
			&item.AvgHashrateTHs,
			&item.AvgPowerWatts,
			&item.AvgTempCelsius,
			&item.MaxTempCelsius,
			&item.AvgAmbientCelsius,
			&item.AvgEfficiencyJTH,
			&item.NominalHashrateTHs,
			&item.NominalPowerWatts,
			&item.RampUpKWPerInterval,
			&item.RampDownKWPerInterval,
			&item.SafetyLocked,
		); err != nil {
			return orchestrator.HistoricalReplayInput{}, fmt.Errorf("scan historical replay point: %w", err)
		}
		points = append(points, item)
	}
	if err := rows.Err(); err != nil {
		return orchestrator.HistoricalReplayInput{}, fmt.Errorf("iterate historical replay points: %w", err)
	}
	if len(points) == 0 {
		return orchestrator.HistoricalReplayInput{}, pgx.ErrNoRows
	}
	input.Points = points

	if r.relationExists(ctx, "alerts") {
		if err := r.pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM alerts
WHERE site_id = $1
  AND first_seen_at >= $2
  AND first_seen_at < $3
`, siteID, start, end).Scan(&input.ObservedPersistedAlerts); err != nil {
			return orchestrator.HistoricalReplayInput{}, fmt.Errorf("count observed alerts for replay: %w", err)
		}
	}

	return input, nil
}

func (r *PostgresRepository) relationExists(ctx context.Context, name string) bool {
	var relationName *string
	if err := r.pool.QueryRow(ctx, `SELECT to_regclass($1)::text`, "public."+name).Scan(&relationName); err != nil {
		return false
	}
	return relationName != nil && *relationName != ""
}

func (r *PostgresRepository) ListSiteProfiles(ctx context.Context) ([]orchestrator.SiteProfile, error) {
	rows, err := r.pool.Query(ctx, `
SELECT
  site_id,
  campus_name,
  target_capacity_mw,
  operating_reserve_pct,
  ambient_reference_c,
  ambient_derate_start_c,
  ambient_derate_pct_per_deg,
  ramp_up_kw_per_interval,
  ramp_down_kw_per_interval,
  ramp_interval_seconds,
  advisory_mode
FROM energy_site_profiles
ORDER BY site_id
`)
	if err != nil {
		return nil, fmt.Errorf("query site profiles: %w", err)
	}
	defer rows.Close()

	items := make([]orchestrator.SiteProfile, 0)
	for rows.Next() {
		var item orchestrator.SiteProfile
		if err := rows.Scan(
			&item.SiteID,
			&item.CampusName,
			&item.TargetCapacityMW,
			&item.OperatingReservePct,
			&item.AmbientReferenceC,
			&item.AmbientDerateStartC,
			&item.AmbientDeratePctPerDeg,
			&item.RampUpKWPerInterval,
			&item.RampDownKWPerInterval,
			&item.RampIntervalSeconds,
			&item.AdvisoryMode,
		); err != nil {
			return nil, fmt.Errorf("scan site profile: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate site profiles: %w", err)
	}
	return items, nil
}

func (r *PostgresRepository) LoadRiskProjectionInput(ctx context.Context, siteID string, at time.Time) (orchestrator.RiskProjectionInput, error) {
	budgetInput, err := r.LoadBudgetInput(ctx, siteID, at)
	if err != nil {
		return orchestrator.RiskProjectionInput{}, err
	}
	budget := orchestrator.ComputeSiteBudget(budgetInput)

	start := at.UTC().Add(-4 * time.Hour)
	rows, err := r.pool.Query(ctx, `
SELECT
  time_bucket(INTERVAL '1 hour', ts) AS bucket,
  COALESCE(SUM(power_watts) / 1000.0, 0) AS load_kw,
  COALESCE(AVG(
    CASE
      WHEN NULLIF(tags->>'ambient_temp_c', '') ~ '^-?[0-9]+(\.[0-9]+)?$'
        THEN (tags->>'ambient_temp_c')::DOUBLE PRECISION
      ELSE NULL
    END
  ), 0) AS ambient_celsius,
  COALESCE(MAX(temp_celsius), 0) AS max_temp_celsius,
  COUNT(*) FILTER (WHERE status IN ('critical', 'offline'))::bigint AS critical_events,
  COALESCE(
    COUNT(*) FILTER (WHERE hashrate_ths <= 0 AND power_watts > 0)::DOUBLE PRECISION / NULLIF(COUNT(*), 0),
    0
  ) AS sensor_error_rate
FROM telemetry_readings
WHERE site_id = $1
  AND ts >= $2
  AND ts <= $3
GROUP BY 1
ORDER BY 1
`, siteID, start, at.UTC())
	if err != nil {
		return orchestrator.RiskProjectionInput{}, fmt.Errorf("query risk projection samples: %w", err)
	}
	defer rows.Close()

	samples := make([]orchestrator.SiteProjectionSample, 0)
	for rows.Next() {
		var item orchestrator.SiteProjectionSample
		if err := rows.Scan(
			&item.Bucket,
			&item.LoadKW,
			&item.AmbientCelsius,
			&item.MaxTempCelsius,
			&item.CriticalEvents,
			&item.SensorErrorRate,
		); err != nil {
			return orchestrator.RiskProjectionInput{}, fmt.Errorf("scan risk projection sample: %w", err)
		}
		samples = append(samples, item)
	}
	if err := rows.Err(); err != nil {
		return orchestrator.RiskProjectionInput{}, fmt.Errorf("iterate risk projection samples: %w", err)
	}

	return orchestrator.RiskProjectionInput{
		At:            at.UTC(),
		Site:          budgetInput.Site,
		Budget:        budget,
		Samples:       samples,
		CurrentTariff: r.loadTariffWindow(ctx, siteID, at),
	}, nil
}

func (r *PostgresRepository) loadTariffWindow(ctx context.Context, siteID string, at time.Time) *orchestrator.TariffWindow {
	if !r.relationExists(ctx, "energy_tariff_windows") {
		return nil
	}

	var item orchestrator.TariffWindow
	err := r.pool.QueryRow(ctx, `
SELECT
  window_id,
  site_id,
  tariff_code,
  price_usd_per_mwh,
  effective_from,
  effective_to,
  price_usd_per_mwh >= 140 AS is_expensive_band
FROM energy_tariff_windows
WHERE site_id = $1
  AND $2 >= effective_from
  AND $2 < effective_to
ORDER BY effective_from DESC
LIMIT 1
`, siteID, at.UTC()).Scan(
		&item.WindowID,
		&item.SiteID,
		&item.TariffCode,
		&item.PriceUSDPerMWh,
		&item.EffectiveFrom,
		&item.EffectiveTo,
		&item.IsExpensiveBand,
	)
	if err != nil {
		return nil
	}
	return &item
}

func (r *PostgresRepository) GetRecommendationReview(ctx context.Context, siteID, recommendationID string) (orchestrator.RecommendationReview, error) {
	row := r.pool.QueryRow(ctx, `
SELECT
  review_id,
  site_id,
  snapshot_id,
  recommendation_id,
  COALESCE(rack_id, ''),
  action,
  criticality_class,
  requested_delta_kw,
  recommended_delta_kw,
  reason,
  decision,
  status,
  sensitivity,
  requires_dual_confirmation,
  requested_by,
  requested_by_role,
  COALESCE(first_approved_by, ''),
  COALESCE(second_approved_by, ''),
  COALESCE(rejected_by, ''),
  COALESCE(postponed_by, ''),
  postponed_until,
  COALESCE(comment, ''),
  final_decision_at,
  created_at,
  updated_at
FROM energy_recommendation_reviews
WHERE site_id = $1
  AND recommendation_id = $2
`, siteID, recommendationID)
	return scanRecommendationReview(row)
}

func (r *PostgresRepository) ListRecommendationReviews(ctx context.Context, siteID, status string) ([]orchestrator.RecommendationReview, error) {
	query := `
SELECT
  review_id,
  site_id,
  snapshot_id,
  recommendation_id,
  COALESCE(rack_id, ''),
  action,
  criticality_class,
  requested_delta_kw,
  recommended_delta_kw,
  reason,
  decision,
  status,
  sensitivity,
  requires_dual_confirmation,
  requested_by,
  requested_by_role,
  COALESCE(first_approved_by, ''),
  COALESCE(second_approved_by, ''),
  COALESCE(rejected_by, ''),
  COALESCE(postponed_by, ''),
  postponed_until,
  COALESCE(comment, ''),
  final_decision_at,
  created_at,
  updated_at
FROM energy_recommendation_reviews
WHERE site_id = $1
`
	args := []any{siteID}
	if status != "" {
		query += ` AND status = $2`
		args = append(args, status)
	}
	query += ` ORDER BY updated_at DESC, created_at DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query recommendation reviews: %w", err)
	}
	defer rows.Close()

	items := make([]orchestrator.RecommendationReview, 0)
	for rows.Next() {
		item, err := scanRecommendationReview(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recommendation reviews: %w", err)
	}
	return items, nil
}

func (r *PostgresRepository) CreateRecommendationReview(ctx context.Context, input RecommendationReviewCreateInput) (orchestrator.RecommendationReview, error) {
	row := r.pool.QueryRow(ctx, `
INSERT INTO energy_recommendation_reviews (
  review_id,
  site_id,
  snapshot_id,
  recommendation_id,
  rack_id,
  action,
  criticality_class,
  requested_delta_kw,
  recommended_delta_kw,
  reason,
  decision,
  status,
  sensitivity,
  requires_dual_confirmation,
  requested_by,
  requested_by_role,
  first_approved_by,
  second_approved_by,
  rejected_by,
  postponed_by,
  postponed_until,
  comment,
  final_decision_at
)
VALUES (
  $1, $2, $3, $4, NULLIF($5, ''), $6, $7, $8, $9, $10, $11, $12, $13, $14,
  $15, $16, NULLIF($17, ''), NULLIF($18, ''), NULLIF($19, ''), NULLIF($20, ''), $21, $22, $23
)
RETURNING
  review_id,
  site_id,
  snapshot_id,
  recommendation_id,
  COALESCE(rack_id, ''),
  action,
  criticality_class,
  requested_delta_kw,
  recommended_delta_kw,
  reason,
  decision,
  status,
  sensitivity,
  requires_dual_confirmation,
  requested_by,
  requested_by_role,
  COALESCE(first_approved_by, ''),
  COALESCE(second_approved_by, ''),
  COALESCE(rejected_by, ''),
  COALESCE(postponed_by, ''),
  postponed_until,
  COALESCE(comment, ''),
  final_decision_at,
  created_at,
  updated_at
`, uuid.NewString(), input.SiteID, input.SnapshotID, input.RecommendationID, input.RackID, input.Action, input.CriticalityClass,
		input.RequestedDeltaKW, input.RecommendedDeltaKW, input.Reason, input.Decision, input.Status, input.Sensitivity,
		input.RequiresDualConfirmation, input.RequestedBy, input.RequestedByRole, input.FirstApprovedBy, input.SecondApprovedBy,
		input.RejectedBy, input.PostponedBy, input.PostponedUntil, input.Comment, input.FinalDecisionAt,
	)
	return scanRecommendationReview(row)
}

func (r *PostgresRepository) ListRecommendationReviewEvents(ctx context.Context, reviewID string) ([]orchestrator.RecommendationReviewEvent, error) {
	rows, err := r.pool.Query(ctx, `
SELECT
  event_id,
  review_id,
  site_id,
  COALESCE(rack_id, ''),
  actor_id,
  actor_role,
  event_type,
  COALESCE(decision, ''),
  COALESCE(comment, ''),
  created_at
FROM energy_recommendation_review_events
WHERE review_id = $1
ORDER BY created_at ASC, event_id ASC
`, reviewID)
	if err != nil {
		return nil, fmt.Errorf("query recommendation review events: %w", err)
	}
	defer rows.Close()

	items := make([]orchestrator.RecommendationReviewEvent, 0)
	for rows.Next() {
		item, err := scanRecommendationReviewEvent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recommendation review events: %w", err)
	}
	return items, nil
}

func (r *PostgresRepository) UpdateRecommendationReview(ctx context.Context, input RecommendationReviewUpdateInput) (orchestrator.RecommendationReview, error) {
	row := r.pool.QueryRow(ctx, `
UPDATE energy_recommendation_reviews
SET
  status = $2,
  decision = $3,
  first_approved_by = NULLIF($4, ''),
  second_approved_by = NULLIF($5, ''),
  rejected_by = NULLIF($6, ''),
  postponed_by = NULLIF($7, ''),
  postponed_until = $8,
  comment = $9,
  final_decision_at = $10,
  updated_at = NOW()
WHERE review_id = $1
RETURNING
  review_id,
  site_id,
  snapshot_id,
  recommendation_id,
  COALESCE(rack_id, ''),
  action,
  criticality_class,
  requested_delta_kw,
  recommended_delta_kw,
  reason,
  decision,
  status,
  sensitivity,
  requires_dual_confirmation,
  requested_by,
  requested_by_role,
  COALESCE(first_approved_by, ''),
  COALESCE(second_approved_by, ''),
  COALESCE(rejected_by, ''),
  COALESCE(postponed_by, ''),
  postponed_until,
  COALESCE(comment, ''),
  final_decision_at,
  created_at,
  updated_at
`, input.ReviewID, input.Status, input.Decision, input.FirstApprovedBy, input.SecondApprovedBy, input.RejectedBy, input.PostponedBy, input.PostponedUntil, input.Comment, input.FinalDecisionAt)
	return scanRecommendationReview(row)
}

func (r *PostgresRepository) AppendRecommendationReviewEvent(ctx context.Context, input RecommendationReviewEventCreateInput) (orchestrator.RecommendationReviewEvent, error) {
	event := orchestrator.RecommendationReviewEvent{}
	err := r.pool.QueryRow(ctx, `
INSERT INTO energy_recommendation_review_events (
  event_id,
  review_id,
  site_id,
  rack_id,
  actor_id,
  actor_role,
  event_type,
  decision,
  comment
)
VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7, NULLIF($8, ''), $9)
RETURNING event_id, review_id, site_id, COALESCE(rack_id, ''), actor_id, actor_role, event_type, COALESCE(decision, ''), comment, created_at
`, uuid.NewString(), input.ReviewID, input.SiteID, input.RackID, input.ActorID, input.ActorRole, input.EventType, input.Decision, input.Comment).Scan(
		&event.EventID,
		&event.ReviewID,
		&event.SiteID,
		&event.RackID,
		&event.ActorID,
		&event.ActorRole,
		&event.EventType,
		&event.Decision,
		&event.Comment,
		&event.CreatedAt,
	)
	if err != nil {
		return orchestrator.RecommendationReviewEvent{}, fmt.Errorf("insert recommendation review event: %w", err)
	}
	return event, nil
}

type reviewScanner interface {
	Scan(dest ...any) error
}

func scanRecommendationReview(scanner reviewScanner) (orchestrator.RecommendationReview, error) {
	var item orchestrator.RecommendationReview
	if err := scanner.Scan(
		&item.ReviewID,
		&item.SiteID,
		&item.SnapshotID,
		&item.RecommendationID,
		&item.RackID,
		&item.Action,
		&item.CriticalityClass,
		&item.RequestedDeltaKW,
		&item.RecommendedDeltaKW,
		&item.Reason,
		&item.Decision,
		&item.Status,
		&item.Sensitivity,
		&item.RequiresDualConfirmation,
		&item.RequestedBy,
		&item.RequestedByRole,
		&item.FirstApprovedBy,
		&item.SecondApprovedBy,
		&item.RejectedBy,
		&item.PostponedBy,
		&item.PostponedUntil,
		&item.Comment,
		&item.FinalDecisionAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return orchestrator.RecommendationReview{}, fmt.Errorf("scan recommendation review: %w", err)
	}
	return item, nil
}

func scanRecommendationReviewEvent(scanner reviewScanner) (orchestrator.RecommendationReviewEvent, error) {
	var item orchestrator.RecommendationReviewEvent
	if err := scanner.Scan(
		&item.EventID,
		&item.ReviewID,
		&item.SiteID,
		&item.RackID,
		&item.ActorID,
		&item.ActorRole,
		&item.EventType,
		&item.Decision,
		&item.Comment,
		&item.CreatedAt,
	); err != nil {
		return orchestrator.RecommendationReviewEvent{}, fmt.Errorf("scan recommendation review event: %w", err)
	}
	return item, nil
}

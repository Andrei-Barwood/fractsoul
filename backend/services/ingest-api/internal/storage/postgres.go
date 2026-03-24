package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/telemetry"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultQueryLimit = 50
	maxQueryLimit     = 500
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

func (r *PostgresRepository) PersistTelemetry(ctx context.Context, request telemetry.IngestRequest, rawPayload []byte) error {
	tags := request.Tags
	if tags == nil {
		tags = map[string]string{}
	}

	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	siteName := strings.ToUpper(request.SiteID)
	countryCode := inferCountryCode(request.SiteID)

	_, err = tx.Exec(ctx, `
INSERT INTO sites (site_id, site_name, country_code, timezone)
VALUES ($1, $2, $3, $4)
ON CONFLICT (site_id) DO UPDATE
SET site_name = EXCLUDED.site_name,
    country_code = EXCLUDED.country_code
`, request.SiteID, siteName, countryCode, "UTC")
	if err != nil {
		return fmt.Errorf("upsert site: %w", err)
	}

	rackLabel := strings.ToUpper(request.RackID)
	_, err = tx.Exec(ctx, `
INSERT INTO racks (rack_id, site_id, rack_label, max_miners)
VALUES ($1, $2, $3, $4)
ON CONFLICT (rack_id) DO UPDATE
SET site_id = EXCLUDED.site_id,
    rack_label = EXCLUDED.rack_label
`, request.RackID, request.SiteID, rackLabel, 24)
	if err != nil {
		return fmt.Errorf("upsert rack: %w", err)
	}

	nominalHashrate := request.Metrics.HashrateTHs
	if nominalHashrate <= 0 {
		nominalHashrate = 1
	}
	nominalPower := request.Metrics.PowerWatts
	if nominalPower <= 0 {
		nominalPower = 1
	}

	_, err = tx.Exec(ctx, `
INSERT INTO miners (
  miner_id,
  site_id,
  rack_id,
  miner_model,
  firmware_version,
  nominal_hashrate_ths,
  nominal_power_watts,
  is_active
)
VALUES ($1, $2, $3, $4, $5, $6, $7, TRUE)
ON CONFLICT (miner_id) DO UPDATE
SET site_id = EXCLUDED.site_id,
    rack_id = EXCLUDED.rack_id,
    firmware_version = EXCLUDED.firmware_version,
    nominal_hashrate_ths = EXCLUDED.nominal_hashrate_ths,
    nominal_power_watts = EXCLUDED.nominal_power_watts,
    is_active = TRUE
`, request.MinerID, request.SiteID, request.RackID, "unknown", request.FirmwareVersion, nominalHashrate, nominalPower)
	if err != nil {
		return fmt.Errorf("upsert miner: %w", err)
	}

	loadPct := parseLoadPct(tags)

	_, err = tx.Exec(ctx, `
INSERT INTO telemetry_readings (
  ts,
  event_id,
  site_id,
  rack_id,
  miner_id,
  firmware_version,
  hashrate_ths,
  power_watts,
  temp_celsius,
  fan_rpm,
  efficiency_jth,
  status,
  load_pct,
  tags,
  raw_payload
)
VALUES (
  $1, $2, $3, $4, $5, $6,
  $7, $8, $9, $10, $11, $12,
  $13, $14, $15
)
ON CONFLICT (event_id, ts) DO NOTHING
`,
		request.Timestamp.UTC(),
		request.EventID,
		request.SiteID,
		request.RackID,
		request.MinerID,
		request.FirmwareVersion,
		request.Metrics.HashrateTHs,
		request.Metrics.PowerWatts,
		request.Metrics.TempCelsius,
		request.Metrics.FanRPM,
		request.Metrics.EfficiencyJTH,
		string(request.Metrics.Status),
		loadPct,
		tagsJSON,
		rawPayload,
	)
	if err != nil {
		return fmt.Errorf("insert telemetry reading: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (r *PostgresRepository) ListReadings(ctx context.Context, filter ReadingsFilter) ([]TelemetryReading, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultQueryLimit
	}
	if limit > maxQueryLimit {
		limit = maxQueryLimit
	}

	query := `
SELECT
  ts,
  event_id::text,
  site_id,
  rack_id,
  miner_id,
  firmware_version,
  hashrate_ths,
  power_watts,
  temp_celsius,
  fan_rpm,
  efficiency_jth,
  status,
  load_pct,
  tags,
  ingested_at
FROM telemetry_readings
`
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 6)
	argPos := 1

	if filter.SiteID != "" {
		clauses = append(clauses, fmt.Sprintf("site_id = $%d", argPos))
		args = append(args, filter.SiteID)
		argPos++
	}
	if filter.RackID != "" {
		clauses = append(clauses, fmt.Sprintf("rack_id = $%d", argPos))
		args = append(args, filter.RackID)
		argPos++
	}
	if filter.MinerID != "" {
		clauses = append(clauses, fmt.Sprintf("miner_id = $%d", argPos))
		args = append(args, filter.MinerID)
		argPos++
	}
	if filter.From != nil {
		clauses = append(clauses, fmt.Sprintf("ts >= $%d", argPos))
		args = append(args, filter.From.UTC())
		argPos++
	}
	if filter.To != nil {
		clauses = append(clauses, fmt.Sprintf("ts <= $%d", argPos))
		args = append(args, filter.To.UTC())
		argPos++
	}

	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	query += fmt.Sprintf(" ORDER BY ts DESC LIMIT $%d", argPos)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query telemetry readings: %w", err)
	}
	defer rows.Close()

	items := make([]TelemetryReading, 0, limit)
	for rows.Next() {
		var item TelemetryReading
		var status string
		var tagsRaw []byte
		if err := rows.Scan(
			&item.Timestamp,
			&item.EventID,
			&item.SiteID,
			&item.RackID,
			&item.MinerID,
			&item.FirmwareVersion,
			&item.HashrateTHs,
			&item.PowerWatts,
			&item.TempCelsius,
			&item.FanRPM,
			&item.EfficiencyJTH,
			&status,
			&item.LoadPct,
			&tagsRaw,
			&item.IngestedAt,
		); err != nil {
			return nil, fmt.Errorf("scan telemetry reading: %w", err)
		}

		item.Status = telemetry.Status(status)
		if len(tagsRaw) > 0 {
			if err := json.Unmarshal(tagsRaw, &item.Tags); err != nil {
				return nil, fmt.Errorf("decode tags json: %w", err)
			}
		} else {
			item.Tags = map[string]string{}
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate telemetry readings: %w", err)
	}

	return items, nil
}

func (r *PostgresRepository) SummarizeReadings(ctx context.Context, filter SummaryFilter) (TelemetrySummary, error) {
	windowMinutes := filter.WindowMinutes
	if windowMinutes <= 0 {
		windowMinutes = 60
	}
	if windowMinutes > 24*60 {
		windowMinutes = 24 * 60
	}

	query := `
SELECT
  COUNT(*) AS samples,
  COALESCE(AVG(hashrate_ths), 0),
  COALESCE(AVG(power_watts), 0),
  COALESCE(AVG(temp_celsius), 0),
  COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY temp_celsius), 0),
  COALESCE(MAX(temp_celsius), 0),
  COALESCE(AVG(fan_rpm), 0),
  COALESCE(AVG(efficiency_jth), 0)
FROM telemetry_readings
WHERE ts >= NOW() - ($1::int * INTERVAL '1 minute')
`
	args := []any{windowMinutes}
	argPos := 2

	if filter.SiteID != "" {
		query += fmt.Sprintf(" AND site_id = $%d", argPos)
		args = append(args, filter.SiteID)
		argPos++
	}
	if filter.RackID != "" {
		query += fmt.Sprintf(" AND rack_id = $%d", argPos)
		args = append(args, filter.RackID)
		argPos++
	}
	if filter.MinerID != "" {
		query += fmt.Sprintf(" AND miner_id = $%d", argPos)
		args = append(args, filter.MinerID)
	}

	summary := TelemetrySummary{WindowMinutes: windowMinutes}
	if err := r.pool.QueryRow(ctx, query, args...).Scan(
		&summary.Samples,
		&summary.AvgHashrateTHs,
		&summary.AvgPowerWatts,
		&summary.AvgTempCelsius,
		&summary.P95TempCelsius,
		&summary.MaxTempCelsius,
		&summary.AvgFanRPM,
		&summary.AvgEfficiencyJTH,
	); err != nil {
		return TelemetrySummary{}, fmt.Errorf("query telemetry summary: %w", err)
	}

	return summary, nil
}

func (r *PostgresRepository) Close() {
	if r == nil || r.pool == nil {
		return
	}
	r.pool.Close()
}

func parseLoadPct(tags map[string]string) float64 {
	if raw, ok := tags["load_pct"]; ok {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil && parsed >= 0 && parsed <= 150 {
			return parsed
		}
	}

	return 100
}

func inferCountryCode(siteID string) string {
	parts := strings.Split(strings.ToLower(siteID), "-")
	if len(parts) >= 3 && len(parts[1]) == 2 {
		return strings.ToUpper(parts[1])
	}
	return "NA"
}

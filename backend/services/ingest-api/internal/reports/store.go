package reports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(ctx context.Context, databaseURL string) (*Store, error) {
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

	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	if s == nil || s.pool == nil {
		return
	}
	s.pool.Close()
}

func (s *Store) CollectMetrics(ctx context.Context, from, to time.Time) (DailyMetrics, error) {
	metrics := DailyMetrics{}

	global, err := s.queryGlobalMetrics(ctx, from, to)
	if err != nil {
		return DailyMetrics{}, err
	}
	metrics.Global = global

	sites, err := s.querySiteMetrics(ctx, from, to)
	if err != nil {
		return DailyMetrics{}, err
	}
	metrics.Sites = sites

	alerts, err := s.queryAlertMetrics(ctx, from, to)
	if err != nil {
		return DailyMetrics{}, err
	}
	metrics.Alerts = alerts

	changes, err := s.queryChangeMetrics(ctx, from, to)
	if err != nil {
		return DailyMetrics{}, err
	}
	metrics.Changes = changes

	hotspots, err := s.queryHotspots(ctx, from, to, 5)
	if err != nil {
		return DailyMetrics{}, err
	}
	metrics.Hotspots = hotspots

	return metrics, nil
}

func (s *Store) SaveDailyReport(ctx context.Context, report Report, markdown string) (string, error) {
	reportID := report.ReportID
	if reportID == "" {
		reportID = uuid.NewString()
	}

	reportJSON, err := json.Marshal(report)
	if err != nil {
		return "", fmt.Errorf("marshal report json: %w", err)
	}

	var persistedID string
	err = s.pool.QueryRow(ctx, `
INSERT INTO daily_reports (
  report_id,
  report_date,
  timezone,
  window_from,
  window_to,
  generated_at,
  report_json,
  report_markdown
)
VALUES (
  $1,
  $2::date,
  $3,
  $4,
  $5,
  $6,
  $7,
  $8
)
ON CONFLICT (report_date, timezone) DO UPDATE
SET
  report_id = EXCLUDED.report_id,
  window_from = EXCLUDED.window_from,
  window_to = EXCLUDED.window_to,
  generated_at = EXCLUDED.generated_at,
  report_json = EXCLUDED.report_json,
  report_markdown = EXCLUDED.report_markdown,
  updated_at = NOW()
RETURNING report_id
`, reportID, report.ReportDate, report.Timezone, report.WindowFrom, report.WindowTo, report.GeneratedAt, reportJSON, markdown).Scan(&persistedID)
	if err != nil {
		return "", fmt.Errorf("upsert daily report: %w", err)
	}

	return persistedID, nil
}

func (s *Store) queryGlobalMetrics(ctx context.Context, from, to time.Time) (GlobalMetrics, error) {
	row := GlobalMetrics{}
	err := s.pool.QueryRow(ctx, `
SELECT
  COUNT(*)::bigint AS samples,
  COUNT(DISTINCT miner_id)::bigint AS active_miners,
  COALESCE(AVG(hashrate_ths), 0) AS avg_hashrate_ths,
  COALESCE(AVG(power_watts), 0) AS avg_power_watts,
  COALESCE(AVG(temp_celsius), 0) AS avg_temp_celsius,
  COALESCE(AVG(efficiency_jth), 0) AS avg_efficiency_jth,
  COALESCE(SUM(CASE WHEN status = 'critical' THEN 1 ELSE 0 END), 0)::bigint AS critical_events,
  COALESCE(SUM(CASE WHEN status = 'warning' THEN 1 ELSE 0 END), 0)::bigint AS warning_events
FROM telemetry_readings
WHERE ts >= $1
  AND ts < $2
`, from.UTC(), to.UTC()).Scan(
		&row.Samples,
		&row.ActiveMiners,
		&row.AvgHashrateTHs,
		&row.AvgPowerWatts,
		&row.AvgTempCelsius,
		&row.AvgEfficiencyJTH,
		&row.CriticalEvents,
		&row.WarningEvents,
	)
	if err != nil {
		return GlobalMetrics{}, fmt.Errorf("query global metrics: %w", err)
	}
	return row, nil
}

func (s *Store) querySiteMetrics(ctx context.Context, from, to time.Time) ([]SiteMetrics, error) {
	rows, err := s.pool.Query(ctx, `
SELECT
  site_id,
  COUNT(*)::bigint AS samples,
  COUNT(DISTINCT miner_id)::bigint AS active_miners,
  COALESCE(AVG(hashrate_ths), 0) AS avg_hashrate_ths,
  COALESCE(AVG(power_watts), 0) AS avg_power_watts,
  COALESCE(AVG(temp_celsius), 0) AS avg_temp_celsius,
  COALESCE(AVG(efficiency_jth), 0) AS avg_efficiency_jth,
  COALESCE(SUM(CASE WHEN status = 'critical' THEN 1 ELSE 0 END), 0)::bigint AS critical_events,
  COALESCE(SUM(CASE WHEN status = 'warning' THEN 1 ELSE 0 END), 0)::bigint AS warning_events
FROM telemetry_readings
WHERE ts >= $1
  AND ts < $2
GROUP BY site_id
ORDER BY site_id ASC
`, from.UTC(), to.UTC())
	if err != nil {
		return nil, fmt.Errorf("query site metrics: %w", err)
	}
	defer rows.Close()

	items := make([]SiteMetrics, 0)
	for rows.Next() {
		var item SiteMetrics
		if err := rows.Scan(
			&item.SiteID,
			&item.Samples,
			&item.ActiveMiners,
			&item.AvgHashrateTHs,
			&item.AvgPowerWatts,
			&item.AvgTempCelsius,
			&item.AvgEfficiencyJTH,
			&item.CriticalEvents,
			&item.WarningEvents,
		); err != nil {
			return nil, fmt.Errorf("scan site metric: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate site metrics: %w", err)
	}

	return items, nil
}

func (s *Store) queryAlertMetrics(ctx context.Context, from, to time.Time) (AlertMetrics, error) {
	if !s.relationExists(ctx, "alerts") {
		return AlertMetrics{}, nil
	}

	metrics := AlertMetrics{}
	err := s.pool.QueryRow(ctx, `
SELECT
  COUNT(*)::bigint AS total,
  COALESCE(SUM(CASE WHEN severity = 'critical' THEN 1 ELSE 0 END), 0)::bigint AS critical,
  COALESCE(SUM(CASE WHEN severity = 'warning' THEN 1 ELSE 0 END), 0)::bigint AS warning
FROM alerts
WHERE last_seen_at >= $1
  AND last_seen_at < $2
`, from.UTC(), to.UTC()).Scan(&metrics.Total, &metrics.Critical, &metrics.Warning)
	if err != nil {
		if isUndefinedRelation(err) {
			return AlertMetrics{}, nil
		}
		return AlertMetrics{}, fmt.Errorf("query alert metrics: %w", err)
	}

	rows, err := s.pool.Query(ctx, `
SELECT
  rule_id,
  COUNT(*)::bigint AS total
FROM alerts
WHERE last_seen_at >= $1
  AND last_seen_at < $2
GROUP BY rule_id
ORDER BY total DESC
LIMIT 5
`, from.UTC(), to.UTC())
	if err != nil {
		if isUndefinedRelation(err) {
			return metrics, nil
		}
		return AlertMetrics{}, fmt.Errorf("query top alert rules: %w", err)
	}
	defer rows.Close()

	metrics.TopRules = make([]AlertRuleCount, 0, 5)
	for rows.Next() {
		var item AlertRuleCount
		if err := rows.Scan(&item.RuleID, &item.Count); err != nil {
			return AlertMetrics{}, fmt.Errorf("scan top rule: %w", err)
		}
		metrics.TopRules = append(metrics.TopRules, item)
	}
	if err := rows.Err(); err != nil {
		return AlertMetrics{}, fmt.Errorf("iterate top rules: %w", err)
	}

	return metrics, nil
}

func (s *Store) queryChangeMetrics(ctx context.Context, from, to time.Time) (ChangeMetrics, error) {
	if !s.relationExists(ctx, "recommendation_changes") {
		return ChangeMetrics{}, nil
	}

	metrics := ChangeMetrics{}
	err := s.pool.QueryRow(ctx, `
SELECT
  COALESCE(SUM(CASE WHEN operation = 'apply' THEN 1 ELSE 0 END), 0)::bigint AS applied,
  COALESCE(SUM(CASE WHEN operation = 'rollback' THEN 1 ELSE 0 END), 0)::bigint AS rolled_back
FROM recommendation_changes
WHERE created_at >= $1
  AND created_at < $2
`, from.UTC(), to.UTC()).Scan(&metrics.Applied, &metrics.RolledBack)
	if err != nil {
		if isUndefinedRelation(err) {
			return ChangeMetrics{}, nil
		}
		return ChangeMetrics{}, fmt.Errorf("query change metrics: %w", err)
	}

	return metrics, nil
}

func (s *Store) queryHotspots(ctx context.Context, from, to time.Time, limit int) ([]Hotspot, error) {
	if limit <= 0 {
		limit = 5
	}

	rows, err := s.pool.Query(ctx, `
SELECT
  site_id,
  rack_id,
  miner_id,
  COALESCE(MAX(temp_celsius), 0) AS max_temp_celsius,
  COALESCE(AVG(temp_celsius), 0) AS avg_temp_celsius,
  COALESCE(AVG(hashrate_ths), 0) AS avg_hashrate_ths,
  COALESCE(AVG(efficiency_jth), 0) AS avg_efficiency_jth
FROM telemetry_readings
WHERE ts >= $1
  AND ts < $2
GROUP BY site_id, rack_id, miner_id
ORDER BY max_temp_celsius DESC
LIMIT $3
`, from.UTC(), to.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("query hotspots: %w", err)
	}
	defer rows.Close()

	items := make([]Hotspot, 0, limit)
	for rows.Next() {
		var item Hotspot
		if err := rows.Scan(
			&item.SiteID,
			&item.RackID,
			&item.MinerID,
			&item.MaxTempCelsius,
			&item.AvgTempCelsius,
			&item.AvgHashrateTHs,
			&item.AvgEfficiencyJTH,
		); err != nil {
			return nil, fmt.Errorf("scan hotspot: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate hotspots: %w", err)
	}
	return items, nil
}

func (s *Store) relationExists(ctx context.Context, name string) bool {
	var relation *string
	err := s.pool.QueryRow(ctx, `SELECT to_regclass($1)::text`, "public."+name).Scan(&relation)
	if err != nil {
		return false
	}
	return relation != nil && *relation != ""
}

func isUndefinedRelation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "42P01"
	}
	return false
}

func parseDate(raw string, location *time.Location) (time.Time, error) {
	date, err := time.ParseInLocation("2006-01-02", raw, location)
	if err != nil {
		return time.Time{}, err
	}
	return date, nil
}

func normalizeDate(date time.Time, location *time.Location) time.Time {
	local := date.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}

func (s *Store) WindowForDate(date time.Time, location *time.Location) (time.Time, time.Time) {
	normalized := normalizeDate(date, location)
	return normalized.UTC(), normalized.Add(24 * time.Hour).UTC()
}

func (s *Store) DateFromString(raw string, location *time.Location) (time.Time, error) {
	return parseDate(raw, location)
}

func closeRows(rows pgx.Rows) {
	if rows != nil {
		rows.Close()
	}
}

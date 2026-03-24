package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/alerts"
	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/efficiency"
	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/telemetry"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultQueryLimit  = 50
	maxQueryLimit      = 500
	defaultSeriesLimit = 720
	maxSeriesLimit     = 10000
	defaultEffLimit    = 100
	maxEffLimit        = 1000
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
	minerModel := parseMinerModel(tags)

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
    miner_model = EXCLUDED.miner_model,
    firmware_version = EXCLUDED.firmware_version,
    nominal_hashrate_ths = EXCLUDED.nominal_hashrate_ths,
    nominal_power_watts = EXCLUDED.nominal_power_watts,
    is_active = TRUE
`, request.MinerID, request.SiteID, request.RackID, minerModel, request.FirmwareVersion, nominalHashrate, nominalPower)
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

func (r *PostgresRepository) UpsertAlert(ctx context.Context, input alerts.PersistInput) (alerts.UpsertResult, error) {
	if strings.TrimSpace(input.RuleID) == "" {
		return alerts.UpsertResult{}, fmt.Errorf("rule_id is required")
	}
	if strings.TrimSpace(input.MinerID) == "" {
		return alerts.UpsertResult{}, fmt.Errorf("miner_id is required")
	}
	if strings.TrimSpace(input.EventID) == "" {
		return alerts.UpsertResult{}, fmt.Errorf("event_id is required")
	}
	if input.SuppressionWindow <= 0 {
		input.SuppressionWindow = 10 * time.Minute
	}
	if input.ObservedAt.IsZero() {
		input.ObservedAt = time.Now().UTC()
	} else {
		input.ObservedAt = input.ObservedAt.UTC()
	}

	details := input.Details
	if details == nil {
		details = map[string]any{}
	}
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return alerts.UpsertResult{}, fmt.Errorf("marshal alert details: %w", err)
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return alerts.UpsertResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var existing persistedAlertRow
	err = tx.QueryRow(ctx, `
SELECT
  alert_id,
  rule_id,
  rule_name,
  severity,
  status,
  message,
  fingerprint,
  dedupe_key,
  site_id,
  rack_id,
  miner_id,
  event_id::text,
  miner_model,
  firmware_version,
  metric_name,
  metric_value,
  threshold_value,
  first_seen_at,
  last_seen_at,
  suppression_until,
  occurrences,
  notify_count,
  last_notified_at,
  created_at,
  updated_at,
  details
FROM alerts
WHERE fingerprint = $1
  AND status IN ('open', 'suppressed')
ORDER BY last_seen_at DESC
LIMIT 1
FOR UPDATE
`, input.Fingerprint).Scan(
		&existing.AlertID,
		&existing.RuleID,
		&existing.RuleName,
		&existing.Severity,
		&existing.Status,
		&existing.Message,
		&existing.Fingerprint,
		&existing.DedupeKey,
		&existing.SiteID,
		&existing.RackID,
		&existing.MinerID,
		&existing.EventID,
		&existing.MinerModel,
		&existing.Firmware,
		&existing.MetricName,
		&existing.MetricValue,
		&existing.Threshold,
		&existing.FirstSeenAt,
		&existing.LastSeenAt,
		&existing.SuppressionUntil,
		&existing.Occurrences,
		&existing.NotifyCount,
		&existing.LastNotifiedAt,
		&existing.CreatedAt,
		&existing.UpdatedAt,
		&existing.DetailsRaw,
	)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return alerts.UpsertResult{}, fmt.Errorf("query active alert by fingerprint: %w", err)
	}

	suppressionUntil := input.ObservedAt.Add(input.SuppressionWindow)
	now := time.Now().UTC()

	if errors.Is(err, pgx.ErrNoRows) {
		row := persistedAlertRow{
			AlertID:          uuid.NewString(),
			RuleID:           input.RuleID,
			RuleName:         input.RuleName,
			Severity:         string(input.Severity),
			Status:           string(alerts.StatusOpen),
			Message:          input.Message,
			Fingerprint:      input.Fingerprint,
			DedupeKey:        input.DedupeKey,
			SiteID:           input.SiteID,
			RackID:           input.RackID,
			MinerID:          input.MinerID,
			EventID:          input.EventID,
			MinerModel:       input.MinerModel,
			Firmware:         input.Firmware,
			MetricName:       input.MetricName,
			MetricValue:      input.MetricValue,
			Threshold:        input.Threshold,
			FirstSeenAt:      input.ObservedAt,
			LastSeenAt:       input.ObservedAt,
			SuppressionUntil: suppressionUntil,
			Occurrences:      1,
			NotifyCount:      0,
			CreatedAt:        now,
			UpdatedAt:        now,
			DetailsRaw:       detailsJSON,
		}

		_, err := tx.Exec(ctx, `
INSERT INTO alerts (
  alert_id,
  rule_id,
  rule_name,
  severity,
  status,
  message,
  fingerprint,
  dedupe_key,
  site_id,
  rack_id,
  miner_id,
  event_id,
  miner_model,
  firmware_version,
  metric_name,
  metric_value,
  threshold_value,
  first_seen_at,
  last_seen_at,
  suppression_until,
  occurrences,
  notify_count,
  details
)
VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8,
  $9, $10, $11, $12::uuid, $13, $14, $15, $16, $17,
  $18, $19, $20, $21, $22, $23
)
`, row.AlertID, row.RuleID, row.RuleName, row.Severity, row.Status, row.Message, row.Fingerprint,
			row.DedupeKey, row.SiteID, row.RackID, row.MinerID, row.EventID, row.MinerModel, row.Firmware,
			row.MetricName, row.MetricValue, row.Threshold, row.FirstSeenAt, row.LastSeenAt, row.SuppressionUntil,
			row.Occurrences, row.NotifyCount, row.DetailsRaw)
		if err != nil {
			return alerts.UpsertResult{}, fmt.Errorf("insert alert: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return alerts.UpsertResult{}, fmt.Errorf("commit alert insert tx: %w", err)
		}

		alert, err := row.toAlert()
		if err != nil {
			return alerts.UpsertResult{}, err
		}
		return alerts.UpsertResult{
			Alert:        alert,
			ShouldNotify: true,
			Suppressed:   false,
			IsNew:        true,
		}, nil
	}

	suppressed := input.ObservedAt.Before(existing.SuppressionUntil)
	status := alerts.StatusOpen
	if suppressed {
		status = alerts.StatusSuppressed
	}

	err = tx.QueryRow(ctx, `
UPDATE alerts
SET
  severity = $2,
  status = $3,
  message = $4,
  site_id = $5,
  rack_id = $6,
  miner_id = $7,
  event_id = $8::uuid,
  miner_model = $9,
  firmware_version = $10,
  metric_name = $11,
  metric_value = $12,
  threshold_value = $13,
  last_seen_at = $14,
  suppression_until = $15,
  occurrences = occurrences + 1,
  details = $16,
  updated_at = NOW()
WHERE alert_id = $1
RETURNING
  alert_id,
  rule_id,
  rule_name,
  severity,
  status,
  message,
  fingerprint,
  dedupe_key,
  site_id,
  rack_id,
  miner_id,
  event_id::text,
  miner_model,
  firmware_version,
  metric_name,
  metric_value,
  threshold_value,
  first_seen_at,
  last_seen_at,
  suppression_until,
  occurrences,
  notify_count,
  last_notified_at,
  created_at,
  updated_at,
  details
`,
		existing.AlertID,
		string(input.Severity),
		string(status),
		input.Message,
		input.SiteID,
		input.RackID,
		input.MinerID,
		input.EventID,
		input.MinerModel,
		input.Firmware,
		input.MetricName,
		input.MetricValue,
		input.Threshold,
		input.ObservedAt,
		suppressionUntil,
		detailsJSON,
	).Scan(
		&existing.AlertID,
		&existing.RuleID,
		&existing.RuleName,
		&existing.Severity,
		&existing.Status,
		&existing.Message,
		&existing.Fingerprint,
		&existing.DedupeKey,
		&existing.SiteID,
		&existing.RackID,
		&existing.MinerID,
		&existing.EventID,
		&existing.MinerModel,
		&existing.Firmware,
		&existing.MetricName,
		&existing.MetricValue,
		&existing.Threshold,
		&existing.FirstSeenAt,
		&existing.LastSeenAt,
		&existing.SuppressionUntil,
		&existing.Occurrences,
		&existing.NotifyCount,
		&existing.LastNotifiedAt,
		&existing.CreatedAt,
		&existing.UpdatedAt,
		&existing.DetailsRaw,
	)
	if err != nil {
		return alerts.UpsertResult{}, fmt.Errorf("update alert: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return alerts.UpsertResult{}, fmt.Errorf("commit alert update tx: %w", err)
	}

	persisted, err := existing.toAlert()
	if err != nil {
		return alerts.UpsertResult{}, err
	}

	return alerts.UpsertResult{
		Alert:        persisted,
		ShouldNotify: !suppressed,
		Suppressed:   suppressed,
		IsNew:        false,
	}, nil
}

func (r *PostgresRepository) RecordAlertNotification(ctx context.Context, record alerts.NotificationRecord) error {
	if strings.TrimSpace(record.AlertID) == "" {
		return fmt.Errorf("alert_id is required")
	}
	if strings.TrimSpace(string(record.Channel)) == "" {
		return fmt.Errorf("channel is required")
	}
	if strings.TrimSpace(record.Destination) == "" {
		return fmt.Errorf("destination is required")
	}
	if strings.TrimSpace(string(record.Status)) == "" {
		return fmt.Errorf("status is required")
	}
	if record.Attempt <= 0 {
		record.Attempt = 1
	}
	if record.NotifiedAt.IsZero() {
		record.NotifiedAt = time.Now().UTC()
	}

	payload := record.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal notification payload: %w", err)
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	_, err = tx.Exec(ctx, `
INSERT INTO alert_notifications (
  notification_id,
  alert_id,
  channel,
  destination,
  status,
  attempt,
  error_message,
  response_code,
  payload,
  sent_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, 0), $9, $10)
`,
		uuid.NewString(),
		record.AlertID,
		string(record.Channel),
		record.Destination,
		string(record.Status),
		record.Attempt,
		strings.TrimSpace(record.ErrorMessage),
		record.ResponseCode,
		payloadJSON,
		record.NotifiedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert alert notification: %w", err)
	}

	if record.Status == alerts.NotificationSent {
		_, err = tx.Exec(ctx, `
UPDATE alerts
SET
  notify_count = notify_count + 1,
  last_notified_at = $2,
  updated_at = NOW()
WHERE alert_id = $1
`, record.AlertID, record.NotifiedAt.UTC())
		if err != nil {
			return fmt.Errorf("update alert notify metadata: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit alert notification tx: %w", err)
	}

	return nil
}

type persistedAlertRow struct {
	AlertID          string
	RuleID           string
	RuleName         string
	Severity         string
	Status           string
	Message          string
	Fingerprint      string
	DedupeKey        string
	SiteID           string
	RackID           string
	MinerID          string
	EventID          string
	MinerModel       string
	Firmware         string
	MetricName       string
	MetricValue      float64
	Threshold        float64
	FirstSeenAt      time.Time
	LastSeenAt       time.Time
	SuppressionUntil time.Time
	Occurrences      int
	NotifyCount      int
	LastNotifiedAt   *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DetailsRaw       []byte
}

func (row persistedAlertRow) toAlert() (alerts.PersistedAlert, error) {
	details := map[string]any{}
	if len(row.DetailsRaw) > 0 {
		if err := json.Unmarshal(row.DetailsRaw, &details); err != nil {
			return alerts.PersistedAlert{}, fmt.Errorf("decode alert details: %w", err)
		}
	}

	return alerts.PersistedAlert{
		AlertID:          row.AlertID,
		RuleID:           row.RuleID,
		RuleName:         row.RuleName,
		Severity:         alerts.Severity(row.Severity),
		Status:           alerts.Status(row.Status),
		Message:          row.Message,
		Fingerprint:      row.Fingerprint,
		DedupeKey:        row.DedupeKey,
		SiteID:           row.SiteID,
		RackID:           row.RackID,
		MinerID:          row.MinerID,
		EventID:          row.EventID,
		MinerModel:       row.MinerModel,
		Firmware:         row.Firmware,
		MetricName:       row.MetricName,
		MetricValue:      row.MetricValue,
		Threshold:        row.Threshold,
		FirstSeenAt:      row.FirstSeenAt,
		LastSeenAt:       row.LastSeenAt,
		SuppressionUntil: row.SuppressionUntil,
		Occurrences:      row.Occurrences,
		NotifyCount:      row.NotifyCount,
		LastNotifiedAt:   row.LastNotifiedAt,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		Details:          details,
	}, nil
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
  tr.ts,
  tr.event_id::text,
  tr.site_id,
  tr.rack_id,
  tr.miner_id,
  m.miner_model,
  tr.firmware_version,
  tr.hashrate_ths,
  tr.power_watts,
  tr.temp_celsius,
  tr.fan_rpm,
  tr.efficiency_jth,
  tr.status,
  tr.load_pct,
  tr.tags,
  tr.ingested_at
FROM telemetry_readings tr
LEFT JOIN miners m ON m.miner_id = tr.miner_id
`
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 6)
	argPos := 1

	if filter.SiteID != "" {
		clauses = append(clauses, fmt.Sprintf("tr.site_id = $%d", argPos))
		args = append(args, filter.SiteID)
		argPos++
	}
	if filter.RackID != "" {
		clauses = append(clauses, fmt.Sprintf("tr.rack_id = $%d", argPos))
		args = append(args, filter.RackID)
		argPos++
	}
	if filter.MinerID != "" {
		clauses = append(clauses, fmt.Sprintf("tr.miner_id = $%d", argPos))
		args = append(args, filter.MinerID)
		argPos++
	}
	if filter.Model != "" {
		clauses = append(clauses, fmt.Sprintf("LOWER(COALESCE(m.miner_model, 'unknown')) = LOWER($%d)", argPos))
		args = append(args, filter.Model)
		argPos++
	}
	if filter.Status != "" {
		clauses = append(clauses, fmt.Sprintf("tr.status = $%d", argPos))
		args = append(args, string(filter.Status))
		argPos++
	}
	if filter.From != nil {
		clauses = append(clauses, fmt.Sprintf("tr.ts >= $%d", argPos))
		args = append(args, filter.From.UTC())
		argPos++
	}
	if filter.To != nil {
		clauses = append(clauses, fmt.Sprintf("tr.ts <= $%d", argPos))
		args = append(args, filter.To.UTC())
		argPos++
	}

	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	query += fmt.Sprintf(" ORDER BY tr.ts DESC LIMIT $%d", argPos)
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
			&item.MinerModel,
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
		argPos++
	}
	if filter.Model != "" {
		query += " AND miner_id IN (SELECT miner_id FROM miners WHERE LOWER(COALESCE(miner_model, 'unknown')) = LOWER($"
		query += strconv.Itoa(argPos)
		query += "))"
		args = append(args, filter.Model)
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

func (r *PostgresRepository) ListMinerSeries(ctx context.Context, filter MinerSeriesFilter) ([]MinerSeriesPoint, error) {
	if strings.TrimSpace(filter.MinerID) == "" {
		return nil, fmt.Errorf("miner_id is required")
	}
	if filter.To.Before(filter.From) {
		return nil, fmt.Errorf("from must be before to")
	}

	resolution := filter.Resolution
	if resolution != ResolutionMinute && resolution != ResolutionHour {
		resolution = ResolutionMinute
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultSeriesLimit
	}
	if limit > maxSeriesLimit {
		limit = maxSeriesLimit
	}

	viewName, bucketInterval := seriesSourceForResolution(resolution)
	query := fmt.Sprintf(`
SELECT
  bucket,
  samples,
  avg_hashrate_ths,
  avg_power_watts,
  avg_temp_celsius,
  max_temp_celsius,
  avg_fan_rpm,
  avg_efficiency_jth,
  critical_events
FROM %s
WHERE miner_id = $1
  AND bucket >= $2
  AND bucket <= $3
ORDER BY bucket ASC
LIMIT $4
`, viewName)

	rows, err := r.pool.Query(ctx, query, filter.MinerID, filter.From.UTC(), filter.To.UTC(), limit)
	if err != nil {
		if isUndefinedRelation(err) {
			return r.listMinerSeriesFromRaw(ctx, filter, bucketInterval, limit)
		}
		return nil, fmt.Errorf("query miner series from %s: %w", viewName, err)
	}
	defer rows.Close()

	points, err := scanMinerSeries(rows)
	if err != nil {
		return nil, err
	}

	// Fallback to raw telemetry when continuous aggregates are empty/unrefreshed.
	if len(points) == 0 {
		return r.listMinerSeriesFromRaw(ctx, filter, bucketInterval, limit)
	}

	return points, nil
}

func (r *PostgresRepository) ListMinerEfficiency(ctx context.Context, filter EfficiencyFilter) ([]MinerEfficiency, error) {
	windowMinutes, limit := normalizeEfficiencyWindowAndLimit(filter.WindowMinutes, filter.Limit)

	ambientExpr := `
CASE
  WHEN (tr.tags->>'ambient_temp_c') ~ '^-?[0-9]+(\.[0-9]+)?$' THEN (tr.tags->>'ambient_temp_c')::double precision
  WHEN (tr.tags->>'ambient_c') ~ '^-?[0-9]+(\.[0-9]+)?$' THEN (tr.tags->>'ambient_c')::double precision
  ELSE NULL
END
`

	query := `
SELECT
  tr.site_id,
  tr.rack_id,
  tr.miner_id,
  COALESCE(m.miner_model, 'unknown') AS miner_model,
  COUNT(*)::bigint AS samples,
  COALESCE(AVG(tr.hashrate_ths), 0) AS avg_hashrate_ths,
  COALESCE(AVG(tr.power_watts), 0) AS avg_power_watts,
  COALESCE(AVG(tr.temp_celsius), 0) AS avg_temp_celsius,
  COALESCE(AVG(` + ambientExpr + `), 25) AS avg_ambient_celsius,
  COALESCE(MAX(tr.ts), NOW()) AS last_seen_at
FROM telemetry_readings tr
LEFT JOIN miners m ON m.miner_id = tr.miner_id
WHERE tr.ts >= NOW() - ($1::int * INTERVAL '1 minute')
`
	args := []any{windowMinutes}
	argPos := 2

	if filter.SiteID != "" {
		query += fmt.Sprintf(" AND tr.site_id = $%d", argPos)
		args = append(args, filter.SiteID)
		argPos++
	}
	if filter.RackID != "" {
		query += fmt.Sprintf(" AND tr.rack_id = $%d", argPos)
		args = append(args, filter.RackID)
		argPos++
	}
	if filter.MinerID != "" {
		query += fmt.Sprintf(" AND tr.miner_id = $%d", argPos)
		args = append(args, filter.MinerID)
		argPos++
	}
	if filter.Model != "" {
		query += fmt.Sprintf(" AND LOWER(COALESCE(m.miner_model, 'unknown')) = LOWER($%d)", argPos)
		args = append(args, filter.Model)
		argPos++
	}

	query += `
GROUP BY tr.site_id, tr.rack_id, tr.miner_id, m.miner_model
`
	query += fmt.Sprintf(" ORDER BY last_seen_at DESC LIMIT $%d", argPos)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query miner efficiency: %w", err)
	}
	defer rows.Close()

	items := make([]MinerEfficiency, 0, limit)
	for rows.Next() {
		var item MinerEfficiency
		if err := rows.Scan(
			&item.SiteID,
			&item.RackID,
			&item.MinerID,
			&item.MinerModel,
			&item.Samples,
			&item.AvgHashrateTHs,
			&item.AvgPowerWatts,
			&item.AvgTempCelsius,
			&item.AvgAmbientCelsius,
			&item.LastSeenAt,
		); err != nil {
			return nil, fmt.Errorf("scan miner efficiency row: %w", err)
		}

		baseline := efficiency.BaselineForModel(item.MinerModel)
		item.RawJTH = efficiency.ComputeJTH(item.AvgPowerWatts, item.AvgHashrateTHs)
		item.CompensatedJTH = efficiency.CompensateJTH(item.RawJTH, item.AvgAmbientCelsius, baseline)
		item.BaselineJTH = baseline.NominalJTH
		item.DeltaPct = efficiency.DeltaPct(item.CompensatedJTH, baseline.NominalJTH)
		item.ThermalBand = efficiency.ClassifyThermalBand(item.AvgTempCelsius, baseline)

		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate miner efficiency rows: %w", err)
	}

	return items, nil
}

func (r *PostgresRepository) ListRackEfficiency(ctx context.Context, filter EfficiencyFilter) ([]RackEfficiency, error) {
	windowMinutes, limit := normalizeEfficiencyWindowAndLimit(filter.WindowMinutes, filter.Limit)

	ambientExpr := `
CASE
  WHEN (tr.tags->>'ambient_temp_c') ~ '^-?[0-9]+(\.[0-9]+)?$' THEN (tr.tags->>'ambient_temp_c')::double precision
  WHEN (tr.tags->>'ambient_c') ~ '^-?[0-9]+(\.[0-9]+)?$' THEN (tr.tags->>'ambient_c')::double precision
  ELSE NULL
END
`

	query := `
SELECT
  tr.site_id,
  tr.rack_id,
  COUNT(DISTINCT tr.miner_id)::bigint AS miners,
  COUNT(*)::bigint AS samples,
  COALESCE(AVG(tr.hashrate_ths), 0) AS avg_hashrate_ths,
  COALESCE(AVG(tr.power_watts), 0) AS avg_power_watts,
  COALESCE(AVG(tr.temp_celsius), 0) AS avg_temp_celsius,
  COALESCE(AVG(` + ambientExpr + `), 25) AS avg_ambient_celsius,
  COALESCE(MAX(tr.ts), NOW()) AS last_seen_at
FROM telemetry_readings tr
LEFT JOIN miners m ON m.miner_id = tr.miner_id
WHERE tr.ts >= NOW() - ($1::int * INTERVAL '1 minute')
`
	args := []any{windowMinutes}
	argPos := 2

	if filter.SiteID != "" {
		query += fmt.Sprintf(" AND tr.site_id = $%d", argPos)
		args = append(args, filter.SiteID)
		argPos++
	}
	if filter.RackID != "" {
		query += fmt.Sprintf(" AND tr.rack_id = $%d", argPos)
		args = append(args, filter.RackID)
		argPos++
	}
	if filter.Model != "" {
		query += fmt.Sprintf(" AND LOWER(COALESCE(m.miner_model, 'unknown')) = LOWER($%d)", argPos)
		args = append(args, filter.Model)
		argPos++
	}

	query += `
GROUP BY tr.site_id, tr.rack_id
`
	query += fmt.Sprintf(" ORDER BY last_seen_at DESC LIMIT $%d", argPos)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query rack efficiency: %w", err)
	}
	defer rows.Close()

	items := make([]RackEfficiency, 0, limit)
	for rows.Next() {
		var item RackEfficiency
		if err := rows.Scan(
			&item.SiteID,
			&item.RackID,
			&item.Miners,
			&item.Samples,
			&item.AvgHashrateTHs,
			&item.AvgPowerWatts,
			&item.AvgTempCelsius,
			&item.AvgAmbientCelsius,
			&item.LastSeenAt,
		); err != nil {
			return nil, fmt.Errorf("scan rack efficiency row: %w", err)
		}

		baseline := efficiency.BaselineForModel(filter.Model)
		item.RawJTH = efficiency.ComputeJTH(item.AvgPowerWatts, item.AvgHashrateTHs)
		item.CompensatedJTH = efficiency.CompensateJTH(item.RawJTH, item.AvgAmbientCelsius, baseline)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rack efficiency rows: %w", err)
	}

	return items, nil
}

func (r *PostgresRepository) ListSiteEfficiency(ctx context.Context, filter EfficiencyFilter) ([]SiteEfficiency, error) {
	windowMinutes, limit := normalizeEfficiencyWindowAndLimit(filter.WindowMinutes, filter.Limit)

	ambientExpr := `
CASE
  WHEN (tr.tags->>'ambient_temp_c') ~ '^-?[0-9]+(\.[0-9]+)?$' THEN (tr.tags->>'ambient_temp_c')::double precision
  WHEN (tr.tags->>'ambient_c') ~ '^-?[0-9]+(\.[0-9]+)?$' THEN (tr.tags->>'ambient_c')::double precision
  ELSE NULL
END
`

	query := `
SELECT
  tr.site_id,
  COUNT(DISTINCT tr.miner_id)::bigint AS miners,
  COUNT(DISTINCT tr.rack_id)::bigint AS racks,
  COUNT(*)::bigint AS samples,
  COALESCE(AVG(tr.hashrate_ths), 0) AS avg_hashrate_ths,
  COALESCE(AVG(tr.power_watts), 0) AS avg_power_watts,
  COALESCE(AVG(tr.temp_celsius), 0) AS avg_temp_celsius,
  COALESCE(AVG(` + ambientExpr + `), 25) AS avg_ambient_celsius,
  COALESCE(MAX(tr.ts), NOW()) AS last_seen_at
FROM telemetry_readings tr
LEFT JOIN miners m ON m.miner_id = tr.miner_id
WHERE tr.ts >= NOW() - ($1::int * INTERVAL '1 minute')
`
	args := []any{windowMinutes}
	argPos := 2

	if filter.SiteID != "" {
		query += fmt.Sprintf(" AND tr.site_id = $%d", argPos)
		args = append(args, filter.SiteID)
		argPos++
	}
	if filter.Model != "" {
		query += fmt.Sprintf(" AND LOWER(COALESCE(m.miner_model, 'unknown')) = LOWER($%d)", argPos)
		args = append(args, filter.Model)
		argPos++
	}

	query += `
GROUP BY tr.site_id
`
	query += fmt.Sprintf(" ORDER BY last_seen_at DESC LIMIT $%d", argPos)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query site efficiency: %w", err)
	}
	defer rows.Close()

	items := make([]SiteEfficiency, 0, limit)
	for rows.Next() {
		var item SiteEfficiency
		if err := rows.Scan(
			&item.SiteID,
			&item.Miners,
			&item.Racks,
			&item.Samples,
			&item.AvgHashrateTHs,
			&item.AvgPowerWatts,
			&item.AvgTempCelsius,
			&item.AvgAmbientCelsius,
			&item.LastSeenAt,
		); err != nil {
			return nil, fmt.Errorf("scan site efficiency row: %w", err)
		}

		baseline := efficiency.BaselineForModel(filter.Model)
		item.RawJTH = efficiency.ComputeJTH(item.AvgPowerWatts, item.AvgHashrateTHs)
		item.CompensatedJTH = efficiency.CompensateJTH(item.RawJTH, item.AvgAmbientCelsius, baseline)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate site efficiency rows: %w", err)
	}

	return items, nil
}

func (r *PostgresRepository) listMinerSeriesFromRaw(
	ctx context.Context,
	filter MinerSeriesFilter,
	bucketInterval string,
	limit int,
) ([]MinerSeriesPoint, error) {
	query := `
SELECT
  time_bucket($4::interval, ts) AS bucket,
  COUNT(*)::bigint AS samples,
  COALESCE(AVG(hashrate_ths), 0) AS avg_hashrate_ths,
  COALESCE(AVG(power_watts), 0) AS avg_power_watts,
  COALESCE(AVG(temp_celsius), 0) AS avg_temp_celsius,
  COALESCE(MAX(temp_celsius), 0) AS max_temp_celsius,
  COALESCE(AVG(fan_rpm), 0) AS avg_fan_rpm,
  COALESCE(AVG(efficiency_jth), 0) AS avg_efficiency_jth,
  COALESCE(SUM(CASE WHEN status IN ('critical', 'offline') THEN 1 ELSE 0 END), 0)::bigint AS critical_events
FROM telemetry_readings
WHERE miner_id = $1
  AND ts >= $2
  AND ts <= $3
GROUP BY bucket
ORDER BY bucket ASC
LIMIT $5
`

	rows, err := r.pool.Query(
		ctx,
		query,
		filter.MinerID,
		filter.From.UTC(),
		filter.To.UTC(),
		bucketInterval,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query miner series from telemetry_readings: %w", err)
	}
	defer rows.Close()

	points, err := scanMinerSeries(rows)
	if err != nil {
		return nil, err
	}
	return points, nil
}

func scanMinerSeries(rows pgx.Rows) ([]MinerSeriesPoint, error) {
	points := make([]MinerSeriesPoint, 0)
	for rows.Next() {
		var point MinerSeriesPoint
		if err := rows.Scan(
			&point.Bucket,
			&point.Samples,
			&point.AvgHashrateTHs,
			&point.AvgPowerWatts,
			&point.AvgTempCelsius,
			&point.MaxTempCelsius,
			&point.AvgFanRPM,
			&point.AvgEfficiencyJTH,
			&point.CriticalEvents,
		); err != nil {
			return nil, fmt.Errorf("scan miner series point: %w", err)
		}
		points = append(points, point)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate miner series rows: %w", err)
	}

	return points, nil
}

func seriesSourceForResolution(resolution BucketResolution) (viewName string, bucketInterval string) {
	switch resolution {
	case ResolutionHour:
		return "telemetry_agg_hour", "1 hour"
	default:
		return "telemetry_agg_minute", "1 minute"
	}
}

func isUndefinedRelation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "42P01"
	}
	return false
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

func normalizeEfficiencyWindowAndLimit(windowMinutes, limit int) (int, int) {
	if windowMinutes <= 0 {
		windowMinutes = 60
	}
	if windowMinutes > 24*60 {
		windowMinutes = 24 * 60
	}

	if limit <= 0 {
		limit = defaultEffLimit
	}
	if limit > maxEffLimit {
		limit = maxEffLimit
	}

	return windowMinutes, limit
}

func inferCountryCode(siteID string) string {
	parts := strings.Split(strings.ToLower(siteID), "-")
	if len(parts) >= 3 && len(parts[1]) == 2 {
		return strings.ToUpper(parts[1])
	}
	return "NA"
}

func parseMinerModel(tags map[string]string) string {
	candidates := []string{
		tags["asic_model"],
		tags["miner_model"],
		tags["model"],
	}

	for _, candidate := range candidates {
		value := strings.TrimSpace(candidate)
		if value == "" {
			continue
		}
		return strings.ToUpper(value)
	}

	return "unknown"
}

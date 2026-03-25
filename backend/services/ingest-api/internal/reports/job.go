package reports

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type JobConfig struct {
	Timezone     string
	Schedule     string
	OutputDir    string
	RunOnStartup bool
}

type Job struct {
	logger       *slog.Logger
	store        *Store
	location     *time.Location
	scheduleHour int
	scheduleMin  int
	outputDir    string
	runOnStartup bool
}

func NewJob(logger *slog.Logger, store *Store, cfg JobConfig) (*Job, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if store == nil {
		return nil, fmt.Errorf("reports store is required")
	}

	timezone := strings.TrimSpace(cfg.Timezone)
	if timezone == "" {
		timezone = "UTC"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("load timezone %s: %w", timezone, err)
	}

	hour, minute, err := parseSchedule(cfg.Schedule)
	if err != nil {
		return nil, err
	}

	return &Job{
		logger:       logger,
		store:        store,
		location:     location,
		scheduleHour: hour,
		scheduleMin:  minute,
		outputDir:    strings.TrimSpace(cfg.OutputDir),
		runOnStartup: cfg.RunOnStartup,
	}, nil
}

func (j *Job) RunLoop(ctx context.Context) error {
	if j.runOnStartup {
		if _, _, err := j.GeneratePreviousDay(ctx, time.Now()); err != nil {
			j.logger.Error("daily report startup generation failed", "error", err)
		}
	}

	for {
		next := NextRunAt(time.Now().In(j.location), j.scheduleHour, j.scheduleMin)
		waitDuration := time.Until(next)
		j.logger.Info(
			"daily report scheduler waiting",
			"next_run", next.Format(time.RFC3339),
			"wait", waitDuration.String(),
			"timezone", j.location.String(),
		)

		timer := time.NewTimer(waitDuration)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
			if _, _, err := j.GeneratePreviousDay(ctx, next); err != nil {
				j.logger.Error("daily report generation failed", "error", err)
			}
		}
	}
}

func (j *Job) GeneratePreviousDay(ctx context.Context, now time.Time) (Report, string, error) {
	localNow := now.In(j.location)
	target := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, j.location).AddDate(0, 0, -1)
	return j.GenerateForDate(ctx, target)
}

func (j *Job) GenerateForDate(ctx context.Context, date time.Time) (Report, string, error) {
	windowFrom, windowTo := j.store.WindowForDate(date, j.location)
	metrics, err := j.store.CollectMetrics(ctx, windowFrom, windowTo)
	if err != nil {
		return Report{}, "", fmt.Errorf("collect daily metrics: %w", err)
	}

	report := BuildReport(
		date,
		j.location,
		windowFrom,
		windowTo,
		metrics,
		time.Now().UTC(),
	)
	markdown := RenderExecutiveOperationalMarkdown(report)

	reportID, err := j.store.SaveDailyReport(ctx, report, markdown)
	if err != nil {
		return Report{}, "", fmt.Errorf("persist daily report: %w", err)
	}
	report.ReportID = reportID

	if err := j.persistMarkdown(report, markdown); err != nil {
		return Report{}, "", err
	}

	j.logger.Info(
		"daily report generated",
		"report_id", report.ReportID,
		"report_date", report.ReportDate,
		"timezone", report.Timezone,
		"samples", report.Global.Samples,
		"alerts_total", report.Alerts.Total,
		"changes_applied", report.Changes.Applied,
		"changes_rollback", report.Changes.RolledBack,
	)

	return report, markdown, nil
}

func (j *Job) persistMarkdown(report Report, markdown string) error {
	if j.outputDir == "" {
		return nil
	}

	if err := os.MkdirAll(j.outputDir, 0o755); err != nil {
		return fmt.Errorf("create reports output dir: %w", err)
	}

	filename := fmt.Sprintf(
		"daily_report_%s_%s.md",
		report.ReportDate,
		sanitizeFilename(report.Timezone),
	)
	path := filepath.Join(j.outputDir, filename)
	if err := os.WriteFile(path, []byte(markdown), 0o644); err != nil {
		return fmt.Errorf("write report markdown file: %w", err)
	}

	return nil
}

func parseSchedule(raw string) (int, int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = "08:00"
	}

	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid report schedule format, expected HH:MM")
	}

	hour, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("invalid report schedule hour")
	}

	minute, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("invalid report schedule minute")
	}

	return hour, minute, nil
}

func NextRunAt(now time.Time, hour, minute int) time.Time {
	candidate := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !candidate.After(now) {
		candidate = candidate.Add(24 * time.Hour)
	}
	return candidate
}

func sanitizeFilename(value string) string {
	replacer := strings.NewReplacer("/", "_", " ", "_", ":", "_")
	return replacer.Replace(strings.TrimSpace(value))
}

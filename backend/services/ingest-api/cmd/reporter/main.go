package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/observability"
	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/reports"
)

func main() {
	mode := flag.String("mode", envOrDefault("REPORT_MODE", "once"), "once|daemon")
	date := flag.String("date", envOrDefault("REPORT_DATE", ""), "date in YYYY-MM-DD format (only for mode=once)")
	timezone := flag.String("timezone", envOrDefault("REPORT_TIMEZONE", "UTC"), "IANA timezone (e.g. America/Santiago)")
	schedule := flag.String("schedule", envOrDefault("REPORT_SCHEDULE", "08:00"), "daemon schedule HH:MM in selected timezone")
	outputDir := flag.String("output-dir", envOrDefault("REPORT_OUTPUT_DIR", ""), "optional directory for markdown report snapshots")
	runOnStartup := flag.Bool("run-on-startup", envAsBool("REPORT_RUN_ON_STARTUP", true), "generate previous-day report on daemon startup")
	flag.Parse()

	logLevel := envOrDefault("LOG_LEVEL", "info")
	logger := observability.NewLogger(logLevel).With("service", "daily-reporter", "component", "reporter")

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := reports.NewStore(ctx, databaseURL)
	if err != nil {
		logger.Error("failed to initialize reports store", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	job, err := reports.NewJob(logger, store, reports.JobConfig{
		Timezone:     *timezone,
		Schedule:     *schedule,
		OutputDir:    *outputDir,
		RunOnStartup: *runOnStartup,
	})
	if err != nil {
		logger.Error("failed to initialize reports job", "error", err)
		os.Exit(1)
	}

	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "once":
		if err := runOnce(ctx, logger, job, *timezone, strings.TrimSpace(*date)); err != nil {
			logger.Error("failed to generate daily report", "error", err)
			os.Exit(1)
		}
	case "daemon":
		logger.Info(
			"starting daily report daemon",
			"timezone", *timezone,
			"schedule", *schedule,
			"run_on_startup", *runOnStartup,
		)
		if err := job.RunLoop(ctx); err != nil {
			logger.Error("daily report daemon stopped with error", "error", err)
			os.Exit(1)
		}
	default:
		logger.Error("invalid mode", "mode", *mode, "expected", "once|daemon")
		os.Exit(1)
	}
}

func runOnce(
	ctx context.Context,
	logger *slog.Logger,
	job *reports.Job,
	timezone string,
	rawDate string,
) error {
	if rawDate == "" {
		report, _, err := job.GeneratePreviousDay(ctx, time.Now())
		if err != nil {
			return err
		}
		logger.Info(
			"daily report generated (once)",
			"report_id", report.ReportID,
			"report_date", report.ReportDate,
			"timezone", report.Timezone,
		)
		return nil
	}

	location, err := time.LoadLocation(strings.TrimSpace(timezone))
	if err != nil {
		return fmt.Errorf("load timezone %s: %w", timezone, err)
	}

	targetDate, err := time.ParseInLocation("2006-01-02", rawDate, location)
	if err != nil {
		return fmt.Errorf("invalid date %s, expected YYYY-MM-DD: %w", rawDate, err)
	}

	report, _, err := job.GenerateForDate(ctx, targetDate)
	if err != nil {
		return err
	}
	logger.Info(
		"daily report generated (once)",
		"report_id", report.ReportID,
		"report_date", report.ReportDate,
		"timezone", report.Timezone,
	)
	return nil
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envAsBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

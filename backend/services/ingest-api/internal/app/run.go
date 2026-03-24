package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/alerts"
	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/httpapi"
	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/observability"
	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/processor"
	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/storage"
	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/telemetry"
	"github.com/gin-gonic/gin"
)

func Run(ctx context.Context, cfg Config) error {
	logger := observability.NewLogger(cfg.LogLevel)
	gin.SetMode(cfg.GinMode)

	if cfg.APIAuthEnabled && len(cfg.APIKeys) == 0 {
		return fmt.Errorf("api auth is enabled but API_KEYS is empty")
	}

	streamSubjects := []string{cfg.TelemetrySubject, cfg.TelemetryDLQSubject}
	publisher, err := telemetry.NewNATSPublisher(cfg.NATSURL, cfg.TelemetryStream, streamSubjects)
	if err != nil {
		return fmt.Errorf("connect nats: %w", err)
	}
	defer func() {
		if closeErr := publisher.Close(); closeErr != nil {
			logger.Error("failed to close nats publisher", "error", closeErr)
		}
	}()

	logger.Info(
		"nats publisher ready",
		"nats_url", cfg.NATSURL,
		"stream", cfg.TelemetryStream,
		"subject", cfg.TelemetrySubject,
		"dlq_subject", cfg.TelemetryDLQSubject,
	)

	var repository storage.Repository
	if cfg.DatabaseURL != "" {
		repository, err = storage.NewPostgresRepository(ctx, cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("connect postgres: %w", err)
		}
		defer repository.Close()
		logger.Info("postgres repository ready")
	} else {
		logger.Warn("database url not configured; persistence and read api disabled")
	}

	if repository != nil && cfg.ProcessorEnabled {
		var alertHandler processor.AlertHandler
		if cfg.AlertsEnabled {
			alertRepo, ok := any(repository).(alerts.Repository)
			if !ok {
				return fmt.Errorf("repository does not implement alerts repository interface")
			}

			notifiers := make([]alerts.Notifier, 0, 2)
			if cfg.AlertWebhookEnabled {
				webhookNotifier, err := alerts.NewWebhookNotifier(alerts.WebhookConfig{
					URL:        cfg.AlertWebhookURL,
					AuthHeader: cfg.AlertWebhookHeader,
					AuthToken:  cfg.AlertWebhookToken,
					Timeout:    cfg.AlertNotifyTimeout,
				})
				if err != nil {
					return fmt.Errorf("init webhook notifier: %w", err)
				}
				notifiers = append(notifiers, webhookNotifier)
			}
			if cfg.AlertEmailEnabled {
				emailNotifier, err := alerts.NewSMTPNotifier(alerts.SMTPConfig{
					Addr:          cfg.AlertSMTPAddr,
					Username:      cfg.AlertSMTPUsername,
					Password:      cfg.AlertSMTPPassword,
					From:          cfg.AlertEmailFrom,
					To:            cfg.AlertEmailTo,
					SubjectPrefix: cfg.AlertEmailSubject,
				})
				if err != nil {
					return fmt.Errorf("init smtp notifier: %w", err)
				}
				notifiers = append(notifiers, emailNotifier)
			}

			alertService, err := alerts.NewService(
				logger,
				alertRepo,
				alerts.ServiceConfig{
					Enabled:           cfg.AlertsEnabled,
					SuppressionWindow: cfg.AlertSuppressWindow,
					NotifyTimeout:     cfg.AlertNotifyTimeout,
					NotifyRetries:     cfg.AlertNotifyRetries,
					RetryBackoff:      cfg.AlertNotifyBackoff,
					QueueSize:         cfg.AlertQueueSize,
					WorkerCount:       cfg.AlertWorkerCount,
				},
				alerts.DefaultRules(),
				notifiers,
			)
			if err != nil {
				return fmt.Errorf("init alerts service: %w", err)
			}
			defer alertService.Close()
			alertHandler = alertService

			logger.Info(
				"alerts service ready",
				"enabled", cfg.AlertsEnabled,
				"suppress_window", cfg.AlertSuppressWindow.String(),
				"webhook_enabled", cfg.AlertWebhookEnabled,
				"email_enabled", cfg.AlertEmailEnabled,
				"notify_retries", cfg.AlertNotifyRetries,
				"queue_size", cfg.AlertQueueSize,
				"workers", cfg.AlertWorkerCount,
			)
		}

		consumerCfg := processor.Config{
			Subject:    cfg.TelemetrySubject,
			StreamName: cfg.TelemetryStream,
			Durable:    cfg.ConsumerDurable,
			DLQSubject: cfg.TelemetryDLQSubject,
			MaxDeliver: cfg.ProcessorMaxDeliver,
			RetryDelay: cfg.ProcessorRetryDelay,
		}

		consumer, err := processor.NewConsumer(logger, repository, alertHandler, cfg.NATSURL, consumerCfg)
		if err != nil {
			return fmt.Errorf("start telemetry processor: %w", err)
		}
		defer func() {
			if closeErr := consumer.Close(); closeErr != nil {
				logger.Error("failed to close telemetry processor", "error", closeErr)
			}
		}()

		if err := consumer.Start(); err != nil {
			return fmt.Errorf("subscribe telemetry processor: %w", err)
		}

		logger.Info(
			"telemetry processor ready",
			"stream", cfg.TelemetryStream,
			"subject", cfg.TelemetrySubject,
			"durable", cfg.ConsumerDurable,
			"dlq_subject", cfg.TelemetryDLQSubject,
			"max_deliver", cfg.ProcessorMaxDeliver,
			"retry_delay", cfg.ProcessorRetryDelay.String(),
		)
	}

	authConfig := httpapi.APIKeyAuthConfig{
		Enabled: cfg.APIAuthEnabled,
		Header:  cfg.APIKeyHeader,
		Keys:    cfg.APIKeys,
	}

	logger.Info(
		"http auth configuration",
		"enabled", cfg.APIAuthEnabled,
		"header", cfg.APIKeyHeader,
		"keys_count", len(cfg.APIKeys),
	)

	router := httpapi.NewRouter(
		logger,
		publisher,
		cfg.TelemetrySubject,
		repository,
		cfg.IngestMaxBodyBytes,
		authConfig,
	)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting ingest api", "port", cfg.Port, "mode", cfg.GinMode)
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		logger.Info("shutdown signal received")
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown ingest api: %w", err)
		}
		return nil
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("run ingest api: %w", err)
		}
		return nil
	}
}

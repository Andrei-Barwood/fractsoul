package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

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

	publisher, err := telemetry.NewNATSPublisher(cfg.NATSURL)
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
		"subject", cfg.TelemetrySubject,
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
		consumer, err := processor.NewConsumer(logger, repository, cfg.NATSURL, cfg.TelemetrySubject)
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

		logger.Info("telemetry processor ready", "subject", cfg.TelemetrySubject)
	}

	router := httpapi.NewRouter(logger, publisher, cfg.TelemetrySubject, repository)

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

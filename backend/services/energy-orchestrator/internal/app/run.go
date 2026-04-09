package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/contracts"
	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/events"
	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/fractsoul"
	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/httpapi"
	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/observability"
	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/service"
	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/storage"
	"github.com/gin-gonic/gin"
)

func Run(ctx context.Context, cfg Config) error {
	gin.SetMode(cfg.GinMode)

	logger := observability.NewLogger(cfg.LogLevel).With(
		"service", "energy-orchestrator",
		"component", "app",
	)

	repository, err := storage.NewPostgresRepository(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create postgres repository: %w", err)
	}
	defer repository.Close()

	var publisher events.Publisher = events.NewNoopPublisher()
	if cfg.EventsEnabled {
		natsPublisher, err := events.NewNATSPublisher(cfg.NATSURL, cfg.EnergyStream, []string{
			contracts.SubjectLoadBudgetUpdated,
			contracts.SubjectCurtailmentRecommended,
			contracts.SubjectDispatchRejected,
		})
		if err != nil {
			return fmt.Errorf("create nats publisher: %w", err)
		}
		publisher = natsPublisher
	}
	defer publisher.Close()

	fractsoulClient := fractsoul.NewClient(
		cfg.FractsoulAPIBaseURL,
		cfg.APIKeyHeader,
		cfg.FractsoulAPIKey,
		cfg.FractsoulAPITimeout,
	)

	appService := service.NewService(logger, repository, publisher, fractsoulClient)

	router := httpapi.NewRouter(
		logger,
		appService,
		httpapi.APIKeyAuthConfig{
			Enabled:     cfg.APIAuthEnabled,
			Header:      cfg.APIKeyHeader,
			Keys:        cfg.APIKeys,
			RBACEnabled: cfg.APIRBACEnabled,
			DefaultRole: cfg.APIDefaultRole,
			KeyRoles:    cfg.APIKeyRoles,
		},
		httpapi.RuntimeOptions{
			ContextRackLimit:     cfg.ContextRackLimit,
			ContextWindowMinutes: cfg.ContextWindowMinutes,
		},
	)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting http server", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		logger.Info("shutting down http server")
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		slog.Error("http server failed", "error", err)
		return err
	}
}

package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/observability"
	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/storage"
	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/telemetry"
	"github.com/gin-gonic/gin"
)

func NewRouter(
	logger *slog.Logger,
	publisher telemetry.Publisher,
	telemetrySubject string,
	repository storage.Repository,
	ingestMaxBodyBytes int64,
	authConfig APIKeyAuthConfig,
) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(RequestIDMiddleware())
	router.Use(AccessLogMiddleware(logger))

	dashboardFS := dashboardFileSystem()
	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/dashboard/")
	})
	router.GET("/dashboard", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/dashboard/")
	})
	router.StaticFS("/dashboard", dashboardFS)

	telemetryHandler := NewTelemetryHandler(logger, publisher, telemetrySubject, ingestMaxBodyBytes)
	telemetryReadHandler := NewTelemetryReadHandler(logger, repository)
	efficiencyHandler := NewEfficiencyHandler(logger, repository)
	anomalyHandler := NewAnomalyHandler(logger, repository)

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/metrics", gin.WrapH(observability.MetricsHandler()))

	v1 := router.Group("/v1")
	v1.Use(APIKeyAuthMiddleware(logger, authConfig))
	{
		v1.POST("/telemetry/ingest", telemetryHandler.Ingest)
		v1.GET("/telemetry/readings", telemetryReadHandler.Readings)
		v1.GET("/telemetry/summary", telemetryReadHandler.Summary)
		v1.GET("/telemetry/sites/:site_id/racks/:rack_id/readings", telemetryReadHandler.RackReadings)
		v1.GET("/telemetry/miners/:miner_id/timeseries", telemetryReadHandler.MinerTimeSeries)
		v1.GET("/efficiency/miners", efficiencyHandler.MinerEfficiency)
		v1.GET("/efficiency/racks", efficiencyHandler.RackEfficiency)
		v1.GET("/efficiency/sites", efficiencyHandler.SiteEfficiency)
		v1.GET("/anomalies/miners/:miner_id/analyze", anomalyHandler.AnalyzeMiner)
		v1.POST("/anomalies/miners/:miner_id/changes/apply", anomalyHandler.ApplyRecommendationChange)
		v1.POST("/anomalies/changes/:change_id/rollback", anomalyHandler.RollbackRecommendationChange)
		v1.GET("/anomalies/changes", anomalyHandler.ListRecommendationChanges)
	}

	return router
}

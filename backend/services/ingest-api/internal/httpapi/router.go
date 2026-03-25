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
		writeGroup := v1.Group("")
		writeGroup.Use(RequireRoles(logger, RoleOperator, RoleAdmin))
		{
			writeGroup.POST("/telemetry/ingest", telemetryHandler.Ingest)
		}

		readGroup := v1.Group("")
		readGroup.Use(RequireRoles(logger, RoleViewer, RoleOperator, RoleAdmin))
		{
			readGroup.GET("/telemetry/readings", telemetryReadHandler.Readings)
			readGroup.GET("/telemetry/summary", telemetryReadHandler.Summary)
			readGroup.GET("/telemetry/sites/:site_id/racks/:rack_id/readings", telemetryReadHandler.RackReadings)
			readGroup.GET("/telemetry/miners/:miner_id/timeseries", telemetryReadHandler.MinerTimeSeries)
			readGroup.GET("/efficiency/miners", efficiencyHandler.MinerEfficiency)
			readGroup.GET("/efficiency/racks", efficiencyHandler.RackEfficiency)
			readGroup.GET("/efficiency/sites", efficiencyHandler.SiteEfficiency)
			readGroup.GET("/anomalies/miners/:miner_id/analyze", anomalyHandler.AnalyzeMiner)
			readGroup.GET("/anomalies/changes", anomalyHandler.ListRecommendationChanges)
		}

		adminGroup := v1.Group("")
		adminGroup.Use(RequireRoles(logger, RoleAdmin))
		{
			adminGroup.POST("/anomalies/miners/:miner_id/changes/apply", anomalyHandler.ApplyRecommendationChange)
			adminGroup.POST("/anomalies/changes/:change_id/rollback", anomalyHandler.RollbackRecommendationChange)
		}
	}

	return router
}

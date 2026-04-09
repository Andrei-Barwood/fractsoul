package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/observability"
	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/service"
	"github.com/gin-gonic/gin"
)

type RuntimeOptions struct {
	ContextRackLimit     int
	ContextWindowMinutes int
}

func NewRouter(logger *slog.Logger, appService *service.Service, authConfig APIKeyAuthConfig, options RuntimeOptions) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(RequestIDMiddleware())
	router.Use(AccessLogMiddleware(logger))

	energyHandler := NewEnergyHandler(logger, appService, options)

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/metrics", gin.WrapH(observability.MetricsHandler()))

	v1 := router.Group("/v1")
	v1.Use(APIKeyAuthMiddleware(logger, authConfig))
	{
		readGroup := v1.Group("")
		readGroup.Use(RequireRoles(logger, RoleViewer, RoleOperator, RoleAdmin))
		{
			readGroup.GET("/energy/sites/:site_id/budget", energyHandler.SiteBudget)
		}

		writeGroup := v1.Group("")
		writeGroup.Use(RequireRoles(logger, RoleOperator, RoleAdmin))
		{
			writeGroup.POST("/energy/sites/:site_id/dispatch/validate", energyHandler.ValidateDispatch)
		}
	}

	return router
}

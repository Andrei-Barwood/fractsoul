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

	dashboardFS := dashboardFileSystem()
	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/dashboard/energy/")
	})
	router.GET("/dashboard", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/dashboard/energy/")
	})
	router.StaticFS("/dashboard/energy", dashboardFS)

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
			readGroup.GET("/energy/overview", energyHandler.CampusOverview)

			siteReadGroup := readGroup.Group("/energy/sites/:site_id")
			siteReadGroup.Use(RequireSiteAccess(logger))
			{
				siteReadGroup.GET("/budget", energyHandler.SiteBudget)
				siteReadGroup.GET("/operations", energyHandler.SiteOperationalView)
				siteReadGroup.GET("/constraints/active", energyHandler.ActiveConstraints)
				siteReadGroup.GET("/recommendations/pending", energyHandler.PendingRecommendations)
				siteReadGroup.GET("/recommendations/reviews", energyHandler.ListRecommendationReviews)
				siteReadGroup.GET("/actions/blocked", energyHandler.BlockedActions)
				siteReadGroup.GET("/explanations", energyHandler.DecisionExplanations)
				siteReadGroup.GET("/replay/historical", energyHandler.HistoricalReplay)
				siteReadGroup.GET("/pilot/shadow", energyHandler.ShadowPilot)
			}
		}

		writeGroup := v1.Group("")
		writeGroup.Use(RequireRoles(logger, RoleOperator, RoleAdmin))
		{
			siteWriteGroup := writeGroup.Group("/energy/sites/:site_id")
			siteWriteGroup.Use(RequireSiteAccess(logger))
			{
				siteWriteGroup.POST("/dispatch/validate", energyHandler.ValidateDispatch)
				siteWriteGroup.POST("/recommendations/reviews", energyHandler.ReviewRecommendation)
			}
		}
	}

	return router
}

package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/telemetry"
	"github.com/gin-gonic/gin"
)

func NewRouter(logger *slog.Logger, publisher telemetry.Publisher, telemetrySubject string) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(RequestIDMiddleware())
	router.Use(AccessLogMiddleware(logger))

	telemetryHandler := NewTelemetryHandler(logger, publisher, telemetrySubject)

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := router.Group("/v1")
	{
		v1.POST("/telemetry/ingest", telemetryHandler.Ingest)
	}

	return router
}

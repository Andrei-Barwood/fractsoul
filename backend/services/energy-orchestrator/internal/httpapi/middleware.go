package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/observability"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}

		c.Set(requestIDContextKey, requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)
		c.Next()
	}
}

func AccessLogMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		observability.IncHTTPInFlight()
		c.Next()
		observability.DecHTTPInFlight()

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		status := c.Writer.Status()
		duration := time.Since(startedAt)
		observability.ObserveHTTPRequest(c.Request.Method, path, status, duration)

		attributes := []any{
			"request_id", RequestID(c),
			"method", c.Request.Method,
			"path", path,
			"status", status,
			"duration_ms", duration.Milliseconds(),
			"client_ip", c.ClientIP(),
		}

		switch {
		case status >= http.StatusInternalServerError:
			logger.Error("http request completed", attributes...)
		case status >= http.StatusBadRequest:
			logger.Warn("http request completed", attributes...)
		default:
			logger.Info("http request completed", attributes...)
		}
	}
}

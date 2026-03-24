package httpapi

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type APIKeyAuthConfig struct {
	Enabled bool
	Header  string
	Keys    []string
}

func APIKeyAuthMiddleware(logger *slog.Logger, cfg APIKeyAuthConfig) gin.HandlerFunc {
	allowedKeys := make(map[string]struct{}, len(cfg.Keys))
	for _, key := range cfg.Keys {
		value := strings.TrimSpace(key)
		if value == "" {
			continue
		}
		allowedKeys[value] = struct{}{}
	}

	headerName := strings.TrimSpace(cfg.Header)
	if headerName == "" {
		headerName = "X-API-Key"
	}

	return func(c *gin.Context) {
		if !cfg.Enabled {
			c.Next()
			return
		}

		apiKey := strings.TrimSpace(c.GetHeader(headerName))
		if apiKey == "" {
			logger.Warn(
				"missing api key",
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
				"client_ip", c.ClientIP(),
			)
			WriteError(
				c,
				http.StatusUnauthorized,
				"unauthorized",
				"missing api key",
				map[string]string{"header": headerName},
			)
			return
		}

		if _, ok := allowedKeys[apiKey]; !ok {
			logger.Warn(
				"invalid api key",
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
				"client_ip", c.ClientIP(),
			)
			WriteError(
				c,
				http.StatusUnauthorized,
				"unauthorized",
				"invalid api key",
				map[string]string{"header": headerName},
			)
			return
		}

		c.Next()
	}
}

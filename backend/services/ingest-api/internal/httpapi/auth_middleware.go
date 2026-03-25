package httpapi

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

type APIKeyAuthConfig struct {
	Enabled     bool
	Header      string
	Keys        []string
	RBACEnabled bool
	DefaultRole string
	KeyRoles    map[string]string
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

	defaultRole := normalizeRole(cfg.DefaultRole)
	if defaultRole == "" {
		defaultRole = RoleAdmin
	}

	roleByKey := make(map[string]string, len(cfg.KeyRoles))
	for key, role := range cfg.KeyRoles {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		normalizedRole := normalizeRole(role)
		if normalizedRole == "" {
			continue
		}
		roleByKey[normalizedKey] = normalizedRole
	}

	return func(c *gin.Context) {
		if !cfg.Enabled {
			c.Set(authRoleContextKey, RoleAdmin)
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

		resolvedRole := defaultRole
		if cfg.RBACEnabled {
			if mappedRole, ok := roleByKey[apiKey]; ok {
				resolvedRole = mappedRole
			}
		}

		c.Set(authRoleContextKey, resolvedRole)
		c.Next()
	}
}

func RequireRoles(logger *slog.Logger, requiredRoles ...string) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}

	required := make([]string, 0, len(requiredRoles))
	for _, role := range requiredRoles {
		normalized := normalizeRole(role)
		if normalized == "" || slices.Contains(required, normalized) {
			continue
		}
		required = append(required, normalized)
	}

	return func(c *gin.Context) {
		if len(required) == 0 {
			c.Next()
			return
		}

		role := normalizeRole(PrincipalRole(c))
		if role == "" {
			role = RoleAdmin
		}

		if slices.Contains(required, role) {
			c.Next()
			return
		}

		logger.Warn(
			"forbidden by role policy",
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
			"role", role,
			"required_roles", strings.Join(required, ","),
		)
		WriteError(
			c,
			http.StatusForbidden,
			"forbidden",
			"insufficient role privileges",
			map[string]any{
				"required_roles": required,
				"current_role":   role,
			},
		)
	}
}

const (
	RoleViewer   = "viewer"
	RoleOperator = "operator"
	RoleAdmin    = "admin"
)

func normalizeRole(value string) string {
	role := strings.TrimSpace(strings.ToLower(value))
	switch role {
	case RoleViewer, RoleOperator, RoleAdmin:
		return role
	default:
		return ""
	}
}

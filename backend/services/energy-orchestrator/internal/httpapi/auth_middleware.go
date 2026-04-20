package httpapi

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

type APIKeyAuthConfig struct {
	Enabled         bool
	Header          string
	Keys            []string
	RBACEnabled     bool
	DefaultRole     string
	KeyRoles        map[string]string
	KeyPrincipalIDs map[string]string
	KeySiteScopes   map[string][]string
	KeyRackScopes   map[string][]string
}

func APIKeyAuthMiddleware(logger *slog.Logger, cfg APIKeyAuthConfig) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}

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

	principalByKey := make(map[string]string, len(cfg.KeyPrincipalIDs))
	for key, principalID := range cfg.KeyPrincipalIDs {
		normalizedKey := strings.TrimSpace(key)
		normalizedPrincipal := strings.TrimSpace(principalID)
		if normalizedKey == "" || normalizedPrincipal == "" {
			continue
		}
		principalByKey[normalizedKey] = normalizedPrincipal
	}

	siteScopesByKey := make(map[string][]string, len(cfg.KeySiteScopes))
	for key, scopes := range cfg.KeySiteScopes {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		siteScopesByKey[normalizedKey] = normalizeScopes(scopes)
	}

	rackScopesByKey := make(map[string][]string, len(cfg.KeyRackScopes))
	for key, scopes := range cfg.KeyRackScopes {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		rackScopesByKey[normalizedKey] = normalizeScopes(scopes)
	}

	return func(c *gin.Context) {
		if !cfg.Enabled {
			principal := AuthPrincipal{
				PrincipalID:    "local-admin",
				Role:           RoleAdmin,
				AllowedSiteIDs: []string{"*"},
				AllowedRackIDs: []string{"*"},
			}
			c.Set(authRoleContextKey, RoleAdmin)
			c.Set(authPrincipalContextKey, principal)
			c.Next()
			return
		}

		apiKey := strings.TrimSpace(c.GetHeader(headerName))
		if apiKey == "" {
			logger.Warn("missing api key", "path", c.Request.URL.Path, "method", c.Request.Method)
			WriteError(c, http.StatusUnauthorized, "unauthorized", "missing api key", map[string]string{"header": headerName})
			return
		}

		if _, ok := allowedKeys[apiKey]; !ok {
			logger.Warn("invalid api key", "path", c.Request.URL.Path, "method", c.Request.Method)
			WriteError(c, http.StatusUnauthorized, "unauthorized", "invalid api key", map[string]string{"header": headerName})
			return
		}

		resolvedRole := defaultRole
		if cfg.RBACEnabled {
			if mappedRole, ok := roleByKey[apiKey]; ok {
				resolvedRole = mappedRole
			}
		}

		resolvedPrincipalID := apiKey
		if mappedPrincipalID, ok := principalByKey[apiKey]; ok {
			resolvedPrincipalID = mappedPrincipalID
		}

		siteScopes := []string{"*"}
		if mappedScopes, ok := siteScopesByKey[apiKey]; ok && len(mappedScopes) > 0 {
			siteScopes = mappedScopes
		}
		rackScopes := []string{"*"}
		if mappedScopes, ok := rackScopesByKey[apiKey]; ok && len(mappedScopes) > 0 {
			rackScopes = mappedScopes
		}

		c.Set(authRoleContextKey, resolvedRole)
		c.Set(authPrincipalContextKey, AuthPrincipal{
			APIKey:         apiKey,
			PrincipalID:    resolvedPrincipalID,
			Role:           resolvedRole,
			AllowedSiteIDs: siteScopes,
			AllowedRackIDs: rackScopes,
		})
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

		logger.Warn("forbidden by role policy", "path", c.Request.URL.Path, "method", c.Request.Method, "role", role)
		WriteError(c, http.StatusForbidden, "forbidden", "insufficient role privileges", map[string]any{
			"required_roles": required,
			"current_role":   role,
		})
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

func RequireSiteAccess(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return func(c *gin.Context) {
		siteID := strings.TrimSpace(c.Param("site_id"))
		if siteID == "" {
			WriteError(c, http.StatusBadRequest, "validation_error", "site_id is required", nil)
			return
		}
		if siteAccessAllowed(PrincipalSiteScopes(c), siteID) {
			c.Next()
			return
		}

		logger.Warn("forbidden by site scope", "path", c.Request.URL.Path, "method", c.Request.Method, "site_id", siteID, "principal_id", PrincipalID(c))
		WriteError(c, http.StatusForbidden, "forbidden", "principal is not allowed for this site_id", map[string]any{
			"site_id":      siteID,
			"principal_id": PrincipalID(c),
		})
	}
}

func EnsureRackAccess(c *gin.Context, rackIDs []string) bool {
	allowed := PrincipalRackScopes(c)
	if len(rackIDs) == 0 || scopeAllows(allowed, "*") {
		return true
	}

	denied := make([]string, 0)
	seen := make(map[string]struct{}, len(rackIDs))
	for _, rackID := range rackIDs {
		normalizedRackID := strings.TrimSpace(rackID)
		if normalizedRackID == "" {
			continue
		}
		if _, exists := seen[normalizedRackID]; exists {
			continue
		}
		seen[normalizedRackID] = struct{}{}
		if !scopeAllows(allowed, normalizedRackID) {
			denied = append(denied, normalizedRackID)
		}
	}
	if len(denied) == 0 {
		return true
	}

	WriteError(c, http.StatusForbidden, "forbidden", "principal is not allowed for one or more rack_id values", map[string]any{
		"rack_ids":      denied,
		"principal_id":  PrincipalID(c),
		"allowed_racks": allowed,
	})
	return false
}

func siteAccessAllowed(allowed []string, siteID string) bool {
	return scopeAllows(allowed, siteID)
}

func scopeAllows(allowed []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return true
	}
	if len(allowed) == 0 {
		return true
	}
	for _, scope := range allowed {
		normalizedScope := strings.TrimSpace(scope)
		if normalizedScope == "" {
			continue
		}
		if normalizedScope == "*" || normalizedScope == target {
			return true
		}
	}
	return false
}

func normalizeScopes(values []string) []string {
	scopes := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		scope := strings.TrimSpace(value)
		if scope == "" {
			continue
		}
		if _, exists := seen[scope]; exists {
			continue
		}
		seen[scope] = struct{}{}
		scopes = append(scopes, scope)
	}
	return scopes
}

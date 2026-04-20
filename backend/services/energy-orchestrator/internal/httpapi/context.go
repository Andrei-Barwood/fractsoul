package httpapi

import "github.com/gin-gonic/gin"

const (
	requestIDContextKey     = "request_id"
	authRoleContextKey      = "auth_role"
	authPrincipalContextKey = "auth_principal"
)

type AuthPrincipal struct {
	APIKey         string   `json:"api_key,omitempty"`
	PrincipalID    string   `json:"principal_id"`
	Role           string   `json:"role"`
	AllowedSiteIDs []string `json:"allowed_site_ids,omitempty"`
	AllowedRackIDs []string `json:"allowed_rack_ids,omitempty"`
}

func RequestID(c *gin.Context) string {
	value, exists := c.Get(requestIDContextKey)
	if !exists {
		return ""
	}

	requestID, ok := value.(string)
	if !ok {
		return ""
	}

	return requestID
}

func PrincipalRole(c *gin.Context) string {
	principal := Principal(c)
	if principal.Role != "" {
		return principal.Role
	}

	value, exists := c.Get(authRoleContextKey)
	if !exists {
		return ""
	}

	role, ok := value.(string)
	if !ok {
		return ""
	}

	return role
}

func Principal(c *gin.Context) AuthPrincipal {
	value, exists := c.Get(authPrincipalContextKey)
	if !exists {
		return AuthPrincipal{}
	}

	principal, ok := value.(AuthPrincipal)
	if !ok {
		return AuthPrincipal{}
	}

	return principal
}

func PrincipalID(c *gin.Context) string {
	return Principal(c).PrincipalID
}

func PrincipalSiteScopes(c *gin.Context) []string {
	return Principal(c).AllowedSiteIDs
}

func PrincipalRackScopes(c *gin.Context) []string {
	return Principal(c).AllowedRackIDs
}

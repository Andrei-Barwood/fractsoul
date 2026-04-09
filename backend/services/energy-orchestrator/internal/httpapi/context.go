package httpapi

import "github.com/gin-gonic/gin"

const (
	requestIDContextKey = "request_id"
	authRoleContextKey  = "auth_role"
)

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

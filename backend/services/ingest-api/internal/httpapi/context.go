package httpapi

import "github.com/gin-gonic/gin"

const requestIDContextKey = "request_id"

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

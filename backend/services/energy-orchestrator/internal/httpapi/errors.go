package httpapi

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type ErrorResponse struct {
	RequestID string `json:"request_id"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Details   any    `json:"details,omitempty"`
}

func WriteError(c *gin.Context, status int, code, message string, details any) {
	response := ErrorResponse{
		RequestID: RequestID(c),
		Code:      code,
		Message:   message,
		Details:   details,
	}

	c.AbortWithStatusJSON(status, response)
}

func ValidationDetails(err error) any {
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		issues := make([]map[string]string, 0, len(validationErrors))
		for _, issue := range validationErrors {
			issues = append(issues, map[string]string{
				"field": strings.ToLower(issue.Field()),
				"rule":  issue.Tag(),
				"value": fmt.Sprintf("%v", issue.Value()),
			})
		}

		return issues
	}

	return err.Error()
}

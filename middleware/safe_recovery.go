package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"syscall"

	"github.com/gin-gonic/gin"
	"payroll-backend/observability"
)

const disconnectedKey = "request_disconnected"

// SafeRecovery never dumps a request, cookie, panic value or source-code lines.
func SafeRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if value := recover(); value != nil {
				if err, ok := value.(error); ok && (errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET)) {
					c.Set(disconnectedKey, true)
					c.Abort()
					return
				}
				slog.Error("request panic recovered", "request_id", observability.RequestID(c),
					"route", observability.Route(c), "panic_type", fmt.Sprintf("%T", value), "stack", string(debug.Stack()))
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Request could not be completed"})
			}
		}()
		c.Next()
	}
}

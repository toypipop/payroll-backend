package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireXMLHttpRequest blocks mutating requests that don't carry the X-Requested-With header.
// Browsers prevent cross-origin JS from setting custom headers, so this stops CSRF attacks
// without needing a stateful token.
func RequireXMLHttpRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			if c.GetHeader("X-Requested-With") != "XMLHttpRequest" {
				c.JSON(http.StatusForbidden, gin.H{"error": "CSRF check failed"})
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

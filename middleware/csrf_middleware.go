package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireXMLHttpRequest blocks mutating requests that don't carry the X-Requested-With header.
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

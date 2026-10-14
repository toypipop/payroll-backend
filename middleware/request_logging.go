package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"payroll-backend/observability"

	"github.com/gin-gonic/gin"
)

// RequestID attaches a server-generated correlation ID to every request and response.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID, err := newRequestID()
		if err != nil {
			observability.LogError("request ID generation failed", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Request could not be completed"})
			return
		}
		c.Set(observability.RequestIDKey, requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// RequestLogger writes one structured access log entry after the request has completed.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()
		status := c.Writer.Status()
		if disconnected, _ := c.Get(disconnectedKey); disconnected == true {
			status = 499
		}
		slog.Info("request completed",
			"request_id", observability.RequestID(c),
			"status", status,
			"method", observability.Method(c),
			"route", observability.Route(c),
			"duration", time.Since(startedAt).String(),
		)
	}
}

func newRequestID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

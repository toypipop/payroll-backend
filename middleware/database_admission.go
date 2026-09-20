package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// DatabaseAdmission reserves capacity for both the company-lock transaction and
// the domain query/transaction each request may hold. Apply before all API routes,
// including company status changes, which may wait for those company locks.
func DatabaseAdmission(maxOpen int) gin.HandlerFunc {
	if maxOpen < 2 {
		panic("database admission requires at least two connections")
	}
	slots := make(chan struct{}, maxOpen/2)
	return func(c *gin.Context) {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
			if c.Request.Context().Err() != nil {
				c.AbortWithStatus(http.StatusServiceUnavailable)
				return
			}
			c.Next()
		case <-c.Request.Context().Done():
			c.AbortWithStatus(http.StatusServiceUnavailable)
		case <-timer.C:
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Server is busy; please retry shortly"})
		}
	}
}

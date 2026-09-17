package observability

import "github.com/gin-gonic/gin"

const RequestIDKey = "request_id"

// RequestID returns the server-generated identifier attached to this request.
func RequestID(c *gin.Context) string {
	requestID, _ := c.Get(RequestIDKey)
	id, _ := requestID.(string)
	return id
}

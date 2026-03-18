package middleware

import (
	"net/http"
	"strconv"

	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

// PreventDeleteIfReferenced blocks deletion
func PreventDeleteIfReferenced(entity string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request == nil || c.Request.Method != http.MethodDelete {
			c.Next()
			return
		}

		idStr := c.Param("id")
		if idStr == "" {
			c.Next()
			return
		}
		parsed, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil || parsed == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			c.Abort()
			return
		}
		id := uint(parsed)

		// Resolve company from target record
		blocked, reason, checkErr := repository.CheckDeleteBlocked(entity, id, 0)
		if checkErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": checkErr.Error()})
			c.Abort()
			return
		}
		if blocked {
			c.JSON(http.StatusConflict, gin.H{"error": repository.ErrDeleteBlocked.Error(), "reason": reason})
			c.Abort()
			return
		}

		c.Next()
	}
}

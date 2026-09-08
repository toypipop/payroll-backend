package settings

import (
	"net/http"
	"strconv"

	"payroll-backend/handlers/httpx"

	"github.com/gin-gonic/gin"
)

// GetAuditLogs lists successful business changes within the data scope granted to this module.
func (h *Handler) GetAuditLogs(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	var companyID *uint
	if raw := c.Query("companies_id"); raw != "" {
		value, err := strconv.ParseUint(raw, 10, 0)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "companies_id must be a positive integer"})
			return
		}
		parsed := uint(value)
		companyID = &parsed
	}
	entries, err := h.AuditLog.List(scope, companyID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, entries)
}

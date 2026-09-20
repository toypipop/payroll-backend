package settings

import (
	"net/http"
	"strconv"

	"payroll-backend/handlers/httpx"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

// GetAuditLogs lists successful business changes within the data scope granted to this module.
func (h *Handler) GetAuditLogs(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	filter := repository.AuditLogQuery{}
	switch c.Query("scope") {
	case "":
	case string(repository.AuditLogScopeShared):
		filter.Scope = repository.AuditLogScopeShared
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "scope must be shared"})
		return
	}
	if raw := c.Query("companies_id"); raw != "" {
		if filter.Scope == repository.AuditLogScopeShared {
			c.JSON(http.StatusBadRequest, gin.H{"error": "companies_id cannot be combined with shared scope"})
			return
		}
		value, err := strconv.ParseUint(raw, 10, 0)
		if err != nil || value == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "companies_id must be a positive integer"})
			return
		}
		parsed := uint(value)
		filter.CompanyID = &parsed
	}
	query, ok := httpx.ParsePage(c)
	if !ok {
		return
	}
	entries, err := h.AuditLog.List(scope, filter, query)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, entries)
}

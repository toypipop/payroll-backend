package payrollconfig

import (
	"net/http"

	"payroll-backend/handlers/httpx"

	"github.com/gin-gonic/gin"
)

// GetConfigByPayrollCycle returns the configuration in force for a cycle's company.
func (h *Handler) GetConfigByPayrollCycle(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	cycleID, ok := httpx.ParseID(c, "cycle_id", "payroll cycle")
	if !ok {
		return
	}

	result, err := h.PayrollConfig.ForCycle(scope, cycleID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config":       result.Config,
		"cycle_id":     result.CycleID,
		"companies_id": result.CompaniesID,
	})
}

package payroll

import (
	"net/http"
	"time"

	"payroll-backend/handlers/httpx"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetPayrollCycles(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	cycles, err := h.PayrollRun.Cycles(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, cycles)
}

func (h *Handler) CreatePayrollCycle(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input struct {
		CycleName   string    `json:"cycle_name" binding:"required"`
		StartDate   time.Time `json:"start_date" binding:"required"`
		EndDate     time.Time `json:"end_date" binding:"required"`
		CompaniesID uint      `json:"companies_id"`
	}
	if !httpx.BindJSON(c, &input) {
		return
	}

	cycle, err := h.PayrollRun.CreateCycle(scope, input.CycleName, input.StartDate, input.EndDate, input.CompaniesID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, cycle)
}

// ClosePayrollCycle snapshots the cycle's transactions and then closes it, so a closed cycle always carries the figures it was closed on.
func (h *Handler) ClosePayrollCycle(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	cycleID, ok := httpx.ParseID(c, "id", "payroll cycle")
	if !ok {
		return
	}

	cycle, result, err := h.PayrollRun.Close(scope, cycleID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":            "Cycle closed successfully and transactions snapshotted",
		"data":               cycle,
		"processed_count":    result.ProcessedCount,
		"skipped_count":      result.SkippedCount,
		"skipped_employees":  result.SkippedNames,
		"prorated_employees": result.Prorated,
	})
}

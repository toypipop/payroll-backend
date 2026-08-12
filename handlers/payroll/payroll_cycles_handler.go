package payroll

import (
	"net/http"
	"time"

	"payroll-backend/handlers/httpx"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// payrollRunService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func payrollRunService() *service.PayrollRunService {
	return service.NewPayrollRunService(repository.NewPayrollRunRepository())
}

func GetPayrollCycles(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	cycles, err := payrollRunService().Cycles(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, cycles)
}

func CreatePayrollCycle(c *gin.Context) {
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

	cycle, err := payrollRunService().CreateCycle(scope, input.CycleName, input.StartDate, input.EndDate, input.CompaniesID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, cycle)
}

// ClosePayrollCycle snapshots the cycle's transactions and then closes it, so a
// closed cycle always carries the figures it was closed on.
func ClosePayrollCycle(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	cycleID, ok := httpx.ParseID(c, "id", "payroll cycle")
	if !ok {
		return
	}

	cycle, result, err := payrollRunService().Close(scope, cycleID)
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

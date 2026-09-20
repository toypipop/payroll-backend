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

	query, ok := httpx.ParsePage(c)
	if !ok {
		return
	}
	active, ok := httpx.ParseOptionalBool(c, "is_closed")
	if !ok {
		return
	}
	cycles, err := h.PayrollRun.CyclesPage(scope, query, active)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, cycles)
}

// GetPayrollCycleOptions serves the cycle calendar to pages other than Payroll Cycles — the salary forms
// need to know which periods are already closed, which is not a reason to require permission on that page.
// The company filter still applies: without CheckPermission the scope falls back to the caller's accessible companies.
func (h *Handler) GetPayrollCycleOptions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	cycles, err := h.PayrollRun.Cycles(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	type companyOption struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	}
	type option struct {
		ID          uint          `json:"id"`
		CycleName   string        `json:"cycle_name"`
		StartDate   time.Time     `json:"start_date"`
		EndDate     time.Time     `json:"end_date"`
		IsClosed    bool          `json:"is_closed"`
		CompaniesID uint          `json:"companies_id"`
		Companies   companyOption `json:"companies"`
	}
	options := make([]option, 0, len(cycles))
	for _, cycle := range cycles {
		options = append(options, option{
			ID:          cycle.ID,
			CycleName:   cycle.CycleName,
			StartDate:   cycle.StartDate,
			EndDate:     cycle.EndDate,
			IsClosed:    cycle.IsClosed,
			CompaniesID: cycle.CompaniesID,
			Companies:   companyOption{ID: cycle.CompaniesID, Name: cycle.Companies.Name},
		})
	}
	c.JSON(http.StatusOK, options)
}

func (h *Handler) CreatePayrollCycle(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	// The dates arrive as bare "2006-01-02" text. Binding them straight into time.Time would make the
	// caller decide the timezone, which is how the same table ended up holding two conventions; the service
	// reads them in the business timezone instead.
	var input struct {
		CycleName   string `json:"cycle_name" binding:"required"`
		StartDate   string `json:"start_date" binding:"required"`
		EndDate     string `json:"end_date" binding:"required"`
		CompaniesID uint   `json:"companies_id"`
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

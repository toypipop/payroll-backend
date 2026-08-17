package payroll

import (
	"net/http"

	"payroll-backend/handlers/httpx"

	"github.com/gin-gonic/gin"
)

// CalculatePayroll reruns the payroll engine for every active employee in a cycle.
func (h *Handler) CalculatePayroll(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	cycleID, ok := httpx.ParseID(c, "cycle_id", "payroll cycle")
	if !ok {
		return
	}

	cycle, result, err := h.PayrollRun.Recalculate(scope, cycleID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":            "Payroll calculation completed",
		"cycle_id":           cycle.ID,
		"processed_count":    result.ProcessedCount,
		"skipped_count":      result.SkippedCount,
		"skipped_employees":  result.SkippedNames,
		"prorated_employees": result.Prorated,
	})
}

// GetPayrollTransactions returns a summary table of all transactions for a cycle.
func (h *Handler) GetPayrollTransactions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	cycleID, ok := httpx.ParseID(c, "cycle_id", "payroll cycle")
	if !ok {
		return
	}

	cycle, transactions, err := h.PayrollRun.Transactions(scope, cycleID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"cycle": cycle, "transactions": transactions})
}

// GetPayrollSlip returns one employee's payslip.
func (h *Handler) GetPayrollSlip(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	transactionID, ok := httpx.ParseID(c, "transaction_id", "transaction")
	if !ok {
		return
	}

	cycle, transaction, company, err := h.PayrollRun.Slip(scope, transactionID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"cycle": cycle, "slip": transaction, "company": company})
}

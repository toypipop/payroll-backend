package payroll

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

type cycleSummary struct {
	CycleID        uint    `json:"cycle_id"`
	CycleName      string  `json:"cycle_name"`
	EmployeeCount  int     `json:"employee_count"`
	TotalEarning   float64 `json:"total_earning"`
	TotalDeduction float64 `json:"total_deduction"`
	SsoAmount      float64 `json:"sso_amount"`
	TaxAmount      float64 `json:"tax_amount"`
	NetPay         float64 `json:"net_pay"`
}

// toCycleSummary converts the service's totals into the wire shape.
func toCycleSummary(totals service.CycleTotals) cycleSummary {
	return cycleSummary{
		CycleID:        totals.CycleID,
		CycleName:      totals.CycleName,
		EmployeeCount:  totals.EmployeeCount,
		TotalEarning:   totals.TotalEarning,
		TotalDeduction: totals.TotalDeduction,
		SsoAmount:      totals.SsoAmount,
		TaxAmount:      totals.TaxAmount,
		NetPay:         totals.NetPay,
	}
}

// GetPayrollDiff compares a cycle's totals against the company's previous closed cycle.
func (h *Handler) GetPayrollDiff(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	cycleID, ok := httpx.ParseID(c, "cycle_id", "payroll cycle")
	if !ok {
		return
	}

	diff, err := h.PayrollRun.Diff(scope, cycleID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	var previous *cycleSummary
	if diff.Previous != nil {
		summary := toCycleSummary(*diff.Previous)
		previous = &summary
	}

	c.JSON(http.StatusOK, gin.H{
		"current":      toCycleSummary(diff.Current),
		"previous":     previous,
		"has_previous": diff.HasPrevious,
	})
}

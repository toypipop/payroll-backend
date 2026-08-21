package attendance

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

type AttendanceSummaryRow struct {
	EmployeesID   uint    `json:"employees_id"`
	EmployeesCode string  `json:"employees_code"`
	FullName      string  `json:"full_name"`
	CompanyName   string  `json:"company_name"`
	TotalLateMin  float64 `json:"total_late_minutes"`
	TotalAbsent   int     `json:"total_absent_days"`
	LateAmount    float64 `json:"late_amount"`
	AbsentAmount  float64 `json:"absent_amount"`
	TotalAmount   float64 `json:"total_amount"`
}

func (h *Handler) GetAttendanceSummary(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	window, err := service.ParseSummaryRange(c.Query("from"), c.Query("to"))
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	rows, err := h.AttendanceSummary.Deductions(scope, window)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	summary := make([]AttendanceSummaryRow, 0, len(rows))
	for _, row := range rows {
		summary = append(summary, AttendanceSummaryRow{
			EmployeesID:   row.EmployeesID,
			EmployeesCode: row.EmployeesCode,
			FullName:      row.FullName,
			CompanyName:   row.CompanyName,
			TotalLateMin:  row.TotalLateMin,
			TotalAbsent:   row.TotalAbsent,
			LateAmount:    row.LateAmount,
			AbsentAmount:  row.AbsentAmount,
			TotalAmount:   row.TotalAmount,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"from":    window.FromLabel(),
		"to":      window.ToLabel(),
		"summary": summary,
	})
}

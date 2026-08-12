package attendance

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

type EarningSummaryRow struct {
	EmployeesID          uint    `json:"employees_id"`
	EmployeesCode        string  `json:"employees_code"`
	FullName             string  `json:"full_name"`
	CompanyName          string  `json:"company_name"`
	TotalOvertimeMinutes float64 `json:"total_overtime_minutes"`
	TotalExtraDays       int     `json:"total_extra_days"`
	OvertimeAmount       float64 `json:"overtime_amount"`
	ExtraAmount          float64 `json:"extra_amount"`
	TotalAmount          float64 `json:"total_amount"`
}

func GetEarningSummary(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	window, err := service.ParseSummaryRange(c.Query("from"), c.Query("to"))
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	rows, err := attendanceSummaryService().Earnings(scope, window)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	summary := make([]EarningSummaryRow, 0, len(rows))
	for _, row := range rows {
		summary = append(summary, EarningSummaryRow{
			EmployeesID:          row.EmployeesID,
			EmployeesCode:        row.EmployeesCode,
			FullName:             row.FullName,
			CompanyName:          row.CompanyName,
			TotalOvertimeMinutes: row.TotalOvertimeMinutes,
			TotalExtraDays:       row.TotalExtraDays,
			OvertimeAmount:       row.OvertimeAmount,
			ExtraAmount:          row.ExtraAmount,
			TotalAmount:          row.TotalAmount,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"from":    window.FromLabel(),
		"to":      window.ToLabel(),
		"summary": summary,
	})
}

package payroll

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

type cycleSummary struct {
	CycleID       uint    `json:"cycle_id"`
	CycleName     string  `json:"cycle_name"`
	EmployeeCount int     `json:"employee_count"`
	TotalEarning  float64 `json:"total_earning"`
	TotalDeduction float64 `json:"total_deduction"`
	SsoAmount     float64 `json:"sso_amount"`
	TaxAmount     float64 `json:"tax_amount"`
	NetPay        float64 `json:"net_pay"`
}

type aggregateRow struct {
	EmployeeCount  int     `json:"employee_count"`
	TotalEarning   float64 `json:"total_earning"`
	TotalDeduction float64 `json:"total_deduction"`
	SsoAmount      float64 `json:"sso_amount"`
	TaxAmount      float64 `json:"tax_amount"`
	NetPay         float64 `json:"net_pay"`
}

func aggregateCycle(cycleID uint) cycleSummary {
	var row aggregateRow
	config.DB.Model(&models.PayrollTransaction{}).
		Select("COUNT(*) as employee_count, COALESCE(SUM(total_earning),0) as total_earning, COALESCE(SUM(total_deduction),0) as total_deduction, COALESCE(SUM(sso_amount),0) as sso_amount, COALESCE(SUM(tax_amount),0) as tax_amount, COALESCE(SUM(net_pay),0) as net_pay").
		Where("cycle_id = ? AND deleted_at = 0", cycleID).
		Scan(&row)
	return cycleSummary{
		CycleID:        cycleID,
		EmployeeCount:  row.EmployeeCount,
		TotalEarning:   row.TotalEarning,
		TotalDeduction: row.TotalDeduction,
		SsoAmount:      row.SsoAmount,
		TaxAmount:      row.TaxAmount,
		NetPay:         row.NetPay,
	}
}

func GetPayrollDiff(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	cycleID := c.Param("cycle_id")

	var cycle models.PayrollCycle
	if err := config.DB.Scopes(repository.ApplyContextScopes(c, currentUser, dataScope, "payroll_cycle.companies_id", "unused")).
		First(&cycle, cycleID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Payroll cycle not found"})
		return
	}

	current := aggregateCycle(cycle.ID)
	current.CycleName = cycle.CycleName

	// Find the most recent closed cycle for the same company before this cycle
	var prevCycle models.PayrollCycle
	hasPrevious := false
	if err := config.DB.
		Where("companies_id = ? AND is_closed = true AND end_date < ? AND deleted_at = 0", cycle.CompaniesID, cycle.StartDate).
		Order("end_date DESC").
		First(&prevCycle).Error; err == nil {
		hasPrevious = true
	}

	var previous *cycleSummary
	if hasPrevious {
		s := aggregateCycle(prevCycle.ID)
		s.CycleName = prevCycle.CycleName
		previous = &s
	}

	c.JSON(http.StatusOK, gin.H{
		"current":      current,
		"previous":     previous,
		"has_previous": hasPrevious,
	})
}

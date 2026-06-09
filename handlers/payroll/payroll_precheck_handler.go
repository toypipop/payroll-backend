package payroll

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

type preCheckIssue struct {
	Type      string   `json:"type"`
	Message   string   `json:"message,omitempty"`
	Employees []string `json:"employees,omitempty"`
}

func PreCheckPayroll(c *gin.Context) {
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

	var issues []preCheckIssue

	// 1. Payroll config must exist
	var configCount int64
	config.DB.Model(&models.PayrollConfig{}).
		Where("companies_id = ? AND deleted_at IS NULL", cycle.CompaniesID).
		Count(&configCount)
	if configCount == 0 {
		issues = append(issues, preCheckIssue{
			Type:    "no_payroll_config",
			Message: "No payroll config found — configure it in Payroll Setup before running",
		})
	}

	// 2. Get all active employees for this company
	var employees []models.Employees
	config.DB.
		Where("companies_id = ? AND is_active = ?", cycle.CompaniesID, true).
		Select("id, first_name, last_name, bank_id").
		Find(&employees)

	if len(employees) == 0 {
		issues = append(issues, preCheckIssue{
			Type:    "no_employees",
			Message: "No active employees found for this company",
		})
		c.JSON(http.StatusOK, gin.H{"ok": len(issues) == 0, "issues": issues})
		return
	}

	empIDs := make([]uint, len(employees))
	empByID := make(map[uint]models.Employees, len(employees))
	for i, e := range employees {
		empIDs[i] = e.ID
		empByID[e.ID] = e
	}

	// 3. Find employees WITH a salary record effective before cycle end
	type salaryRow struct {
		EmployeesID uint
	}
	var salaryRows []salaryRow
	config.DB.Model(&models.SalariesStructure{}).
		Select("DISTINCT employees_id").
		Where("employees_id IN ? AND effective_date <= ?", empIDs, cycle.EndDate).
		Scan(&salaryRows)

	hasSalary := make(map[uint]bool, len(salaryRows))
	for _, r := range salaryRows {
		hasSalary[r.EmployeesID] = true
	}

	var noSalaryNames []string
	for _, e := range employees {
		if !hasSalary[e.ID] {
			noSalaryNames = append(noSalaryNames, e.FirstName+" "+e.LastName)
		}
	}
	if len(noSalaryNames) > 0 {
		issues = append(issues, preCheckIssue{
			Type:      "no_salary",
			Message:   "These employees have no salary structure and will be skipped",
			Employees: noSalaryNames,
		})
	}

	// 4. Find employees with no bank account
	var noBankNames []string
	for _, e := range employees {
		if e.BankID == nil || *e.BankID == 0 {
			noBankNames = append(noBankNames, e.FirstName+" "+e.LastName)
		}
	}
	if len(noBankNames) > 0 {
		issues = append(issues, preCheckIssue{
			Type:      "no_bank",
			Message:   "These employees have no bank account assigned",
			Employees: noBankNames,
		})
	}

	c.JSON(http.StatusOK, gin.H{"ok": len(issues) == 0, "issues": issues})
}

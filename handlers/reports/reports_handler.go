package reports

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────

func resolveCurrentUser(c *gin.Context) (models.Employees, models.DataScope, bool) {
	userID, dataScope := repository.GetContextAuth(c)
	var u models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&u, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return u, dataScope, false
	}
	return u, dataScope, true
}

func fetchClosedCycle(c *gin.Context, user models.Employees, scope models.DataScope) (models.PayrollCycle, bool) {
	cycleID := c.Param("cycle_id")
	var cycle models.PayrollCycle
	if err := config.DB.
		Scopes(repository.ApplyScopes(user, scope, "payroll_cycle.companies_id", "unused")).
		Preload("Companies").
		First(&cycle, cycleID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Payroll cycle not found"})
		return cycle, false
	}
	if !cycle.IsClosed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This report is only available for closed payroll cycles"})
		return cycle, false
	}
	return cycle, true
}

// ─── Payslip (corrected: company comes from the cycle, not the current user) ─

// GetPayslipReport returns a single payslip for a closed cycle transaction.
// This is the canonical handler for GET /payslips/:transaction_id.
func GetPayslipReport(c *gin.Context) {
	user, scope, ok := resolveCurrentUser(c)
	if !ok {
		return
	}

	txID := c.Param("transaction_id")
	var tx models.PayrollTransaction
	if err := config.DB.
		Preload("Employees").
		Preload("Employees.Department").
		Preload("Employees.Position").
		Preload("Employees.Companies").
		Preload("Details").
		First(&tx, txID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	}

	var cycle models.PayrollCycle
	if err := config.DB.
		Scopes(repository.ApplyScopes(user, scope, "payroll_cycle.companies_id", "unused")).
		First(&cycle, tx.CycleID).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}
	if !cycle.IsClosed {
		c.JSON(http.StatusForbidden, gin.H{"error": "Payslips are only available for closed payroll cycles"})
		return
	}

	var company models.Companies
	config.DB.First(&company, cycle.CompaniesID)

	c.JSON(http.StatusOK, gin.H{"cycle": cycle, "slip": tx, "company": company})
}

// ─── Bank Transfer Report ─────────────────────────────────────────────────────

type BankTransferRow struct {
	EmployeesID   uint    `json:"employees_id"`
	EmployeesCode string  `json:"employees_code"`
	FirstName     string  `json:"first_name"`
	LastName      string  `json:"last_name"`
	BankName      string  `json:"bank_name"`
	AccountNo     string  `json:"account_no"`
	AccountName   string  `json:"account_name"`
	NetPay        float64 `json:"net_pay"`
	CompaniesName string  `json:"companies_name"`
}

// GetBankTransferReport returns aggregated Net Pay and bank account details for a closed cycle.
// Output is suitable for generating an iBusiness bank transfer file or a summary PDF.
func GetBankTransferReport(c *gin.Context) {
	user, scope, ok := resolveCurrentUser(c)
	if !ok {
		return
	}
	cycle, ok := fetchClosedCycle(c, user, scope)
	if !ok {
		return
	}

	var transactions []models.PayrollTransaction
	if err := config.DB.Where("cycle_id = ?", cycle.ID).
		Preload("Employees").
		Preload("Employees.Companies").
		Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var rows []BankTransferRow
	totalNet := 0.0

	for _, tx := range transactions {
		row := BankTransferRow{
			EmployeesID:   tx.EmployeesID,
			EmployeesCode: tx.Employees.EmployeesCode,
			FirstName:     tx.Employees.FirstName,
			LastName:      tx.Employees.LastName,
			NetPay:        tx.NetPay,
			CompaniesName: tx.Employees.Companies.Name,
		}

		var bank models.EmployeeBankAccount
		if err := config.DB.Where("employees_id = ? AND companies_id = ?", tx.EmployeesID, cycle.CompaniesID).
			First(&bank).Error; err == nil {
			row.BankName = bank.BankName
			row.AccountNo = bank.AccountNo
			row.AccountName = bank.AccountName
		}

		rows = append(rows, row)
		totalNet += tx.NetPay
	}

	c.JSON(http.StatusOK, gin.H{
		"cycle":         cycle,
		"company":       cycle.Companies,
		"rows":          rows,
		"total_net_pay": totalNet,
	})
}

// ─── Tax Forms ────────────────────────────────────────────────────────────────

type PND1Row struct {
	EmployeesCode string  `json:"employees_code"`
	FirstName     string  `json:"first_name"`
	LastName      string  `json:"last_name"`
	GrossIncome   float64 `json:"gross_income"`
	SSOAmount     float64 `json:"sso_amount"`
	TaxWithheld   float64 `json:"tax_withheld"`
	CompaniesName string  `json:"companies_name"`
}

// GetTaxFormsPND1 returns monthly PND.1 income-tax withholding data for a closed cycle.
func GetTaxFormsPND1(c *gin.Context) {
	user, scope, ok := resolveCurrentUser(c)
	if !ok {
		return
	}
	cycle, ok := fetchClosedCycle(c, user, scope)
	if !ok {
		return
	}

	var transactions []models.PayrollTransaction
	if err := config.DB.Where("cycle_id = ?", cycle.ID).
		Preload("Employees").
		Preload("Employees.Companies").
		Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var rows []PND1Row
	totalGross, totalTax, totalSSO := 0.0, 0.0, 0.0

	for _, tx := range transactions {
		rows = append(rows, PND1Row{
			EmployeesCode: tx.Employees.EmployeesCode,
			FirstName:     tx.Employees.FirstName,
			LastName:      tx.Employees.LastName,
			GrossIncome:   tx.TotalEarning,
			SSOAmount:     tx.SsoAmount,
			TaxWithheld:   tx.TaxAmount,
			CompaniesName: tx.Employees.Companies.Name,
		})
		totalGross += tx.TotalEarning
		totalTax += tx.TaxAmount
		totalSSO += tx.SsoAmount
	}

	c.JSON(http.StatusOK, gin.H{
		"cycle":       cycle,
		"company":     cycle.Companies,
		"rows":        rows,
		"total_gross": totalGross,
		"total_tax":   totalTax,
		"total_sso":   totalSSO,
	})
}

// GetTax50Tawi returns data for a single employee's 50 Tawi withholding tax certificate.
func GetTax50Tawi(c *gin.Context) {
	user, scope, ok := resolveCurrentUser(c)
	if !ok {
		return
	}

	txID := c.Param("transaction_id")
	var tx models.PayrollTransaction
	if err := config.DB.
		Preload("Employees").
		Preload("Employees.Companies").
		Preload("Employees.Department").
		Preload("Employees.Position").
		First(&tx, txID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	}

	var cycle models.PayrollCycle
	if err := config.DB.
		Scopes(repository.ApplyScopes(user, scope, "payroll_cycle.companies_id", "unused")).
		Preload("Companies").
		First(&cycle, tx.CycleID).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}
	if !cycle.IsClosed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "50 Tawi is only available for closed cycles"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"cycle":       cycle,
		"company":     cycle.Companies,
		"transaction": tx,
	})
}

// ─── Social Security (SSO) Report ────────────────────────────────────────────

type SSORow struct {
	EmployeesCode string  `json:"employees_code"`
	FirstName     string  `json:"first_name"`
	LastName      string  `json:"last_name"`
	Wages         float64 `json:"wages"`
	EmployeeSso   float64 `json:"employee_sso"`
	EmployerSso   float64 `json:"employer_sso"`
	CompaniesName string  `json:"companies_name"`
}

// GetSSOReport returns SSO Form 1-10 data for a closed cycle.
// Employer SSO: 5% match, capped at 750
func GetSSOReport(c *gin.Context) {
	user, scope, ok := resolveCurrentUser(c)
	if !ok {
		return
	}
	cycle, ok := fetchClosedCycle(c, user, scope)
	if !ok {
		return
	}

	var payrollConfig models.PayrollConfig
	config.DB.Where("companies_id = ?", cycle.CompaniesID).First(&payrollConfig)

	var transactions []models.PayrollTransaction
	if err := config.DB.Where("cycle_id = ?", cycle.ID).
		Preload("Employees").
		Preload("Employees.Companies").
		Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var rows []SSORow
	totalEmpSSO, totalEmprSSO := 0.0, 0.0

	for _, tx := range transactions {
		rows = append(rows, SSORow{
			EmployeesCode: tx.Employees.EmployeesCode,
			FirstName:     tx.Employees.FirstName,
			LastName:      tx.Employees.LastName,
			Wages:         tx.SalariesAtTime,
			EmployeeSso:   tx.SsoAmount,
			EmployerSso:   tx.SsoAmount, // Thai SSO: employer matches employee at same rate
			CompaniesName: tx.Employees.Companies.Name,
		})
		totalEmpSSO += tx.SsoAmount
		totalEmprSSO += tx.SsoAmount
	}

	c.JSON(http.StatusOK, gin.H{
		"cycle":              cycle,
		"company":            cycle.Companies,
		"sso_rate":           payrollConfig.SsoRate,
		"sso_max":            payrollConfig.SsoMaxAmount,
		"rows":               rows,
		"total_employee_sso": totalEmpSSO,
		"total_employer_sso": totalEmprSSO,
		"total_sso_combined": totalEmpSSO + totalEmprSSO,
	})
}

// ─── Payroll Analysis ─────────────────────────────────────────────────────────

type JournalEntry struct {
	Account string  `json:"account"`
	Debit   float64 `json:"debit"`
	Credit  float64 `json:"credit"`
}

// GetPayrollAnalysis returns the full payroll register with line-items and an accounting journal summary.
func GetPayrollAnalysis(c *gin.Context) {
	user, scope, ok := resolveCurrentUser(c)
	if !ok {
		return
	}
	cycle, ok := fetchClosedCycle(c, user, scope)
	if !ok {
		return
	}

	var transactions []models.PayrollTransaction
	if err := config.DB.Where("cycle_id = ?", cycle.ID).
		Preload("Employees").
		Preload("Employees.Companies").
		Preload("Employees.Department").
		Preload("Employees.Position").
		Preload("Details").
		Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	totalGross, totalOtherDeductions, totalSSO, totalTax, totalNet := 0.0, 0.0, 0.0, 0.0, 0.0
	for _, tx := range transactions {
		totalGross += tx.TotalEarning
		totalOtherDeductions += tx.TotalDeduction
		totalSSO += tx.SsoAmount
		totalTax += tx.TaxAmount
		totalNet += tx.NetPay
	}

	// Double-entry journal summary for the payroll run
	journal := []JournalEntry{
		{Account: "Salary & Wage Expense", Debit: totalGross, Credit: 0},
		{Account: "SSO Expense (Employer)", Debit: totalSSO, Credit: 0},
		{Account: "Net Pay Payable", Debit: 0, Credit: totalNet},
		{Account: "SSO Payable (Employee)", Debit: 0, Credit: totalSSO},
		{Account: "SSO Payable (Employer)", Debit: 0, Credit: totalSSO},
		{Account: "Tax Withholding Payable", Debit: 0, Credit: totalTax},
		{Account: "Other Deductions Payable", Debit: 0, Credit: totalOtherDeductions},
	}

	c.JSON(http.StatusOK, gin.H{
		"cycle":        cycle,
		"company":      cycle.Companies,
		"transactions": transactions,
		"summary": gin.H{
			"total_gross":             totalGross,
			"total_other_deductions":  totalOtherDeductions,
			"total_sso_employee":      totalSSO,
			"total_sso_employer":      totalSSO,
			"total_tax":               totalTax,
			"total_net":               totalNet,
		},
		"journal": journal,
	})
}

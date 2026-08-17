package reports

import (
	"net/http"

	"payroll-backend/handlers/httpx"

	"github.com/gin-gonic/gin"
)

// ─── Payslip ─────────────────────────────────────────────────────────────────

// GetPayslipReport returns a single payslip for a closed cycle transaction.
func (h *Handler) GetPayslipReport(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	transactionID, ok := httpx.ParseID(c, "transaction_id", "transaction")
	if !ok {
		return
	}

	report, err := h.Report.Payslip(scope, transactionID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"cycle": report.Cycle, "slip": report.Transaction, "company": report.Company})
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
func (h *Handler) GetBankTransferReport(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	cycleID, ok := httpx.ParseID(c, "cycle_id", "payroll cycle")
	if !ok {
		return
	}

	report, err := h.Report.BankTransfer(scope, cycleID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	var rows []BankTransferRow
	for _, row := range report.Rows {
		rows = append(rows, BankTransferRow{
			EmployeesID:   row.EmployeesID,
			EmployeesCode: row.EmployeesCode,
			FirstName:     row.FirstName,
			LastName:      row.LastName,
			BankName:      row.BankName,
			AccountNo:     row.AccountNo,
			AccountName:   row.AccountName,
			NetPay:        row.NetPay,
			CompaniesName: row.CompaniesName,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"cycle":         report.Cycle,
		"company":       report.Company,
		"rows":          rows,
		"total_net_pay": report.TotalNetPay,
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
func (h *Handler) GetTaxFormsPND1(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	cycleID, ok := httpx.ParseID(c, "cycle_id", "payroll cycle")
	if !ok {
		return
	}

	report, err := h.Report.PND1(scope, cycleID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	var rows []PND1Row
	for _, row := range report.Rows {
		rows = append(rows, PND1Row{
			EmployeesCode: row.EmployeesCode,
			FirstName:     row.FirstName,
			LastName:      row.LastName,
			GrossIncome:   row.GrossIncome,
			SSOAmount:     row.SSOAmount,
			TaxWithheld:   row.TaxWithheld,
			CompaniesName: row.CompaniesName,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"cycle":       report.Cycle,
		"company":     report.Company,
		"rows":        rows,
		"total_gross": report.TotalGross,
		"total_tax":   report.TotalTax,
		"total_sso":   report.TotalSSO,
	})
}

// GetTax50Tawi returns data for a single employee's 50 Tawi withholding tax certificate.
func (h *Handler) GetTax50Tawi(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	transactionID, ok := httpx.ParseID(c, "transaction_id", "transaction")
	if !ok {
		return
	}

	certificate, err := h.Report.TaxCertificate(scope, transactionID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"cycle":       certificate.Cycle,
		"company":     certificate.Company,
		"transaction": certificate.Transaction,
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
func (h *Handler) GetSSOReport(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	cycleID, ok := httpx.ParseID(c, "cycle_id", "payroll cycle")
	if !ok {
		return
	}

	report, err := h.Report.SSO(scope, cycleID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	var rows []SSORow
	for _, row := range report.Rows {
		rows = append(rows, SSORow{
			EmployeesCode: row.EmployeesCode,
			FirstName:     row.FirstName,
			LastName:      row.LastName,
			Wages:         row.Wages,
			EmployeeSso:   row.EmployeeSso,
			EmployerSso:   row.EmployerSso,
			CompaniesName: row.CompaniesName,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"cycle":              report.Cycle,
		"company":            report.Company,
		"sso_rate":           report.SsoRate,
		"sso_max":            report.SsoMax,
		"rows":               rows,
		"total_employee_sso": report.TotalEmployeeSso,
		"total_employer_sso": report.TotalEmployerSso,
		"total_sso_combined": report.TotalEmployeeSso + report.TotalEmployerSso,
	})
}

// ─── Payroll Analysis ─────────────────────────────────────────────────────────

type JournalEntry struct {
	Account string  `json:"account"`
	Debit   float64 `json:"debit"`
	Credit  float64 `json:"credit"`
}

// GetPayrollAnalysis returns the full payroll register with line-items and an accounting journal summary.
func (h *Handler) GetPayrollAnalysis(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	cycleID, ok := httpx.ParseID(c, "cycle_id", "payroll cycle")
	if !ok {
		return
	}

	report, err := h.Report.Analysis(scope, cycleID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	journal := make([]JournalEntry, 0, len(report.Journal))
	for _, entry := range report.Journal {
		journal = append(journal, JournalEntry{Account: entry.Account, Debit: entry.Debit, Credit: entry.Credit})
	}

	c.JSON(http.StatusOK, gin.H{
		"cycle":        report.Cycle,
		"company":      report.Company,
		"transactions": report.Transactions,
		"summary": gin.H{
			"total_gross":            report.TotalGross,
			"total_other_deductions": report.TotalOtherDeductions,
			"total_sso_employee":     report.TotalSso,
			"total_sso_employer":     report.TotalSso,
			"total_tax":              report.TotalTax,
			"total_net":              report.TotalNet,
		},
		"journal": journal,
	})
}

package service

import (
	"payroll-backend/models"
	"payroll-backend/repository"
)

// Re-exported so handlers import a single package.
var (
	ErrReportCycleNotFound   = repository.ErrReportCycleNotFound
	ErrReportCycleOpen       = repository.ErrReportCycleOpen
	ErrReportTxNotFound      = repository.ErrReportTxNotFound
	ErrReportAccessDenied    = repository.ErrReportAccessDenied
	ErrReportPayslipOpen     = repository.ErrReportPayslipOpen
	ErrReport50TawiCycleOpen = repository.ErrReport50TawiCycleOpen
)

// PayslipReport is one employee's payslip for a closed cycle.
type PayslipReport struct {
	Cycle       models.PayrollCycle
	Transaction models.PayrollTransaction
	Company     models.Companies
}

// BankTransferRow is one payment instruction.
type BankTransferRow struct {
	EmployeesID   uint
	EmployeesCode string
	FirstName     string
	LastName      string
	BankName      string
	AccountNo     string
	AccountName   string
	NetPay        float64
	CompaniesName string
}

// BankTransferReport is a closed cycle's payment file.
type BankTransferReport struct {
	Cycle       models.PayrollCycle
	Company     models.Companies
	Rows        []BankTransferRow
	TotalNetPay float64
}

// PND1Row is one employee's line on the monthly withholding return.
type PND1Row struct {
	TransactionID uint
	EmployeesCode string
	FirstName     string
	LastName      string
	GrossIncome   float64
	SSOAmount     float64
	TaxWithheld   float64
	CompaniesName string
}

// PND1Report is the monthly withholding return for a closed cycle.
type PND1Report struct {
	Cycle      models.PayrollCycle
	Company    models.Companies
	Rows       []PND1Row
	TotalGross float64
	TotalTax   float64
	TotalSSO   float64
}

// TaxCertificate is one employee's withholding tax certificate.
type TaxCertificate struct {
	Cycle       models.PayrollCycle
	Company     models.Companies
	Transaction models.PayrollTransaction
}

// SSORow is one employee's line on the social security return.
type SSORow struct {
	EmployeesCode string
	FirstName     string
	LastName      string
	Wages         float64
	EmployeeSso   float64
	EmployerSso   float64
	CompaniesName string
}

// SSOReport is the social security return for a closed cycle.
type SSOReport struct {
	Cycle            models.PayrollCycle
	Company          models.Companies
	SsoRate          float64
	SsoMax           float64
	Rows             []SSORow
	TotalEmployeeSso float64
	TotalEmployerSso float64
}

// JournalEntry is one line of the double-entry summary.
type JournalEntry struct {
	Account string
	Debit   float64
	Credit  float64
}

// AnalysisReport is the full register plus its accounting summary.
type AnalysisReport struct {
	Cycle                models.PayrollCycle
	Company              models.Companies
	Transactions         []models.PayrollTransaction
	TotalGross           float64
	TotalOtherDeductions float64
	TotalSso             float64
	TotalTax             float64
	TotalNet             float64
	Journal              []JournalEntry
}

// ReportService owns the reporting rules.
type ReportService struct {
	repo repository.ReportRepository
}

// NewReportService wires the service to a repository implementation.
func NewReportService(repo repository.ReportRepository) *ReportService {
	return &ReportService{repo: repo}
}

// Payslip returns one payslip.
func (s *ReportService) Payslip(scope repository.AccessScope, transactionID uint) (PayslipReport, error) {
	transaction, err := s.repo.FindPayslipTransaction(transactionID)
	if err != nil {
		return PayslipReport{}, err
	}

	cycle, err := s.repo.FindCycleScoped(scope, transaction.CycleID)
	if err != nil {
		return PayslipReport{}, ErrReportAccessDenied
	}
	if !cycle.IsClosed {
		return PayslipReport{}, ErrReportPayslipOpen
	}

	company, err := s.repo.FindCompany(cycle.CompaniesID)
	if err != nil {
		return PayslipReport{}, err
	}
	return PayslipReport{Cycle: cycle, Transaction: transaction, Company: company}, nil
}

// BankTransfer returns the payment instructions for a closed cycle.
func (s *ReportService) BankTransfer(scope repository.AccessScope, cycleID uint) (BankTransferReport, error) {
	cycle, err := s.repo.FindClosedCycleScoped(scope, cycleID)
	if err != nil {
		return BankTransferReport{}, err
	}
	transactions, err := s.repo.ListCycleTransactions(cycle.ID)
	if err != nil {
		return BankTransferReport{}, err
	}

	report := BankTransferReport{Cycle: cycle, Company: cycle.Companies}
	for _, transaction := range transactions {
		report.Rows = append(report.Rows, BankTransferRow{
			EmployeesID:   transaction.EmployeesID,
			EmployeesCode: transaction.Employees.EmployeesCode,
			FirstName:     transaction.Employees.FirstName,
			LastName:      transaction.Employees.LastName,
			BankName:      transaction.BankName,
			AccountNo:     transaction.BankAccountNo,
			AccountName:   transaction.BankAccountName,
			NetPay:        transaction.NetPay,
			CompaniesName: transaction.Employees.Companies.Name,
		})
		report.TotalNetPay += transaction.NetPay
	}
	return report, nil
}

// PND1 returns the monthly withholding return for a closed cycle.
func (s *ReportService) PND1(scope repository.AccessScope, cycleID uint) (PND1Report, error) {
	cycle, err := s.repo.FindClosedCycleScoped(scope, cycleID)
	if err != nil {
		return PND1Report{}, err
	}
	transactions, err := s.repo.ListCycleTransactions(cycle.ID)
	if err != nil {
		return PND1Report{}, err
	}

	report := PND1Report{Cycle: cycle, Company: cycle.Companies}
	for _, transaction := range transactions {
		report.Rows = append(report.Rows, PND1Row{
			TransactionID: transaction.ID,
			EmployeesCode: transaction.Employees.EmployeesCode,
			FirstName:     transaction.Employees.FirstName,
			LastName:      transaction.Employees.LastName,
			GrossIncome:   transaction.TotalEarning,
			SSOAmount:     transaction.SsoAmount,
			TaxWithheld:   transaction.TaxAmount,
			CompaniesName: transaction.Employees.Companies.Name,
		})
		report.TotalGross += transaction.TotalEarning
		report.TotalTax += transaction.TaxAmount
		report.TotalSSO += transaction.SsoAmount
	}
	return report, nil
}

// TaxCertificate returns one employee's withholding certificate.
func (s *ReportService) TaxCertificate(scope repository.AccessScope, transactionID uint) (TaxCertificate, error) {
	transaction, err := s.repo.FindTaxCertificateTransaction(transactionID)
	if err != nil {
		return TaxCertificate{}, err
	}

	cycle, err := s.repo.FindCycleWithCompanyScoped(scope, transaction.CycleID)
	if err != nil {
		return TaxCertificate{}, ErrReportAccessDenied
	}
	if !cycle.IsClosed {
		return TaxCertificate{}, ErrReport50TawiCycleOpen
	}
	return TaxCertificate{Cycle: cycle, Company: cycle.Companies, Transaction: transaction}, nil
}

// SSO returns the social security return for a closed cycle.
func (s *ReportService) SSO(scope repository.AccessScope, cycleID uint) (SSOReport, error) {
	cycle, err := s.repo.FindClosedCycleScoped(scope, cycleID)
	if err != nil {
		return SSOReport{}, err
	}
	payrollConfig, err := s.repo.FindPayrollConfig(cycle.CompaniesID)
	if err != nil {
		return SSOReport{}, err
	}
	transactions, err := s.repo.ListCycleTransactions(cycle.ID)
	if err != nil {
		return SSOReport{}, err
	}

	report := SSOReport{
		Cycle:   cycle,
		Company: cycle.Companies,
		SsoRate: payrollConfig.SsoRate,
		SsoMax:  payrollConfig.SsoMaxAmount,
	}
	for _, transaction := range transactions {
		report.Rows = append(report.Rows, SSORow{
			EmployeesCode: transaction.Employees.EmployeesCode,
			FirstName:     transaction.Employees.FirstName,
			LastName:      transaction.Employees.LastName,
			Wages:         ssoWages(transaction),
			EmployeeSso:   transaction.SsoAmount,
			EmployerSso:   transaction.SsoAmount,
			CompaniesName: transaction.Employees.Companies.Name,
		})
		report.TotalEmployeeSso += transaction.SsoAmount
		report.TotalEmployerSso += transaction.SsoAmount
	}
	return report, nil
}

// Analysis returns the full register for a closed cycle with its journal summary.
func (s *ReportService) Analysis(scope repository.AccessScope, cycleID uint) (AnalysisReport, error) {
	cycle, err := s.repo.FindClosedCycleScoped(scope, cycleID)
	if err != nil {
		return AnalysisReport{}, err
	}
	transactions, err := s.repo.ListCycleTransactionsDetailed(cycle.ID)
	if err != nil {
		return AnalysisReport{}, err
	}

	report := AnalysisReport{Cycle: cycle, Company: cycle.Companies, Transactions: transactions}
	for _, transaction := range transactions {
		report.TotalGross += transaction.TotalEarning
		report.TotalOtherDeductions += transaction.TotalDeduction
		report.TotalSso += transaction.SsoAmount
		report.TotalTax += transaction.TaxAmount
		report.TotalNet += transaction.NetPay
	}

	report.Journal = []JournalEntry{
		{Account: "Salary & Wage Expense", Debit: report.TotalGross},
		{Account: "SSO Expense (Employer)", Debit: report.TotalSso},
		{Account: "Net Pay Payable", Credit: report.TotalNet},
		{Account: "SSO Payable (Employee)", Credit: report.TotalSso},
		{Account: "SSO Payable (Employer)", Credit: report.TotalSso},
		{Account: "Tax Withholding Payable", Credit: report.TotalTax},
		{Account: "Other Deductions Payable", Credit: report.TotalOtherDeductions},
	}
	return report, nil
}

// ssoWages is the wage base the SSO return reports: the base the contribution was priced on (BR-114). Cycles run before it was stored have none, so they fall back to the base salary.
func ssoWages(transaction models.PayrollTransaction) float64 {
	if transaction.SsoWageBase == nil {
		return transaction.SalariesAtTime
	}
	return *transaction.SsoWageBase
}

package repository

import (
	"database/sql"
	"payroll-backend/models"
	"time"

	"gorm.io/gorm"
)

type TransactionSummary struct {
	EmployeeCount  int64      `json:"employee_count"`
	TotalEarning   float64    `json:"total_earning"`
	TotalDeduction float64    `json:"total_deduction"`
	NetPay         float64    `json:"net_pay"`
	CalculatedAt   *time.Time `json:"calculated_at"`
}

func (r *gormPayrollRunRepository) ListTransactionsForCycle(cycleID uint, q PageQuery) (Page[models.PayrollTransaction], TransactionSummary, error) {
	var page Page[models.PayrollTransaction]
	var summary TransactionSummary
	// A Calculate commit between reads must not mix old totals with new rows.
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var err error
		page, summary, err = (&gormPayrollRunRepository{db: tx}).transactionPage(cycleID, q)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return page, summary, err
}

func (r *gormPayrollRunRepository) transactionPage(cycleID uint, q PageQuery) (Page[models.PayrollTransaction], TransactionSummary, error) {
	var summary TransactionSummary
	name := "concat_ws(' ', e.first_name, e.last_name)"
	order, err := pageOrder(q, map[string]string{
		"employee_name": name, "employees.first_name": name,
		"companies_name": "c.name", "employees.companies.name": "c.name",
		"department": "payroll_transaction.department_name", "position": "payroll_transaction.position_name", "employees.position.pos_name": "payroll_transaction.position_name",
		"salaries_at_time": "payroll_transaction.salaries_at_time", "total_earning": "payroll_transaction.total_earning", "total_deduction": "payroll_transaction.total_deduction",
		"taxable_income": "payroll_transaction.taxable_income", "sso_amount": "payroll_transaction.sso_amount", "tax_amount": "payroll_transaction.tax_amount", "net_pay": "payroll_transaction.net_pay",
	}, "payroll_transaction.id", "payroll_transaction.id")
	if err != nil {
		return Page[models.PayrollTransaction]{}, summary, err
	}
	base := r.db.Model(&models.PayrollTransaction{}).Where("payroll_transaction.cycle_id = ?", cycleID)
	if err := base.Session(&gorm.Session{}).Select("COUNT(*) AS employee_count, COALESCE(SUM(total_earning),0) AS total_earning, COALESCE(SUM(total_deduction),0) AS total_deduction, COALESCE(SUM(net_pay),0) AS net_pay, MAX(updated_at) AS calculated_at").Scan(&summary).Error; err != nil {
		return Page[models.PayrollTransaction]{}, summary, err
	}
	query := base.Joins("LEFT JOIN employees e ON e.id = payroll_transaction.employees_id AND e.deleted_at = 0").
		Joins("LEFT JOIN companies c ON c.id = e.companies_id AND c.deleted_at = 0")
	if q.Search != "" {
		pattern := searchPattern(q.Search)
		query = query.Where("("+name+" ILIKE ? OR e.employees_code ILIKE ? OR c.name ILIKE ? OR payroll_transaction.position_name ILIKE ? OR payroll_transaction.department_name ILIKE ? OR CAST(payroll_transaction.net_pay AS TEXT) ILIKE ? OR CAST(payroll_transaction.total_earning AS TEXT) ILIKE ? OR CAST(payroll_transaction.total_deduction AS TEXT) ILIKE ? OR CAST(payroll_transaction.salaries_at_time AS TEXT) ILIKE ? OR CAST(payroll_transaction.taxable_income AS TEXT) ILIKE ? OR CAST(payroll_transaction.sso_amount AS TEXT) ILIKE ? OR CAST(payroll_transaction.tax_amount AS TEXT) ILIKE ?)", pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern)
	}
	query = query.Preload("Employees", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, companies_id, first_name, last_name, employees_code")
	}).Preload("Employees.Companies").Preload("Details")
	page, err := readPage[models.PayrollTransaction](query, q, order)
	return page, summary, err
}

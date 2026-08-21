package repository

import (
	"errors"
	"time"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// Payroll-run failures.
var (
	ErrPayrollCycleMissing       = apperr.NotFound("Payroll cycle not found")
	ErrPayrollCycleIsClosed      = apperr.Validation("Cannot recalculate a closed cycle")
	ErrPayrollCycleAlreadyClosed = apperr.Validation("Cycle is already closed")
	ErrPayrollTransactionMissing = apperr.NotFound("Transaction not found")
	// ErrPayrollSalaryMissing is the one lookup failure a run may treat as "skip this employee"; anything else is a real failure.
	ErrPayrollSalaryMissing = apperr.NotFound("No salary structure found for this employee")
	ErrPayrollSlipDenied    = apperr.Forbidden("Access denied")
	ErrPayrollSlipCycleOpen = apperr.Forbidden("Payslips are only available for closed payroll cycles")
)

// TransactionSnapshot is the set of computed values a run writes onto one employee's transaction row.
type TransactionSnapshot struct {
	SalariesStructureID uint
	SalariesAtTime      float64
	TotalEarning        float64
	TotalDeduction      float64
	SsoAmount           float64
	TaxableIncome       float64
	TaxAmount           float64
	NetPay              float64
	PositionName        string
	DepartmentName      string
	EmploymentTypeName  string
	BankName            string
	BankAccountName     string
	BankAccountNo       string
}

// PayrollRunRepository is the persistence boundary for calculating a payroll cycle.
type PayrollRunRepository interface {
	FindCycleScoped(scope AccessScope, cycleID uint) (models.PayrollCycle, error)
	FindOpenCycleScoped(scope AccessScope, cycleID uint) (models.PayrollCycle, error)
	ListCyclesScoped(scope AccessScope) ([]models.PayrollCycle, error)
	CreateCycle(cycle *models.PayrollCycle) error
	MarkCycleClosed(cycle *models.PayrollCycle) error

	FindLatestPayrollConfig(companiesID uint) (models.PayrollConfig, error)
	ListActiveEmployees(companiesID uint) ([]models.Employees, error)
	FindAttendanceConfigForRun(companiesID uint) (models.AttendanceConfig, error)
	ListHolidaysInRange(companiesID uint, from time.Time, to time.Time) ([]models.Holiday, error)
	ListDayOffs(companiesID uint) ([]models.DayOff, error)

	FindActiveSalaryOnOrBefore(employeeID uint, date time.Time) (models.SalariesStructure, error)
	FindEarliestActiveSalaryOnOrBefore(employeeID uint, date time.Time) (models.SalariesStructure, error)
	DeleteTransactionForEmployee(cycleID uint, employeeID uint) error

	ListEmployeeEarnings(employeeID uint) ([]models.EmployeeEarning, error)
	ListCycleEarnings(employeeID uint, cycleID uint) ([]models.PayrollCycleEarning, error)
	ListEmployeeDeductions(employeeID uint) ([]models.EmployeeDeduction, error)
	ListCycleDeductions(employeeID uint, cycleID uint) ([]models.PayrollCycleDeduction, error)
	ListPairedTimeLogs(employeeID uint, from time.Time, to time.Time) ([]models.TimeLog, error)
	ListApprovedLeavesInRange(employeeID uint, cycleStart time.Time, cycleEnd time.Time) ([]models.LeaveRequest, error)

	SaveSnapshot(cycleID uint, employeeID uint, snapshot TransactionSnapshot, details []models.PayrollTransactionDetail) error

	CountLivePayrollConfigs(companiesID uint) (int64, error)
	ListActiveEmployeesBrief(companiesID uint) ([]models.Employees, error)
	ListEmployeeIDsWithSalaryUpTo(employeeIDs []uint, date time.Time) ([]uint, error)
	AggregateCycle(cycleID uint) (CycleAggregate, error)
	FindPreviousClosedCycle(companiesID uint, before time.Time) (models.PayrollCycle, bool, error)

	ListTransactionsForCycle(cycleID uint) ([]models.PayrollTransaction, error)
	FindTransactionDetail(transactionID uint) (models.PayrollTransaction, error)
	FindCompany(companiesID uint) (models.Companies, error)
	ListClosedCycleTransactionsForEmployee(employeeID uint) ([]models.PayrollTransaction, error)
	ListCyclesByIDs(cycleIDs []uint) ([]models.PayrollCycle, error)
}

type gormPayrollRunRepository struct {
	db *gorm.DB
}

// NewPayrollRunRepository returns the GORM-backed implementation on the given connection.
func NewPayrollRunRepository(db *gorm.DB) PayrollRunRepository {
	return &gormPayrollRunRepository{db: db}
}

// FindCycleScoped resolves a cycle the caller's role authorizes.
func (r *gormPayrollRunRepository) FindCycleScoped(scope AccessScope, cycleID uint) (models.PayrollCycle, error) {
	var cycle models.PayrollCycle
	err := r.db.Scopes(scope.ApplyCompany("payroll_cycle.companies_id")).First(&cycle, cycleID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.PayrollCycle{}, ErrPayrollCycleMissing
		}
		return models.PayrollCycle{}, err
	}
	return cycle, nil
}

// FindOpenCycleScoped resolves a cycle the caller may still post items into, keeping missing and closed distinct.
func (r *gormPayrollRunRepository) FindOpenCycleScoped(scope AccessScope, cycleID uint) (models.PayrollCycle, error) {
	cycle, err := r.FindCycleScoped(scope, cycleID)
	if err != nil {
		return models.PayrollCycle{}, err
	}
	if cycle.IsClosed {
		return models.PayrollCycle{}, ErrPayrollCycleClosed
	}
	return cycle, nil
}

func (r *gormPayrollRunRepository) ListCyclesScoped(scope AccessScope) ([]models.PayrollCycle, error) {
	var cycles []models.PayrollCycle
	err := r.db.Scopes(scope.ApplyCompany("payroll_cycle.companies_id")).
		Preload("Companies").
		Order("created_at DESC").
		Find(&cycles).Error
	return cycles, err
}

func (r *gormPayrollRunRepository) CreateCycle(cycle *models.PayrollCycle) error {
	return r.db.Create(cycle).Error
}

func (r *gormPayrollRunRepository) MarkCycleClosed(cycle *models.PayrollCycle) error {
	return r.db.Model(cycle).Update("is_closed", true).Error
}

// FindLatestPayrollConfig returns the newest live configuration.
func (r *gormPayrollRunRepository) FindLatestPayrollConfig(companiesID uint) (models.PayrollConfig, error) {
	var configuration models.PayrollConfig
	err := r.db.Where("companies_id = ? AND deleted_at IS NULL", companiesID).
		Order("created_at DESC").
		First(&configuration).Error
	return configuration, err
}

func (r *gormPayrollRunRepository) ListActiveEmployees(companiesID uint) ([]models.Employees, error) {
	var employees []models.Employees
	err := r.db.
		Preload("Position").
		Preload("Department").
		Preload("EmploymentType").
		Preload("Bank").
		Where("employees.companies_id = ? AND employees.is_active = ?", companiesID, true).
		Find(&employees).Error
	return employees, err
}

// FindAttendanceConfigForRun takes the highest id, which is what the run has always used.
func (r *gormPayrollRunRepository) FindAttendanceConfigForRun(companiesID uint) (models.AttendanceConfig, error) {
	var configuration models.AttendanceConfig
	err := r.db.Where("companies_id = ?", companiesID).Order("id DESC").First(&configuration).Error
	return configuration, err
}

func (r *gormPayrollRunRepository) ListHolidaysInRange(companiesID uint, from time.Time, to time.Time) ([]models.Holiday, error) {
	var holidays []models.Holiday
	err := r.db.Where("companies_id = ? AND date >= ? AND date <= ?", companiesID, from, to).Find(&holidays).Error
	return holidays, err
}

func (r *gormPayrollRunRepository) ListDayOffs(companiesID uint) ([]models.DayOff, error) {
	var dayOffs []models.DayOff
	err := r.db.Where("companies_id = ?", companiesID).Find(&dayOffs).Error
	return dayOffs, err
}

// FindActiveSalaryOnOrBefore returns the salary in force at the given date, newest first.
func (r *gormPayrollRunRepository) FindActiveSalaryOnOrBefore(employeeID uint, date time.Time) (models.SalariesStructure, error) {
	var record models.SalariesStructure
	err := r.db.Where("employees_id = ? AND is_active = ? AND effective_date <= ?", employeeID, true, date).
		Order("effective_date DESC, created_at DESC").
		First(&record).Error
	return record, salaryLookupError(err)
}

// FindEarliestActiveSalaryOnOrBefore returns the employee's first salary record up to the given date, for a new hire whose first record lands mid-cycle.
func (r *gormPayrollRunRepository) FindEarliestActiveSalaryOnOrBefore(employeeID uint, date time.Time) (models.SalariesStructure, error) {
	var record models.SalariesStructure
	err := r.db.Where("employees_id = ? AND is_active = ? AND effective_date <= ?", employeeID, true, date).
		Order("effective_date ASC, created_at ASC").
		First(&record).Error
	return record, salaryLookupError(err)
}

// salaryLookupError names the "no such salary" case so a run can tell it apart from a database failure, which must never read as an employee with no salary.
func salaryLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrPayrollSalaryMissing
	}
	return err
}

// DeleteTransactionForEmployee clears a transaction left by an earlier run that this run is skipping, so a stale figure does not keep appearing.
func (r *gormPayrollRunRepository) DeleteTransactionForEmployee(cycleID uint, employeeID uint) error {
	var stale models.PayrollTransaction
	if err := r.db.Where("cycle_id = ? AND employees_id = ?", cycleID, employeeID).First(&stale).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if err := r.db.Where("transaction_id = ?", stale.ID).Delete(&models.PayrollTransactionDetail{}).Error; err != nil {
		return err
	}
	return r.db.Delete(&stale).Error
}

func (r *gormPayrollRunRepository) ListEmployeeEarnings(employeeID uint) ([]models.EmployeeEarning, error) {
	var items []models.EmployeeEarning
	err := r.db.Preload("EarningType").Where("employees_id = ?", employeeID).Find(&items).Error
	return items, err
}

func (r *gormPayrollRunRepository) ListCycleEarnings(employeeID uint, cycleID uint) ([]models.PayrollCycleEarning, error) {
	var items []models.PayrollCycleEarning
	err := r.db.Preload("EarningType").Where("employees_id = ? AND cycle_id = ?", employeeID, cycleID).Find(&items).Error
	return items, err
}

func (r *gormPayrollRunRepository) ListEmployeeDeductions(employeeID uint) ([]models.EmployeeDeduction, error) {
	var items []models.EmployeeDeduction
	err := r.db.Preload("DeductionType").Where("employees_id = ?", employeeID).Find(&items).Error
	return items, err
}

func (r *gormPayrollRunRepository) ListCycleDeductions(employeeID uint, cycleID uint) ([]models.PayrollCycleDeduction, error) {
	var items []models.PayrollCycleDeduction
	err := r.db.Preload("DeductionType").Where("employees_id = ? AND cycle_id = ?", employeeID, cycleID).Find(&items).Error
	return items, err
}

// ListPairedTimeLogs takes logs with both a check-in and a check-out whose check-in falls in [from, to).
func (r *gormPayrollRunRepository) ListPairedTimeLogs(employeeID uint, from time.Time, to time.Time) ([]models.TimeLog, error) {
	var logs []models.TimeLog
	err := r.db.Where(
		"employees_id = ? AND check_in >= ? AND check_in < ? AND check_in IS NOT NULL AND check_out IS NOT NULL",
		employeeID, from, to,
	).Find(&logs).Error
	return logs, err
}

func (r *gormPayrollRunRepository) ListApprovedLeavesInRange(employeeID uint, cycleStart time.Time, cycleEnd time.Time) ([]models.LeaveRequest, error) {
	var leaves []models.LeaveRequest
	err := r.db.Where(
		"employees_id = ? AND status = ? AND start_date <= ? AND end_date >= ?",
		employeeID, models.LeaveStatusApproved, cycleEnd, cycleStart,
	).Find(&leaves).Error
	return leaves, err
}

// SaveSnapshot writes one employee's computed transaction and replaces its detail lines, as a single transaction.
func (r *gormPayrollRunRepository) SaveSnapshot(cycleID uint, employeeID uint, snapshot TransactionSnapshot, details []models.PayrollTransactionDetail) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var transaction models.PayrollTransaction
		err := tx.Where("cycle_id = ? AND employees_id = ?", cycleID, employeeID).First(&transaction).Error
		isNew := errors.Is(err, gorm.ErrRecordNotFound)
		if err != nil && !isNew {
			return err
		}

		transaction.CycleID = cycleID
		transaction.EmployeesID = employeeID
		transaction.SalariesStructureID = snapshot.SalariesStructureID
		transaction.SalariesAtTime = snapshot.SalariesAtTime
		transaction.TotalEarning = snapshot.TotalEarning
		transaction.TotalDeduction = snapshot.TotalDeduction
		transaction.SsoAmount = snapshot.SsoAmount
		transaction.TaxableIncome = snapshot.TaxableIncome
		transaction.TaxAmount = snapshot.TaxAmount
		transaction.NetPay = snapshot.NetPay
		transaction.PositionName = snapshot.PositionName
		transaction.DepartmentName = snapshot.DepartmentName
		transaction.EmploymentTypeName = snapshot.EmploymentTypeName
		transaction.BankName = snapshot.BankName
		transaction.BankAccountName = snapshot.BankAccountName
		transaction.BankAccountNo = snapshot.BankAccountNo

		if isNew {
			if err := tx.Create(&transaction).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Save(&transaction).Error; err != nil {
				return err
			}
			if err := tx.Where("transaction_id = ?", transaction.ID).Delete(&models.PayrollTransactionDetail{}).Error; err != nil {
				return err
			}
		}

		if len(details) == 0 {
			return nil
		}
		for i := range details {
			details[i].TransactionID = transaction.ID
		}
		return tx.Create(&details).Error
	})
}

// CycleAggregate totals one cycle's transactions, for the run-versus-previous comparison.
type CycleAggregate struct {
	EmployeeCount  int
	TotalEarning   float64
	TotalDeduction float64
	SsoAmount      float64
	TaxAmount      float64
	NetPay         float64
}

func (r *gormPayrollRunRepository) CountLivePayrollConfigs(companiesID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.PayrollConfig{}).
		Where("companies_id = ? AND deleted_at IS NULL", companiesID).
		Count(&count).Error
	return count, err
}

// ListActiveEmployeesBrief loads only what the pre-check reports on.
func (r *gormPayrollRunRepository) ListActiveEmployeesBrief(companiesID uint) ([]models.Employees, error) {
	var employees []models.Employees
	err := r.db.
		Where("companies_id = ? AND is_active = ?", companiesID, true).
		Select("id, first_name, last_name, bank_id").
		Find(&employees).Error
	return employees, err
}

// ListEmployeeIDsWithSalaryUpTo returns which of the given employees have a salary
func (r *gormPayrollRunRepository) ListEmployeeIDsWithSalaryUpTo(employeeIDs []uint, date time.Time) ([]uint, error) {
	ids := make([]uint, 0)
	if len(employeeIDs) == 0 {
		return ids, nil
	}
	err := r.db.Model(&models.SalariesStructure{}).
		Distinct().
		Where("employees_id IN ? AND is_active = ? AND effective_date <= ?", employeeIDs, true, date).
		Pluck("employees_id", &ids).Error
	return ids, err
}

func (r *gormPayrollRunRepository) AggregateCycle(cycleID uint) (CycleAggregate, error) {
	var aggregate CycleAggregate
	err := r.db.Model(&models.PayrollTransaction{}).
		Select("COUNT(*) as employee_count, COALESCE(SUM(total_earning),0) as total_earning, COALESCE(SUM(total_deduction),0) as total_deduction, COALESCE(SUM(sso_amount),0) as sso_amount, COALESCE(SUM(tax_amount),0) as tax_amount, COALESCE(SUM(net_pay),0) as net_pay").
		Where("cycle_id = ? AND deleted_at = 0", cycleID).
		Scan(&aggregate).Error
	return aggregate, err
}

// FindPreviousClosedCycle returns the company's most recent closed cycle ending before
func (r *gormPayrollRunRepository) FindPreviousClosedCycle(companiesID uint, before time.Time) (models.PayrollCycle, bool, error) {
	var cycle models.PayrollCycle
	err := r.db.
		Where("companies_id = ? AND is_closed = true AND end_date < ? AND deleted_at = 0", companiesID, before).
		Order("end_date DESC").
		First(&cycle).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.PayrollCycle{}, false, nil
		}
		return models.PayrollCycle{}, false, err
	}
	return cycle, true, nil
}

func (r *gormPayrollRunRepository) ListTransactionsForCycle(cycleID uint) ([]models.PayrollTransaction, error) {
	var transactions []models.PayrollTransaction
	err := r.db.Where("cycle_id = ?", cycleID).
		Preload("Employees").
		Preload("Employees.Department").
		Preload("Employees.Position").
		Preload("Employees.Companies").
		Preload("Details").
		Find(&transactions).Error
	return transactions, err
}

func (r *gormPayrollRunRepository) FindTransactionDetail(transactionID uint) (models.PayrollTransaction, error) {
	var transaction models.PayrollTransaction
	err := r.db.
		Preload("Employees").
		Preload("Employees.Department").
		Preload("Employees.Position").
		Preload("Details").
		First(&transaction, transactionID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.PayrollTransaction{}, ErrPayrollTransactionMissing
		}
		return models.PayrollTransaction{}, err
	}
	return transaction, nil
}

func (r *gormPayrollRunRepository) FindCompany(companiesID uint) (models.Companies, error) {
	var company models.Companies
	err := r.db.First(&company, companiesID).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Companies{}, err
	}
	// A missing company leaves the zero value, matching the previous behaviour of ignoring this lookup's error.
	return company, nil
}

// ListClosedCycleTransactionsForEmployee backs My Payslips: only closed cycles count.
func (r *gormPayrollRunRepository) ListClosedCycleTransactionsForEmployee(employeeID uint) ([]models.PayrollTransaction, error) {
	var transactions []models.PayrollTransaction
	err := r.db.
		Joins("JOIN payroll_cycle ON payroll_cycle.id = payroll_transaction.cycle_id AND payroll_cycle.is_closed = true AND payroll_cycle.deleted_at = 0").
		Where("payroll_transaction.employees_id = ? AND payroll_transaction.deleted_at = 0", employeeID).
		Preload("Details").
		Order("payroll_transaction.id DESC").
		Find(&transactions).Error
	return transactions, err
}

func (r *gormPayrollRunRepository) ListCyclesByIDs(cycleIDs []uint) ([]models.PayrollCycle, error) {
	var cycles []models.PayrollCycle
	if len(cycleIDs) == 0 {
		return cycles, nil
	}
	err := r.db.Where("id IN ?", cycleIDs).Find(&cycles).Error
	return cycles, err
}

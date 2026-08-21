package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// Report failures.
var (
	ErrReportCycleNotFound   = apperr.NotFound("Payroll cycle not found")
	ErrReportCycleOpen       = apperr.Validation("This report is only available for closed payroll cycles")
	ErrReportTxNotFound      = apperr.NotFound("Transaction not found")
	ErrReportAccessDenied    = apperr.Forbidden("Access denied")
	ErrReportPayslipOpen     = apperr.Forbidden("Payslips are only available for closed payroll cycles")
	ErrReport50TawiCycleOpen = apperr.Validation("50 Tawi is only available for closed cycles")
)

// ReportRepository is the read boundary for the reporting endpoints.
type ReportRepository interface {
	FindClosedCycleScoped(scope AccessScope, cycleID uint) (models.PayrollCycle, error)
	FindCycleScoped(scope AccessScope, cycleID uint) (models.PayrollCycle, error)
	FindCycleWithCompanyScoped(scope AccessScope, cycleID uint) (models.PayrollCycle, error)
	FindPayslipTransaction(transactionID uint) (models.PayrollTransaction, error)
	FindTaxCertificateTransaction(transactionID uint) (models.PayrollTransaction, error)
	FindCompany(companiesID uint) (models.Companies, error)
	FindPayrollConfig(companiesID uint) (models.PayrollConfig, error)
	ListCycleTransactions(cycleID uint) ([]models.PayrollTransaction, error)
	ListCycleTransactionsDetailed(cycleID uint) ([]models.PayrollTransaction, error)
}

type gormReportRepository struct {
	db *gorm.DB
}

// NewReportRepository returns the GORM-backed implementation on the given connection.
func NewReportRepository(db *gorm.DB) ReportRepository {
	return &gormReportRepository{db: db}
}

// FindCycleScoped resolves a cycle the caller's role authorizes, without its company.
func (r *gormReportRepository) FindCycleScoped(scope AccessScope, cycleID uint) (models.PayrollCycle, error) {
	var cycle models.PayrollCycle
	err := r.db.Scopes(scope.ApplyCompany("payroll_cycle.companies_id")).First(&cycle, cycleID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.PayrollCycle{}, ErrReportCycleNotFound
		}
		return models.PayrollCycle{}, err
	}
	return cycle, nil
}

// FindCycleWithCompanyScoped is the same lookup with the company preloaded.
func (r *gormReportRepository) FindCycleWithCompanyScoped(scope AccessScope, cycleID uint) (models.PayrollCycle, error) {
	var cycle models.PayrollCycle
	err := r.db.Scopes(scope.ApplyCompany("payroll_cycle.companies_id")).
		Preload("Companies").
		First(&cycle, cycleID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.PayrollCycle{}, ErrReportCycleNotFound
		}
		return models.PayrollCycle{}, err
	}
	return cycle, nil
}

// FindClosedCycleScoped is the gate the cycle-wide reports share: the cycle must be in reach and closed, since a report is a statement about final figures.
func (r *gormReportRepository) FindClosedCycleScoped(scope AccessScope, cycleID uint) (models.PayrollCycle, error) {
	cycle, err := r.FindCycleWithCompanyScoped(scope, cycleID)
	if err != nil {
		return models.PayrollCycle{}, err
	}
	if !cycle.IsClosed {
		return models.PayrollCycle{}, ErrReportCycleOpen
	}
	return cycle, nil
}

func (r *gormReportRepository) FindPayslipTransaction(transactionID uint) (models.PayrollTransaction, error) {
	var transaction models.PayrollTransaction
	err := r.db.
		Preload("Employees").
		Preload("Employees.Department").
		Preload("Employees.Position").
		Preload("Employees.Companies").
		Preload("Details").
		First(&transaction, transactionID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.PayrollTransaction{}, ErrReportTxNotFound
		}
		return models.PayrollTransaction{}, err
	}
	return transaction, nil
}

func (r *gormReportRepository) FindTaxCertificateTransaction(transactionID uint) (models.PayrollTransaction, error) {
	var transaction models.PayrollTransaction
	err := r.db.
		Preload("Employees").
		Preload("Employees.Companies").
		Preload("Employees.Department").
		Preload("Employees.Position").
		First(&transaction, transactionID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.PayrollTransaction{}, ErrReportTxNotFound
		}
		return models.PayrollTransaction{}, err
	}
	return transaction, nil
}

// FindCompany loads a company for a report header.
func (r *gormReportRepository) FindCompany(companiesID uint) (models.Companies, error) {
	var company models.Companies
	err := r.db.First(&company, companiesID).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Companies{}, err
	}
	return company, nil
}

// FindPayrollConfig loads the company's configuration for the rates a report prints.
func (r *gormReportRepository) FindPayrollConfig(companiesID uint) (models.PayrollConfig, error) {
	var configuration models.PayrollConfig
	err := r.db.Where("companies_id = ?", companiesID).First(&configuration).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.PayrollConfig{}, err
	}
	return configuration, nil
}

func (r *gormReportRepository) ListCycleTransactions(cycleID uint) ([]models.PayrollTransaction, error) {
	var transactions []models.PayrollTransaction
	err := r.db.Where("cycle_id = ?", cycleID).
		Preload("Employees").
		Preload("Employees.Companies").
		Find(&transactions).Error
	return transactions, err
}

func (r *gormReportRepository) ListCycleTransactionsDetailed(cycleID uint) ([]models.PayrollTransaction, error) {
	var transactions []models.PayrollTransaction
	err := r.db.Where("cycle_id = ?", cycleID).
		Preload("Employees").
		Preload("Employees.Companies").
		Preload("Employees.Department").
		Preload("Employees.Position").
		Preload("Details").
		Find(&transactions).Error
	return transactions, err
}

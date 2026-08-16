package repository

import (
	"errors"
	"time"

	"payroll-backend/apperr"
	"payroll-backend/config"
	"payroll-backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrPayrollConfigNotFound means the cycle's company has no live configuration.
var ErrPayrollConfigNotFound = apperr.NotFound("No active payroll config found for this cycle's company")

// PayrollConfigRepository is the persistence boundary for the payroll-config domain.
type PayrollConfigRepository interface {
	ListCompaniesWithConfig(scope AccessScope) ([]models.Companies, error)
	FindCycleScoped(scope AccessScope, cycleID uint) (models.PayrollCycle, error)
	FindActiveForCompany(companyID uint) (models.PayrollConfig, error)
	UpsertForCompany(configuration *models.PayrollConfig) error
}

type gormPayrollConfigRepository struct {
	db *gorm.DB
}

// NewPayrollConfigRepository returns the GORM-backed implementation bound to the process-wide connection.
func NewPayrollConfigRepository() PayrollConfigRepository {
	return &gormPayrollConfigRepository{db: config.DB}
}

// ListCompaniesWithConfig returns the caller's authorized companies, each with its configuration preloaded.
func (r *gormPayrollConfigRepository) ListCompaniesWithConfig(scope AccessScope) ([]models.Companies, error) {
	var companies []models.Companies
	err := r.db.Scopes(scope.ApplyCompany("companies.id")).
		Order("companies.id ASC").
		Preload("PayrollConfig").
		Find(&companies).Error
	return companies, err
}

// FindCycleScoped resolves a payroll cycle the caller may read.
func (r *gormPayrollConfigRepository) FindCycleScoped(scope AccessScope, cycleID uint) (models.PayrollCycle, error) {
	var cycle models.PayrollCycle
	err := r.db.Scopes(scope.ApplyCompany("payroll_cycle.companies_id")).
		Preload("Companies").
		First(&cycle, cycleID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.PayrollCycle{}, ErrPayrollCycleNotFound
		}
		return models.PayrollCycle{}, err
	}
	return cycle, nil
}

// FindActiveForCompany returns the company's newest live configuration.
func (r *gormPayrollConfigRepository) FindActiveForCompany(companyID uint) (models.PayrollConfig, error) {
	var payrollConfig models.PayrollConfig
	err := r.db.Where("companies_id = ? AND deleted_at IS NULL", companyID).
		Order("created_at DESC").
		First(&payrollConfig).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.PayrollConfig{}, ErrPayrollConfigNotFound
		}
		return models.PayrollConfig{}, err
	}
	return payrollConfig, nil
}

// UpsertForCompany writes the configuration, updating in place when the company already has one.
func (r *gormPayrollConfigRepository) UpsertForCompany(configuration *models.PayrollConfig) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "companies_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"sso_rate":        configuration.SsoRate,
			"sso_max_amount":  configuration.SsoMaxAmount,
			"sso_min_salary":  configuration.SsoMinSalary,
			"tax_calculation": configuration.TaxCalculation,
			"updated_at":      time.Now(),
		}),
	}).Create(configuration).Error
}

package repository

import (
	"errors"
	"time"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrPayrollConfigNotFound means the cycle's company has no live configuration.
var ErrPayrollConfigNotFound = apperr.NotFound("No active payroll config found for this cycle's company")

// PayrollConfigRepository is the persistence boundary for the payroll-config domain.
type PayrollConfigRepository interface {
	ListCompaniesWithConfig(scope AccessScope, q PageQuery) (Page[models.Companies], error)
	FindCycleScoped(scope AccessScope, cycleID uint) (models.PayrollCycle, error)
	FindActiveForCompany(companyID uint) (models.PayrollConfig, error)
	UpsertForCompany(configuration *models.PayrollConfig, audit *models.AuditLog) error
}

type gormPayrollConfigRepository struct {
	db *gorm.DB
}

// NewPayrollConfigRepository returns the GORM-backed implementation on the given connection.
func NewPayrollConfigRepository(db *gorm.DB) PayrollConfigRepository {
	return &gormPayrollConfigRepository{db: db}
}

// ListCompaniesWithConfig returns the caller's authorized companies, each with its configuration preloaded.
func (r *gormPayrollConfigRepository) ListCompaniesWithConfig(scope AccessScope, q PageQuery) (Page[models.Companies], error) {
	return readCatalogPage[models.Companies](r.db.Scopes(scope.ApplyCompany("companies.id")).Preload("PayrollConfig"), q, nil)
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
func (r *gormPayrollConfigRepository) UpsertForCompany(configuration *models.PayrollConfig, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Non-nil pointers preserve deliberate zero values on initial INSERT.
		row := struct {
			models.PayrollConfig `gorm:"embedded"`
			SsoRate              *float64 `gorm:"column:sso_rate"`
			SsoMaxAmount         *float64 `gorm:"column:sso_max_amount"`
			SsoMinSalary         *float64 `gorm:"column:sso_min_salary"`
		}{PayrollConfig: *configuration, SsoRate: &configuration.SsoRate, SsoMaxAmount: &configuration.SsoMaxAmount, SsoMinSalary: &configuration.SsoMinSalary}
		if err := tx.Table("payroll_config").Omit(clause.Associations).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "companies_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"sso_rate":        configuration.SsoRate,
				"sso_max_amount":  configuration.SsoMaxAmount,
				"sso_min_salary":  configuration.SsoMinSalary,
				"tax_calculation": configuration.TaxCalculation,
				"updated_at":      time.Now(),
			}),
		}).Create(&row).Error; err != nil {
			return err
		}
		*configuration = row.PayrollConfig
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

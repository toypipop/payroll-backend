package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/config"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// Cycle-deduction lookup failures. Update and delete answered slightly different
// messages before the split, and both are kept.
var (
	ErrCycleDeductionNotFound     = apperr.NotFound("Record not found or access denied")
	ErrCycleDeductionNotDeletable = apperr.NotFound("Record not found or unauthorized")
)

// CycleDeductionRepository is the persistence boundary for one-off deductions attached to a cycle.
type CycleDeductionRepository interface {
	ListScopedByCycle(scope AccessScope, cycleID uint) ([]models.PayrollCycleDeduction, error)
	FindScoped(scope AccessScope, id uint) (models.PayrollCycleDeduction, error)
	FindDetail(id uint) (models.PayrollCycleDeduction, error)
	Create(item *models.PayrollCycleDeduction) error
	UpdateFields(item *models.PayrollCycleDeduction, fields map[string]any) error
	Delete(item *models.PayrollCycleDeduction) error
}

type gormCycleDeductionRepository struct {
	db *gorm.DB
}

// NewCycleDeductionRepository returns the GORM-backed implementation bound to the process-wide connection.
func NewCycleDeductionRepository() CycleDeductionRepository {
	return &gormCycleDeductionRepository{db: config.DB}
}

// scoped filters by company and by employee: the table has a real employee column
// and CycleDeductionsPage is declared employee-level, so Own and Department bite.
func (r *gormCycleDeductionRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyEmployee("payroll_cycle_deduction.companies_id", "payroll_cycle_deduction.employees_id"))
}

func (r *gormCycleDeductionRepository) ListScopedByCycle(scope AccessScope, cycleID uint) ([]models.PayrollCycleDeduction, error) {
	var items []models.PayrollCycleDeduction
	err := r.scoped(scope).
		Preload("DeductionType").
		Preload("Employees").
		Preload("Employees.Companies").
		Where("cycle_id = ?", cycleID).
		Find(&items).Error
	return items, err
}

func (r *gormCycleDeductionRepository) FindScoped(scope AccessScope, id uint) (models.PayrollCycleDeduction, error) {
	var item models.PayrollCycleDeduction
	if err := r.scoped(scope).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.PayrollCycleDeduction{}, ErrCycleDeductionNotFound
		}
		return models.PayrollCycleDeduction{}, err
	}
	return item, nil
}

// FindDetail reloads a row with its relations for the response body.
func (r *gormCycleDeductionRepository) FindDetail(id uint) (models.PayrollCycleDeduction, error) {
	var item models.PayrollCycleDeduction
	if err := r.db.Preload("DeductionType").Preload("Employees").First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.PayrollCycleDeduction{}, ErrCycleDeductionNotFound
		}
		return models.PayrollCycleDeduction{}, err
	}
	return item, nil
}

func (r *gormCycleDeductionRepository) Create(item *models.PayrollCycleDeduction) error {
	return r.db.Create(item).Error
}

func (r *gormCycleDeductionRepository) UpdateFields(item *models.PayrollCycleDeduction, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Model(item).Updates(fields).Error
}

func (r *gormCycleDeductionRepository) Delete(item *models.PayrollCycleDeduction) error {
	return r.db.Delete(item).Error
}

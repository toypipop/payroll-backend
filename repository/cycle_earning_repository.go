package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/config"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// Cycle-earning lookup failures. Update and delete answered slightly different
// messages before the split, and both are kept.
var (
	ErrCycleEarningNotFound     = apperr.NotFound("Record not found or access denied")
	ErrCycleEarningNotDeletable = apperr.NotFound("Record not found or unauthorized")
)

// CycleEarningRepository is the persistence boundary for one-off earnings attached to a cycle.
type CycleEarningRepository interface {
	ListScopedByCycle(scope AccessScope, cycleID uint) ([]models.PayrollCycleEarning, error)
	FindScoped(scope AccessScope, id uint) (models.PayrollCycleEarning, error)
	FindDetail(id uint) (models.PayrollCycleEarning, error)
	Create(item *models.PayrollCycleEarning) error
	UpdateFields(item *models.PayrollCycleEarning, fields map[string]any) error
	Delete(item *models.PayrollCycleEarning) error
}

type gormCycleEarningRepository struct {
	db *gorm.DB
}

// NewCycleEarningRepository returns the GORM-backed implementation bound to the process-wide connection.
func NewCycleEarningRepository() CycleEarningRepository {
	return &gormCycleEarningRepository{db: config.DB}
}

// scoped filters by company and by employee: the table has a real employee column
// and CycleEarningsPage is declared employee-level, so Own and Department bite.
func (r *gormCycleEarningRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyEmployee("payroll_cycle_earning.companies_id", "payroll_cycle_earning.employees_id"))
}

func (r *gormCycleEarningRepository) ListScopedByCycle(scope AccessScope, cycleID uint) ([]models.PayrollCycleEarning, error) {
	var items []models.PayrollCycleEarning
	err := r.scoped(scope).
		Preload("EarningType").
		Preload("Employees").
		Preload("Employees.Companies").
		Where("cycle_id = ?", cycleID).
		Find(&items).Error
	return items, err
}

func (r *gormCycleEarningRepository) FindScoped(scope AccessScope, id uint) (models.PayrollCycleEarning, error) {
	var item models.PayrollCycleEarning
	if err := r.scoped(scope).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.PayrollCycleEarning{}, ErrCycleEarningNotFound
		}
		return models.PayrollCycleEarning{}, err
	}
	return item, nil
}

// FindDetail reloads a row with its relations for the response body.
func (r *gormCycleEarningRepository) FindDetail(id uint) (models.PayrollCycleEarning, error) {
	var item models.PayrollCycleEarning
	if err := r.db.Preload("EarningType").Preload("Employees").First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.PayrollCycleEarning{}, ErrCycleEarningNotFound
		}
		return models.PayrollCycleEarning{}, err
	}
	return item, nil
}

func (r *gormCycleEarningRepository) Create(item *models.PayrollCycleEarning) error {
	return r.db.Create(item).Error
}

func (r *gormCycleEarningRepository) UpdateFields(item *models.PayrollCycleEarning, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Model(item).Updates(fields).Error
}

func (r *gormCycleEarningRepository) Delete(item *models.PayrollCycleEarning) error {
	return r.db.Delete(item).Error
}

package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/config"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// Employee-earning lookup failures. Update and delete answered slightly different
// messages before the split, and both are kept.
var (
	ErrEmployeeEarningNotFound     = apperr.NotFound("Record not found or access denied")
	ErrEmployeeEarningNotDeletable = apperr.NotFound("Record not found or unauthorized")
)

// EmployeeEarningRepository is the persistence boundary for the employee-earnings domain.
type EmployeeEarningRepository interface {
	ListScoped(scope AccessScope) ([]models.EmployeeEarning, error)
	FindScoped(scope AccessScope, id uint) (models.EmployeeEarning, error)
	FindDetail(id uint) (models.EmployeeEarning, error)
	Create(earning *models.EmployeeEarning) error
	UpdateFields(earning *models.EmployeeEarning, fields map[string]any) error
	Delete(earning *models.EmployeeEarning) error
}

type gormEmployeeEarningRepository struct {
	db *gorm.DB
}

// NewEmployeeEarningRepository returns the GORM-backed implementation bound to the process-wide connection.
func NewEmployeeEarningRepository() EmployeeEarningRepository {
	return &gormEmployeeEarningRepository{db: config.DB}
}

// scoped filters by company and by employee: the table has a real employee column
// and EmployeeEarningsPage is declared employee-level, so Own and Department bite.
func (r *gormEmployeeEarningRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyEmployee("employee_earning.companies_id", "employee_earning.employees_id"))
}

func (r *gormEmployeeEarningRepository) ListScoped(scope AccessScope) ([]models.EmployeeEarning, error) {
	var items []models.EmployeeEarning
	err := r.scoped(scope).
		Preload("EarningType").
		Preload("Employees").
		Preload("Employees.Companies").
		Find(&items).Error
	return items, err
}

func (r *gormEmployeeEarningRepository) FindScoped(scope AccessScope, id uint) (models.EmployeeEarning, error) {
	var item models.EmployeeEarning
	if err := r.scoped(scope).Where("id = ?", id).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.EmployeeEarning{}, ErrEmployeeEarningNotFound
		}
		return models.EmployeeEarning{}, err
	}
	return item, nil
}

// FindDetail reloads a row with its earning type for the response body.
func (r *gormEmployeeEarningRepository) FindDetail(id uint) (models.EmployeeEarning, error) {
	var item models.EmployeeEarning
	if err := r.db.Preload("EarningType").First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.EmployeeEarning{}, ErrEmployeeEarningNotFound
		}
		return models.EmployeeEarning{}, err
	}
	return item, nil
}

func (r *gormEmployeeEarningRepository) Create(earning *models.EmployeeEarning) error {
	return r.db.Create(earning).Error
}

func (r *gormEmployeeEarningRepository) UpdateFields(earning *models.EmployeeEarning, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Model(earning).Updates(fields).Error
}

func (r *gormEmployeeEarningRepository) Delete(earning *models.EmployeeEarning) error {
	return r.db.Delete(earning).Error
}

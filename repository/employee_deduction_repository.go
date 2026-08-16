package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/config"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// Employee-deduction lookup failures.
var (
	ErrEmployeeDeductionNotFound     = apperr.NotFound("Record not found or access denied")
	ErrEmployeeDeductionNotDeletable = apperr.NotFound("Record not found or unauthorized")
)

// EmployeeDeductionRepository is the persistence boundary for the employee-deductions domain.
type EmployeeDeductionRepository interface {
	ListScoped(scope AccessScope) ([]models.EmployeeDeduction, error)
	FindScoped(scope AccessScope, id uint) (models.EmployeeDeduction, error)
	FindDetail(id uint) (models.EmployeeDeduction, error)
	Create(deduction *models.EmployeeDeduction) error
	UpdateFields(deduction *models.EmployeeDeduction, fields map[string]any) error
	Delete(deduction *models.EmployeeDeduction) error
}

type gormEmployeeDeductionRepository struct {
	db *gorm.DB
}

// NewEmployeeDeductionRepository returns the GORM-backed implementation bound to the process-wide connection.
func NewEmployeeDeductionRepository() EmployeeDeductionRepository {
	return &gormEmployeeDeductionRepository{db: config.DB}
}

// scoped filters by company and by employee: the table has a real employee column
func (r *gormEmployeeDeductionRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyEmployee("employee_deduction.companies_id", "employee_deduction.employees_id"))
}

func (r *gormEmployeeDeductionRepository) ListScoped(scope AccessScope) ([]models.EmployeeDeduction, error) {
	var items []models.EmployeeDeduction
	err := r.scoped(scope).
		Preload("DeductionType").
		Preload("Employees").
		Preload("Employees.Companies").
		Find(&items).Error
	return items, err
}

func (r *gormEmployeeDeductionRepository) FindScoped(scope AccessScope, id uint) (models.EmployeeDeduction, error) {
	var item models.EmployeeDeduction
	if err := r.scoped(scope).Where("id = ?", id).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.EmployeeDeduction{}, ErrEmployeeDeductionNotFound
		}
		return models.EmployeeDeduction{}, err
	}
	return item, nil
}

// FindDetail reloads a row with its deduction type for the response body.
func (r *gormEmployeeDeductionRepository) FindDetail(id uint) (models.EmployeeDeduction, error) {
	var item models.EmployeeDeduction
	if err := r.db.Preload("DeductionType").First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.EmployeeDeduction{}, ErrEmployeeDeductionNotFound
		}
		return models.EmployeeDeduction{}, err
	}
	return item, nil
}

func (r *gormEmployeeDeductionRepository) Create(deduction *models.EmployeeDeduction) error {
	return r.db.Create(deduction).Error
}

func (r *gormEmployeeDeductionRepository) UpdateFields(deduction *models.EmployeeDeduction, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Model(deduction).Updates(fields).Error
}

func (r *gormEmployeeDeductionRepository) Delete(deduction *models.EmployeeDeduction) error {
	return r.db.Delete(deduction).Error
}

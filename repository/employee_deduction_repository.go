package repository

import (
	"errors"

	"payroll-backend/apperr"
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
	ListScoped(scope AccessScope, q PageQuery) (Page[models.EmployeeDeduction], error)
	FindScoped(scope AccessScope, id uint) (models.EmployeeDeduction, error)
	FindDetail(id uint) (models.EmployeeDeduction, error)
	Create(deduction *models.EmployeeDeduction, audit *models.AuditLog) error
	UpdateFields(deduction *models.EmployeeDeduction, fields map[string]any, audit *models.AuditLog) error
	Delete(deduction *models.EmployeeDeduction, audit *models.AuditLog) error
}

type gormEmployeeDeductionRepository struct {
	db *gorm.DB
}

// NewEmployeeDeductionRepository returns the GORM-backed implementation on the given connection.
func NewEmployeeDeductionRepository(db *gorm.DB) EmployeeDeductionRepository {
	return &gormEmployeeDeductionRepository{db: db}
}

// scoped filters by company and by employee: the table has a real employee column
func (r *gormEmployeeDeductionRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyEmployee("employee_deduction.companies_id", "employee_deduction.employees_id"))
}

func (r *gormEmployeeDeductionRepository) ListScoped(scope AccessScope, q PageQuery) (Page[models.EmployeeDeduction], error) {
	return readEmployeeAdjustmentPage[models.EmployeeDeduction](r.scoped(scope), q, "employee_deduction", "deduction_type", "DeductionType")
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

func (r *gormEmployeeDeductionRepository) Create(deduction *models.EmployeeDeduction, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockActiveEmployees(tx, deduction.EmployeesID); err != nil {
			return err
		}
		if err := tx.Create(deduction).Error; err != nil {
			return err
		}
		if audit != nil && audit.EntityID != nil {
			*audit.EntityID = deduction.ID
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormEmployeeDeductionRepository) UpdateFields(deduction *models.EmployeeDeduction, fields map[string]any, audit *models.AuditLog) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockEmployeeUpdate(tx, deduction.EmployeesID, fields, "employees_id"); err != nil {
			return err
		}
		if err := tx.Model(deduction).Updates(fields).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormEmployeeDeductionRepository) Delete(deduction *models.EmployeeDeduction, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(deduction).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

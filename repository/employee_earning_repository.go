package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// Employee-earning lookup failures.
var (
	ErrEmployeeEarningNotFound     = apperr.NotFound("Record not found or access denied")
	ErrEmployeeEarningNotDeletable = apperr.NotFound("Record not found or unauthorized")
)

// EmployeeEarningRepository is the persistence boundary for the employee-earnings domain.
type EmployeeEarningRepository interface {
	ListScoped(scope AccessScope) ([]models.EmployeeEarning, error)
	FindScoped(scope AccessScope, id uint) (models.EmployeeEarning, error)
	FindDetail(id uint) (models.EmployeeEarning, error)
	Create(earning *models.EmployeeEarning, audit *models.AuditLog) error
	UpdateFields(earning *models.EmployeeEarning, fields map[string]any, audit *models.AuditLog) error
	Delete(earning *models.EmployeeEarning, audit *models.AuditLog) error
}

type gormEmployeeEarningRepository struct {
	db *gorm.DB
}

// NewEmployeeEarningRepository returns the GORM-backed implementation on the given connection.
func NewEmployeeEarningRepository(db *gorm.DB) EmployeeEarningRepository {
	return &gormEmployeeEarningRepository{db: db}
}

// scoped filters by company and by employee: the table has a real employee column and EmployeeEarningsPage is declared employee-level, so Own and Department bite.
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

func (r *gormEmployeeEarningRepository) Create(earning *models.EmployeeEarning, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockActiveEmployees(tx, earning.EmployeesID); err != nil {
			return err
		}
		if err := tx.Create(earning).Error; err != nil {
			return err
		}
		if audit != nil && audit.EntityID != nil {
			*audit.EntityID = earning.ID
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormEmployeeEarningRepository) UpdateFields(earning *models.EmployeeEarning, fields map[string]any, audit *models.AuditLog) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockEmployeeUpdate(tx, earning.EmployeesID, fields, "employees_id"); err != nil {
			return err
		}
		if err := tx.Model(earning).Updates(fields).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormEmployeeEarningRepository) Delete(earning *models.EmployeeEarning, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(earning).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

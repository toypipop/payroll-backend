package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// ErrAccessMappingNotFound covers both "no such row" and "out of the caller's scope".
var ErrAccessMappingNotFound = apperr.NotFound("Access mapping not found")

// EmployeeCompanyAccessRepository is the persistence boundary for the employee-company-access domain.
type EmployeeCompanyAccessRepository interface {
	ListScoped(scope AccessScope, q PageQuery) (Page[models.EmployeeCompanyAccess], error)
	FindScoped(scope AccessScope, id uint) (models.EmployeeCompanyAccess, error)
	FindDetail(id uint) (models.EmployeeCompanyAccess, error)
	FindEmployeeBrief(id uint) (models.Employees, error)
	Create(mapping *models.EmployeeCompanyAccess, audit *models.AuditLog) error
	UpdateFields(mapping *models.EmployeeCompanyAccess, fields map[string]any, audit *models.AuditLog) error
	Delete(mapping *models.EmployeeCompanyAccess, audit *models.AuditLog) error
}

type gormEmployeeCompanyAccessRepository struct {
	db *gorm.DB
}

// NewEmployeeCompanyAccessRepository returns the GORM-backed implementation on the given connection.
func NewEmployeeCompanyAccessRepository(db *gorm.DB) EmployeeCompanyAccessRepository {
	return &gormEmployeeCompanyAccessRepository{db: db}
}

// scoped filters by company and by employee: the table has a real employee column
func (r *gormEmployeeCompanyAccessRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyEmployee(
		"employee_company_access.company_id",
		"employee_company_access.employee_id",
	))
}

func (r *gormEmployeeCompanyAccessRepository) ListScoped(scope AccessScope, q PageQuery) (Page[models.EmployeeCompanyAccess], error) {
	query := r.scoped(scope).Joins("LEFT JOIN employees e ON e.id = employee_company_access.employee_id AND e.deleted_at = 0").Joins("LEFT JOIN companies c ON c.id = employee_company_access.company_id AND c.deleted_at = 0").Joins("LEFT JOIN roles rr ON rr.id = employee_company_access.role_id AND rr.deleted_at = 0").Preload("Employee", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, employees_code, first_name, last_name, companies_id")
	}).Preload("Company").Preload("Role")
	return readCatalogPage[models.EmployeeCompanyAccess](query, q, map[string]string{"employee_name": "concat_ws(' ', e.first_name, e.last_name)", "employee.employees_code": "e.employees_code", "company.name": "c.name", "role.roles_name": "rr.roles_name"})
}

func (r *gormEmployeeCompanyAccessRepository) FindScoped(scope AccessScope, id uint) (models.EmployeeCompanyAccess, error) {
	var mapping models.EmployeeCompanyAccess
	if err := r.scoped(scope).First(&mapping, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.EmployeeCompanyAccess{}, ErrAccessMappingNotFound
		}
		return models.EmployeeCompanyAccess{}, err
	}
	return mapping, nil
}

// FindDetail reloads a row with its relations for the response body.
func (r *gormEmployeeCompanyAccessRepository) FindDetail(id uint) (models.EmployeeCompanyAccess, error) {
	var mapping models.EmployeeCompanyAccess
	err := r.db.Preload("Employee").Preload("Company").Preload("Role").First(&mapping, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.EmployeeCompanyAccess{}, ErrAccessMappingNotFound
		}
		return models.EmployeeCompanyAccess{}, err
	}
	return mapping, nil
}

// FindEmployeeBrief confirms the target employee exists without loading the whole row.
func (r *gormEmployeeCompanyAccessRepository) FindEmployeeBrief(id uint) (models.Employees, error) {
	var employee models.Employees
	if err := r.db.Select("id, companies_id, is_active").First(&employee, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Employees{}, ErrEmployeeNotFound
		}
		return models.Employees{}, err
	}
	return employee, nil
}

func (r *gormEmployeeCompanyAccessRepository) Create(mapping *models.EmployeeCompanyAccess, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockActiveEmployees(tx, mapping.EmployeeID); err != nil {
			return err
		}
		// Preserve explicit false instead of letting GORM substitute default:true.
		active := mapping.IsActive
		row := struct {
			models.EmployeeCompanyAccess `gorm:"embedded"`
			IsActive                     *bool `gorm:"column:is_active"`
		}{EmployeeCompanyAccess: *mapping, IsActive: &active}
		if err := tx.Table("employee_company_access").Create(&row).Error; err != nil {
			return err
		}
		*mapping = row.EmployeeCompanyAccess
		mapping.IsActive = active
		if audit != nil && audit.EntityID != nil {
			*audit.EntityID = mapping.ID
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormEmployeeCompanyAccessRepository) UpdateFields(mapping *models.EmployeeCompanyAccess, fields map[string]any, audit *models.AuditLog) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockEmployeeUpdate(tx, mapping.EmployeeID, fields, "employee_id"); err != nil {
			return err
		}
		if err := tx.Model(mapping).Updates(fields).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormEmployeeCompanyAccessRepository) Delete(mapping *models.EmployeeCompanyAccess, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(mapping).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/config"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// ErrAccessMappingNotFound covers both "no such row" and "out of the caller's scope".
var ErrAccessMappingNotFound = apperr.NotFound("Access mapping not found")

// EmployeeCompanyAccessRepository is the persistence boundary for the employee-company-access domain.
type EmployeeCompanyAccessRepository interface {
	ListScoped(scope AccessScope) ([]models.EmployeeCompanyAccess, error)
	FindScoped(scope AccessScope, id uint) (models.EmployeeCompanyAccess, error)
	FindDetail(id uint) (models.EmployeeCompanyAccess, error)
	FindEmployeeBrief(id uint) (models.Employees, error)
	Create(mapping *models.EmployeeCompanyAccess) error
	UpdateFields(mapping *models.EmployeeCompanyAccess, fields map[string]any) error
	Delete(mapping *models.EmployeeCompanyAccess) error
}

type gormEmployeeCompanyAccessRepository struct {
	db *gorm.DB
}

// NewEmployeeCompanyAccessRepository returns the GORM-backed implementation bound to the process-wide connection.
func NewEmployeeCompanyAccessRepository() EmployeeCompanyAccessRepository {
	return &gormEmployeeCompanyAccessRepository{db: config.DB}
}

// scoped filters by company and by employee: the table has a real employee column
func (r *gormEmployeeCompanyAccessRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyEmployee(
		"employee_company_access.company_id",
		"employee_company_access.employee_id",
	))
}

func (r *gormEmployeeCompanyAccessRepository) ListScoped(scope AccessScope) ([]models.EmployeeCompanyAccess, error) {
	var mappings []models.EmployeeCompanyAccess
	err := r.scoped(scope).
		Preload("Employee").
		Preload("Company").
		Preload("Role").
		Order("employee_company_access.employee_id ASC, employee_company_access.company_id ASC").
		Find(&mappings).Error
	return mappings, err
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
	if err := r.db.Select("id, companies_id").First(&employee, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Employees{}, ErrEmployeeNotFound
		}
		return models.Employees{}, err
	}
	return employee, nil
}

func (r *gormEmployeeCompanyAccessRepository) Create(mapping *models.EmployeeCompanyAccess) error {
	return r.db.Create(mapping).Error
}

func (r *gormEmployeeCompanyAccessRepository) UpdateFields(mapping *models.EmployeeCompanyAccess, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Model(mapping).Updates(fields).Error
}

func (r *gormEmployeeCompanyAccessRepository) Delete(mapping *models.EmployeeCompanyAccess) error {
	return r.db.Delete(mapping).Error
}

package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/config"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// ErrDepartmentNotFound covers both "no such row" and "out of the caller's scope".
var ErrDepartmentNotFound = apperr.NotFound("Department not found or unauthorized")

// DepartmentRepository is the persistence boundary for the departments domain.
type DepartmentRepository interface {
	ListScoped(scope AccessScope) ([]models.Department, error)
	ListMinimalScoped(scope AccessScope) ([]models.Department, error)
	FindScoped(scope AccessScope, id uint) (models.Department, error)
	Create(department *models.Department) error
	Save(department *models.Department) error
	Delete(department *models.Department) error
}

type gormDepartmentRepository struct {
	db *gorm.DB
}

// NewDepartmentRepository returns the GORM-backed implementation bound to the process-wide connection.
func NewDepartmentRepository() DepartmentRepository {
	return &gormDepartmentRepository{db: config.DB}
}

// scoped filters by authorized company, the filter DepartmentSettings declares as a company-level module.
func (r *gormDepartmentRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyCompany("department.companies_id"))
}

func (r *gormDepartmentRepository) ListScoped(scope AccessScope) ([]models.Department, error) {
	var departments []models.Department
	err := r.scoped(scope).Preload("Companies").Preload("Category").Find(&departments).Error
	return departments, err
}

// ListMinimalScoped skips the relations, for the dropdown options endpoint that needs id/name only.
func (r *gormDepartmentRepository) ListMinimalScoped(scope AccessScope) ([]models.Department, error) {
	var departments []models.Department
	err := r.scoped(scope).Find(&departments).Error
	return departments, err
}

func (r *gormDepartmentRepository) FindScoped(scope AccessScope, id uint) (models.Department, error) {
	var department models.Department
	if err := r.scoped(scope).Where("id = ?", id).First(&department).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Department{}, ErrDepartmentNotFound
		}
		return models.Department{}, err
	}
	return department, nil
}

func (r *gormDepartmentRepository) Create(department *models.Department) error {
	return r.db.Create(department).Error
}

func (r *gormDepartmentRepository) Save(department *models.Department) error {
	return r.db.Save(department).Error
}

func (r *gormDepartmentRepository) Delete(department *models.Department) error {
	return r.db.Delete(department).Error
}

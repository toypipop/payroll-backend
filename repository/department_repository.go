package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// ErrDepartmentNotFound covers both "no such row" and "out of the caller's scope".
var ErrDepartmentNotFound = apperr.NotFound("Department not found or unauthorized")

// DepartmentRepository is the persistence boundary for the departments domain.
type DepartmentRepository interface {
	ListScoped(scope AccessScope, q PageQuery) (Page[models.Department], error)
	ListMinimalScoped(scope AccessScope) ([]models.Department, error)
	FindScoped(scope AccessScope, id uint) (models.Department, error)
	Create(department *models.Department, audit *models.AuditLog) error
	Save(department *models.Department, audit *models.AuditLog) error
	Delete(department *models.Department, audit *models.AuditLog) error
}

type gormDepartmentRepository struct {
	db *gorm.DB
}

// NewDepartmentRepository returns the GORM-backed implementation on the given connection.
func NewDepartmentRepository(db *gorm.DB) DepartmentRepository {
	return &gormDepartmentRepository{db: db}
}

// scoped filters by authorized company, the filter DepartmentSettings declares as a company-level module.
func (r *gormDepartmentRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyCompany("department.companies_id"))
}

func (r *gormDepartmentRepository) ListScoped(scope AccessScope, q PageQuery) (Page[models.Department], error) {
	query := r.scoped(scope).Joins("LEFT JOIN companies c ON c.id = department.companies_id AND c.deleted_at = 0").Preload("Companies")
	query = query.Joins("LEFT JOIN department_category cat ON cat.id = department.category_id AND cat.deleted_at = 0").Preload("Category")
	return readCatalogPage[models.Department](query, q, map[string]string{"companies.name": "c.name", "category.name": "cat.name"})
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

func (r *gormDepartmentRepository) Create(department *models.Department, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(department).Error; err != nil {
			return err
		}
		if audit != nil && audit.EntityID != nil {
			*audit.EntityID = department.ID
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormDepartmentRepository) Save(department *models.Department, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(department).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormDepartmentRepository) Delete(department *models.Department, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(department).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

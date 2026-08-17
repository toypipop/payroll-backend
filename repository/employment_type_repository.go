package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// ErrEmploymentTypeNotFound answers 404, which is what this domain has always returned for a missing or out-of-scope row.
var ErrEmploymentTypeNotFound = apperr.NotFound("Employment type not found")

// EmploymentTypeRepository is the persistence boundary for the employment-types domain.
type EmploymentTypeRepository interface {
	ListScoped(scope AccessScope) ([]models.EmploymentType, error)
	ListMinimalScoped(scope AccessScope) ([]models.EmploymentType, error)
	FindScoped(scope AccessScope, id uint) (models.EmploymentType, error)
	Create(employmentType *models.EmploymentType) error
	Save(employmentType *models.EmploymentType) error
	Delete(employmentType *models.EmploymentType) error
}

type gormEmploymentTypeRepository struct {
	db *gorm.DB
}

// NewEmploymentTypeRepository returns the GORM-backed implementation on the given connection.
func NewEmploymentTypeRepository(db *gorm.DB) EmploymentTypeRepository {
	return &gormEmploymentTypeRepository{db: db}
}

// scoped filters by authorized company, the filter EmploymentTypePage declares as a company-level module.
func (r *gormEmploymentTypeRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyCompany("employment_type.companies_id"))
}

func (r *gormEmploymentTypeRepository) ListScoped(scope AccessScope) ([]models.EmploymentType, error) {
	var types []models.EmploymentType
	err := r.scoped(scope).Preload("Companies").Order("name").Find(&types).Error
	return types, err
}

// ListMinimalScoped skips the relations, for the dropdown options endpoint that needs id/name only.
func (r *gormEmploymentTypeRepository) ListMinimalScoped(scope AccessScope) ([]models.EmploymentType, error) {
	var types []models.EmploymentType
	err := r.scoped(scope).Order("name").Find(&types).Error
	return types, err
}

func (r *gormEmploymentTypeRepository) FindScoped(scope AccessScope, id uint) (models.EmploymentType, error) {
	var employmentType models.EmploymentType
	if err := r.scoped(scope).First(&employmentType, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.EmploymentType{}, ErrEmploymentTypeNotFound
		}
		return models.EmploymentType{}, err
	}
	return employmentType, nil
}

func (r *gormEmploymentTypeRepository) Create(employmentType *models.EmploymentType) error {
	return r.db.Create(employmentType).Error
}

func (r *gormEmploymentTypeRepository) Save(employmentType *models.EmploymentType) error {
	return r.db.Save(employmentType).Error
}

func (r *gormEmploymentTypeRepository) Delete(employmentType *models.EmploymentType) error {
	return r.db.Delete(employmentType).Error
}

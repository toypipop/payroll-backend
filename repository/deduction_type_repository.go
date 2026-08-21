package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// Deduction-type lookups answer 403 rather than 404 when a row is missing or out of scope, which is the status this domain has always returned.
var (
	ErrDeductionTypeDenied       = apperr.Forbidden("Access denied or record not found")
	ErrDeductionTypeDeleteDenied = apperr.Forbidden("Delete failed or unauthorized")
)

// ErrDeductionTypeMissing means no such deduction-type row exists at all, as opposed to one the caller cannot reach.
var ErrDeductionTypeMissing = apperr.Validation("Deduction type not found")

// DeductionTypeRepository is the persistence boundary for the deduction-types domain.
type DeductionTypeRepository interface {
	Find(id uint) (models.DeductionType, error)
	ListScoped(scope AccessScope) ([]models.DeductionType, error)
	ListMinimalScoped(scope AccessScope) ([]models.DeductionType, error)
	FindScoped(scope AccessScope, id uint) (models.DeductionType, error)
	Create(deductionType *models.DeductionType) error
	UpdateFields(deductionType *models.DeductionType, fields map[string]any) error
	Delete(deductionType *models.DeductionType) error
}

type gormDeductionTypeRepository struct {
	db *gorm.DB
}

// NewDeductionTypeRepository returns the GORM-backed implementation on the given connection.
func NewDeductionTypeRepository(db *gorm.DB) DeductionTypeRepository {
	return &gormDeductionTypeRepository{db: db}
}

// scoped filters by authorized company, the filter DeductionTypesPage declares as a company-level module.
func (r *gormDeductionTypeRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyCompany("deduction_type.companies_id"))
}

// Find looks up a deduction type without any scope filter.
func (r *gormDeductionTypeRepository) Find(id uint) (models.DeductionType, error) {
	var deductionType models.DeductionType
	if err := r.db.First(&deductionType, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.DeductionType{}, ErrDeductionTypeMissing
		}
		return models.DeductionType{}, err
	}
	return deductionType, nil
}

func (r *gormDeductionTypeRepository) ListScoped(scope AccessScope) ([]models.DeductionType, error) {
	var deductions []models.DeductionType
	err := r.scoped(scope).Preload("Companies").Find(&deductions).Error
	return deductions, err
}

// ListMinimalScoped skips the relations, for the dropdown options endpoint that needs id/name only.
func (r *gormDeductionTypeRepository) ListMinimalScoped(scope AccessScope) ([]models.DeductionType, error) {
	var deductions []models.DeductionType
	err := r.scoped(scope).Find(&deductions).Error
	return deductions, err
}

func (r *gormDeductionTypeRepository) FindScoped(scope AccessScope, id uint) (models.DeductionType, error) {
	var deductionType models.DeductionType
	if err := r.scoped(scope).First(&deductionType, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.DeductionType{}, ErrDeductionTypeDenied
		}
		return models.DeductionType{}, err
	}
	return deductionType, nil
}

func (r *gormDeductionTypeRepository) Create(deductionType *models.DeductionType) error {
	return r.db.Create(deductionType).Error
}

// UpdateFields writes an explicit map so a false boolean is stored rather than skipped as a zero value.
func (r *gormDeductionTypeRepository) UpdateFields(deductionType *models.DeductionType, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Model(deductionType).Updates(fields).Error
}

func (r *gormDeductionTypeRepository) Delete(deductionType *models.DeductionType) error {
	return r.db.Delete(deductionType).Error
}

package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/config"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// Earning-type lookups answer 403 rather than 404 when a row is missing or out of
// scope, which is the status this domain has always returned.
var (
	ErrEarningTypeDenied       = apperr.Forbidden("Access denied or record not found")
	ErrEarningTypeDeleteDenied = apperr.Forbidden("Delete failed or unauthorized")
)

// ErrEarningTypeMissing means no such earning-type row exists at all, as opposed to
// one the caller cannot reach.
var ErrEarningTypeMissing = apperr.Validation("Earning type not found")

// EarningTypeRepository is the persistence boundary for the earning-types domain.
type EarningTypeRepository interface {
	Find(id uint) (models.EarningType, error)
	ListScoped(scope AccessScope) ([]models.EarningType, error)
	ListMinimalScoped(scope AccessScope) ([]models.EarningType, error)
	FindScoped(scope AccessScope, id uint) (models.EarningType, error)
	Create(earningType *models.EarningType) error
	UpdateFields(earningType *models.EarningType, fields map[string]any) error
	Delete(earningType *models.EarningType) error
}

type gormEarningTypeRepository struct {
	db *gorm.DB
}

// NewEarningTypeRepository returns the GORM-backed implementation bound to the process-wide connection.
func NewEarningTypeRepository() EarningTypeRepository {
	return &gormEarningTypeRepository{db: config.DB}
}

// scoped filters by authorized company, the filter EarningTypesPage declares as a company-level module.
func (r *gormEarningTypeRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyCompany("earning_type.companies_id"))
}

// Find looks up an earning type without any scope filter. The cycle-earning rules
// compare its company against the cycle's own, so they need to tell "no such row"
// apart from "belongs to another company".
func (r *gormEarningTypeRepository) Find(id uint) (models.EarningType, error) {
	var earningType models.EarningType
	if err := r.db.First(&earningType, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.EarningType{}, ErrEarningTypeMissing
		}
		return models.EarningType{}, err
	}
	return earningType, nil
}

func (r *gormEarningTypeRepository) ListScoped(scope AccessScope) ([]models.EarningType, error) {
	var earnings []models.EarningType
	err := r.scoped(scope).Preload("Companies").Find(&earnings).Error
	return earnings, err
}

// ListMinimalScoped skips the relations, for the dropdown options endpoint that needs id/name only.
func (r *gormEarningTypeRepository) ListMinimalScoped(scope AccessScope) ([]models.EarningType, error) {
	var earnings []models.EarningType
	err := r.scoped(scope).Find(&earnings).Error
	return earnings, err
}

func (r *gormEarningTypeRepository) FindScoped(scope AccessScope, id uint) (models.EarningType, error) {
	var earningType models.EarningType
	if err := r.scoped(scope).First(&earningType, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.EarningType{}, ErrEarningTypeDenied
		}
		return models.EarningType{}, err
	}
	return earningType, nil
}

func (r *gormEarningTypeRepository) Create(earningType *models.EarningType) error {
	return r.db.Create(earningType).Error
}

// UpdateFields writes an explicit map so a false boolean is stored rather than skipped as a zero value.
func (r *gormEarningTypeRepository) UpdateFields(earningType *models.EarningType, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Model(earningType).Updates(fields).Error
}

func (r *gormEarningTypeRepository) Delete(earningType *models.EarningType) error {
	return r.db.Delete(earningType).Error
}

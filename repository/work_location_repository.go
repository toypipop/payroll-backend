package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/config"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// ErrWorkLocationNotFound covers both "no such row" and "out of the caller's scope".
var ErrWorkLocationNotFound = apperr.NotFound("Work location not found or unauthorized")

// WorkLocationRepository is the persistence boundary for the work-locations domain.
type WorkLocationRepository interface {
	ListScoped(scope AccessScope) ([]models.WorkLocation, error)
	ListMinimalScoped(scope AccessScope) ([]models.WorkLocation, error)
	FindScoped(scope AccessScope, id uint) (models.WorkLocation, error)
	Create(location *models.WorkLocation) error
	Save(location *models.WorkLocation) error
	Delete(location *models.WorkLocation) error
}

type gormWorkLocationRepository struct {
	db *gorm.DB
}

// NewWorkLocationRepository returns the GORM-backed implementation bound to the process-wide connection.
func NewWorkLocationRepository() WorkLocationRepository {
	return &gormWorkLocationRepository{db: config.DB}
}

// scoped filters by authorized company, the filter WorkLocationPage declares as a company-level module.
func (r *gormWorkLocationRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyCompany("work_location.companies_id"))
}

func (r *gormWorkLocationRepository) ListScoped(scope AccessScope) ([]models.WorkLocation, error) {
	var locations []models.WorkLocation
	err := r.scoped(scope).Preload("Companies").Order("name").Find(&locations).Error
	return locations, err
}

// ListMinimalScoped skips the relations, for the dropdown options endpoint that needs id/name only.
func (r *gormWorkLocationRepository) ListMinimalScoped(scope AccessScope) ([]models.WorkLocation, error) {
	var locations []models.WorkLocation
	err := r.scoped(scope).Order("name").Find(&locations).Error
	return locations, err
}

func (r *gormWorkLocationRepository) FindScoped(scope AccessScope, id uint) (models.WorkLocation, error) {
	var location models.WorkLocation
	if err := r.scoped(scope).Where("id = ?", id).First(&location).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.WorkLocation{}, ErrWorkLocationNotFound
		}
		return models.WorkLocation{}, err
	}
	return location, nil
}

func (r *gormWorkLocationRepository) Create(location *models.WorkLocation) error {
	return r.db.Create(location).Error
}

func (r *gormWorkLocationRepository) Save(location *models.WorkLocation) error {
	return r.db.Save(location).Error
}

func (r *gormWorkLocationRepository) Delete(location *models.WorkLocation) error {
	return r.db.Delete(location).Error
}

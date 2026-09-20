package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// ErrWorkLocationNotFound covers both "no such row" and "out of the caller's scope".
var ErrWorkLocationNotFound = apperr.NotFound("Work location not found or unauthorized")

// WorkLocationRepository is the persistence boundary for the work-locations domain.
type WorkLocationRepository interface {
	ListScoped(scope AccessScope, q PageQuery) (Page[models.WorkLocation], error)
	ListMinimalScoped(scope AccessScope) ([]models.WorkLocation, error)
	FindScoped(scope AccessScope, id uint) (models.WorkLocation, error)
	Create(location *models.WorkLocation, audit *models.AuditLog) error
	Save(location *models.WorkLocation, audit *models.AuditLog) error
	Delete(location *models.WorkLocation, audit *models.AuditLog) error
}

type gormWorkLocationRepository struct {
	db *gorm.DB
}

// NewWorkLocationRepository returns the GORM-backed implementation on the given connection.
func NewWorkLocationRepository(db *gorm.DB) WorkLocationRepository {
	return &gormWorkLocationRepository{db: db}
}

// scoped filters by authorized company, the filter WorkLocationPage declares as a company-level module.
func (r *gormWorkLocationRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyCompany("work_location.companies_id"))
}

func (r *gormWorkLocationRepository) ListScoped(scope AccessScope, q PageQuery) (Page[models.WorkLocation], error) {
	query := r.scoped(scope).Joins("LEFT JOIN companies c ON c.id = work_location.companies_id AND c.deleted_at = 0").Preload("Companies")
	return readCatalogPage[models.WorkLocation](query, q, map[string]string{"companies.name": "c.name"})
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

func (r *gormWorkLocationRepository) Create(location *models.WorkLocation, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(location).Error; err != nil {
			return err
		}
		if audit != nil && audit.EntityID != nil {
			*audit.EntityID = location.ID
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormWorkLocationRepository) Save(location *models.WorkLocation, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(location).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormWorkLocationRepository) Delete(location *models.WorkLocation, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(location).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

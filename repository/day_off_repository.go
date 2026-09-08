package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// ErrDayOffNotFound covers both "no such row" and "out of the caller's scope".
var ErrDayOffNotFound = apperr.NotFound("Day off not found or unauthorized")

// DayOffRepository is the persistence boundary for the day-offs domain.
type DayOffRepository interface {
	ListScoped(scope AccessScope) ([]models.DayOff, error)
	ListMinimalScoped(scope AccessScope) ([]models.DayOff, error)
	FindScoped(scope AccessScope, id uint) (models.DayOff, error)
	Create(dayOff *models.DayOff, audit *models.AuditLog) error
	Save(dayOff *models.DayOff, audit *models.AuditLog) error
	Delete(dayOff *models.DayOff, audit *models.AuditLog) error
}

type gormDayOffRepository struct {
	db *gorm.DB
}

// NewDayOffRepository returns the GORM-backed implementation on the given connection.
func NewDayOffRepository(db *gorm.DB) DayOffRepository {
	return &gormDayOffRepository{db: db}
}

// scoped filters by authorized company, the filter DayOffPage declares as a company-level module.
func (r *gormDayOffRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyCompany("day_off.companies_id"))
}

func (r *gormDayOffRepository) ListScoped(scope AccessScope) ([]models.DayOff, error) {
	var dayOffs []models.DayOff
	err := r.scoped(scope).Preload("Companies").Order("weekday ASC, name ASC").Find(&dayOffs).Error
	return dayOffs, err
}

// ListMinimalScoped skips the relations, for the dropdown options endpoint.
func (r *gormDayOffRepository) ListMinimalScoped(scope AccessScope) ([]models.DayOff, error) {
	var dayOffs []models.DayOff
	err := r.scoped(scope).Find(&dayOffs).Error
	return dayOffs, err
}

func (r *gormDayOffRepository) FindScoped(scope AccessScope, id uint) (models.DayOff, error) {
	var dayOff models.DayOff
	if err := r.scoped(scope).Where("id = ?", id).First(&dayOff).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.DayOff{}, ErrDayOffNotFound
		}
		return models.DayOff{}, err
	}
	return dayOff, nil
}

func (r *gormDayOffRepository) Create(dayOff *models.DayOff, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(dayOff).Error; err != nil {
			return err
		}
		if audit != nil && audit.EntityID != nil {
			*audit.EntityID = dayOff.ID
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormDayOffRepository) Save(dayOff *models.DayOff, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(dayOff).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormDayOffRepository) Delete(dayOff *models.DayOff, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(dayOff).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// ErrHolidayNotFound covers both "no such row" and "out of the caller's scope".
var ErrHolidayNotFound = apperr.NotFound("Holiday not found or unauthorized")

// HolidayRepository is the persistence boundary for the holidays domain.
type HolidayRepository interface {
	ListScoped(scope AccessScope, q PageQuery) (Page[models.Holiday], error)
	FindScoped(scope AccessScope, id uint) (models.Holiday, error)
	Create(holiday *models.Holiday, audit *models.AuditLog) error
	Save(holiday *models.Holiday, audit *models.AuditLog) error
	Delete(holiday *models.Holiday, audit *models.AuditLog) error
}

type gormHolidayRepository struct {
	db *gorm.DB
}

// NewHolidayRepository returns the GORM-backed implementation on the given connection.
func NewHolidayRepository(db *gorm.DB) HolidayRepository {
	return &gormHolidayRepository{db: db}
}

// scoped filters by authorized company, the filter HolidayCalendarPage declares as a company-level module.
func (r *gormHolidayRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyCompany("holiday.companies_id"))
}

func (r *gormHolidayRepository) ListScoped(scope AccessScope, q PageQuery) (Page[models.Holiday], error) {
	query := r.scoped(scope).Joins("LEFT JOIN companies c ON c.id = holiday.companies_id AND c.deleted_at = 0").Preload("Companies")
	return readCatalogPage[models.Holiday](query, q, map[string]string{"companies.name": "c.name"})
}

func (r *gormHolidayRepository) FindScoped(scope AccessScope, id uint) (models.Holiday, error) {
	var holiday models.Holiday
	if err := r.scoped(scope).Where("id = ?", id).First(&holiday).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Holiday{}, ErrHolidayNotFound
		}
		return models.Holiday{}, err
	}
	return holiday, nil
}

func (r *gormHolidayRepository) Create(holiday *models.Holiday, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(holiday).Error; err != nil {
			return err
		}
		if audit != nil && audit.EntityID != nil {
			*audit.EntityID = holiday.ID
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormHolidayRepository) Save(holiday *models.Holiday, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(holiday).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormHolidayRepository) Delete(holiday *models.Holiday, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(holiday).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

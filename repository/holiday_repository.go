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
	ListScoped(scope AccessScope) ([]models.Holiday, error)
	FindScoped(scope AccessScope, id uint) (models.Holiday, error)
	Create(holiday *models.Holiday) error
	Save(holiday *models.Holiday) error
	Delete(holiday *models.Holiday) error
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

func (r *gormHolidayRepository) ListScoped(scope AccessScope) ([]models.Holiday, error) {
	var holidays []models.Holiday
	err := r.scoped(scope).Preload("Companies").Order("date").Find(&holidays).Error
	return holidays, err
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

func (r *gormHolidayRepository) Create(holiday *models.Holiday) error {
	return r.db.Create(holiday).Error
}

func (r *gormHolidayRepository) Save(holiday *models.Holiday) error {
	return r.db.Save(holiday).Error
}

func (r *gormHolidayRepository) Delete(holiday *models.Holiday) error {
	return r.db.Delete(holiday).Error
}

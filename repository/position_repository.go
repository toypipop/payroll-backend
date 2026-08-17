package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// ErrPositionNotFound covers both "no such row" and "out of the caller's scope".
var ErrPositionNotFound = apperr.NotFound("Position not found or unauthorized")

// PositionRepository is the persistence boundary for the positions domain.
type PositionRepository interface {
	ListScoped(scope AccessScope) ([]models.Position, error)
	ListMinimalScoped(scope AccessScope) ([]models.Position, error)
	FindScoped(scope AccessScope, id uint) (models.Position, error)
	Create(position *models.Position) error
	Save(position *models.Position) error
	Delete(position *models.Position) error
}

type gormPositionRepository struct {
	db *gorm.DB
}

// NewPositionRepository returns the GORM-backed implementation on the given connection.
func NewPositionRepository(db *gorm.DB) PositionRepository {
	return &gormPositionRepository{db: db}
}

// scoped filters by authorized company, the filter PositionSettings declares as a company-level module.
func (r *gormPositionRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyCompany("position.companies_id"))
}

func (r *gormPositionRepository) ListScoped(scope AccessScope) ([]models.Position, error) {
	var positions []models.Position
	err := r.scoped(scope).Preload("Companies").Find(&positions).Error
	return positions, err
}

// ListMinimalScoped skips the relations, for the dropdown options endpoint that needs id/name only.
func (r *gormPositionRepository) ListMinimalScoped(scope AccessScope) ([]models.Position, error) {
	var positions []models.Position
	err := r.scoped(scope).Find(&positions).Error
	return positions, err
}

func (r *gormPositionRepository) FindScoped(scope AccessScope, id uint) (models.Position, error) {
	var position models.Position
	if err := r.scoped(scope).Where("id = ?", id).First(&position).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Position{}, ErrPositionNotFound
		}
		return models.Position{}, err
	}
	return position, nil
}

func (r *gormPositionRepository) Create(position *models.Position) error {
	return r.db.Create(position).Error
}

func (r *gormPositionRepository) Save(position *models.Position) error {
	return r.db.Save(position).Error
}

func (r *gormPositionRepository) Delete(position *models.Position) error {
	return r.db.Delete(position).Error
}

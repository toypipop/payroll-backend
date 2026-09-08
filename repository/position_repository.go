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
	Create(position *models.Position, audit *models.AuditLog) error
	Save(position *models.Position, audit *models.AuditLog) error
	Delete(position *models.Position, audit *models.AuditLog) error
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

func (r *gormPositionRepository) Create(position *models.Position, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(position).Error; err != nil {
			return err
		}
		if audit != nil && audit.EntityID != nil {
			*audit.EntityID = position.ID
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormPositionRepository) Save(position *models.Position, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(position).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormPositionRepository) Delete(position *models.Position, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(position).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

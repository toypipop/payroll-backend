package repository

import (
	"time"

	"payroll-backend/models"

	"gorm.io/gorm"
)

// SessionRepository persists server-side session revocation state.
type SessionRepository interface {
	Create(session *models.AuthSession) error
	IsActive(tokenID string, employeeID uint, now time.Time) (bool, error)
	Revoke(tokenID string, employeeID uint, now time.Time) error
	RevokeAll(employeeID uint, now time.Time) error
	DeleteExpired(now time.Time) error
}

type gormSessionRepository struct {
	db *gorm.DB
}

func NewSessionRepository(db *gorm.DB) SessionRepository {
	return &gormSessionRepository{db: db}
}

func (r *gormSessionRepository) Create(session *models.AuthSession) error {
	return r.db.Create(session).Error
}

func (r *gormSessionRepository) IsActive(tokenID string, employeeID uint, now time.Time) (bool, error) {
	var count int64
	err := r.db.Model(&models.AuthSession{}).
		Where("token_id = ? AND employee_id = ? AND revoked_at IS NULL AND expires_at > ?", tokenID, employeeID, now).
		Count(&count).Error
	return count == 1, err
}

func (r *gormSessionRepository) Revoke(tokenID string, employeeID uint, now time.Time) error {
	return r.db.Model(&models.AuthSession{}).
		Where("token_id = ? AND employee_id = ? AND revoked_at IS NULL", tokenID, employeeID).
		Update("revoked_at", now).Error
}

func (r *gormSessionRepository) RevokeAll(employeeID uint, now time.Time) error {
	return r.db.Model(&models.AuthSession{}).
		Where("employee_id = ? AND revoked_at IS NULL", employeeID).
		Update("revoked_at", now).Error
}

func (r *gormSessionRepository) DeleteExpired(now time.Time) error {
	return r.db.Where("expires_at <= ?", now).Delete(&models.AuthSession{}).Error
}

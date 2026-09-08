package repository

import (
	"payroll-backend/models"

	"gorm.io/gorm"
)

// AuditLogRepository is append-only persistence for successful business events.
type AuditLogRepository interface {
	Append(entry *models.AuditLog) error
	ListScoped(scope AccessScope, companyID *uint) ([]models.AuditLog, error)
}

type gormAuditLogRepository struct{ db *gorm.DB }

func NewAuditLogRepository(db *gorm.DB) AuditLogRepository { return &gormAuditLogRepository{db: db} }

func (r *gormAuditLogRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Model(&models.AuditLog{}).Scopes(scope.ApplyCompany("audit_log.companies_id"))
}

func (r *gormAuditLogRepository) Append(entry *models.AuditLog) error {
	return r.db.Create(entry).Error
}

func (r *gormAuditLogRepository) ListScoped(scope AccessScope, companyID *uint) ([]models.AuditLog, error) {
	var rows []models.AuditLog
	query := r.scoped(scope)
	if companyID != nil {
		query = query.Where("audit_log.companies_id = ?", *companyID)
	}
	err := query.Order("occurred_at DESC, id DESC").Find(&rows).Error
	return rows, err
}

package repository

import (
	"payroll-backend/models"

	"gorm.io/gorm"
)

// AuditLogScope selects either company-owned events or global reference-data events.
type AuditLogScope string

const AuditLogScopeShared AuditLogScope = "shared"

// AuditLogQuery keeps company and shared-data filtering mutually exclusive.
type AuditLogQuery struct {
	CompanyID *uint
	Scope     AuditLogScope
}

// AuditLogRepository is append-only persistence for successful business events.
type AuditLogRepository interface {
	Append(entry *models.AuditLog) error
	ListScoped(scope AccessScope, query AuditLogQuery) ([]models.AuditLog, error)
}

type gormAuditLogRepository struct{ db *gorm.DB }

func NewAuditLogRepository(db *gorm.DB) AuditLogRepository { return &gormAuditLogRepository{db: db} }

func (r *gormAuditLogRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Model(&models.AuditLog{}).Scopes(scope.ApplyCompany("audit_log.companies_id"))
}

func (r *gormAuditLogRepository) Append(entry *models.AuditLog) error {
	return r.db.Create(entry).Error
}

func (r *gormAuditLogRepository) ListScoped(scope AccessScope, filter AuditLogQuery) ([]models.AuditLog, error) {
	var rows []models.AuditLog
	query := r.scoped(scope)
	if filter.Scope == AuditLogScopeShared {
		query = r.db.Model(&models.AuditLog{}).Where("audit_log.companies_id IS NULL")
	} else if filter.CompanyID != nil {
		query = query.Where("audit_log.companies_id = ?", *filter.CompanyID)
	}
	err := query.Order("occurred_at DESC, id DESC").Find(&rows).Error
	return rows, err
}

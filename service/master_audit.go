package service

import (
	"payroll-backend/models"
	"payroll-backend/repository"
)

// masterAudit keeps company-scoped setup changes consistent and intentionally records only setup values.
func masterAudit(scope repository.AccessScope, event models.AuditEventType, summary, entityType string, companyID uint, entityID *uint, before, after map[string]any) (models.AuditLog, error) {
	return BuildAuditLog(scope, models.AuditLogInput{CompaniesID: &companyID, EventType: event, EntityType: entityType, EntityID: entityID, Summary: summary, BeforeData: before, AfterData: after})
}

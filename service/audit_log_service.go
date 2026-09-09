package service

import (
	"strings"

	"payroll-backend/models"
	"payroll-backend/repository"
)

// AuditLogService owns actor snapshots and safe event construction.
type AuditLogService struct{ repo repository.AuditLogRepository }

func NewAuditLogService(repo repository.AuditLogRepository) *AuditLogService {
	return &AuditLogService{repo: repo}
}

// BuildAuditLog snapshots the authenticated actor and validates safe structured data. Transaction-owning
// services use it before appending through their transaction-bound repository.
func BuildAuditLog(scope repository.AccessScope, input models.AuditLogInput) (models.AuditLog, error) {
	actorID := scope.User.ID
	input.ActorUserID = &actorID
	input.ActorNameSnapshot = strings.TrimSpace(scope.User.FirstName + " " + scope.User.LastName)
	if input.ActorNameSnapshot == "" {
		input.ActorNameSnapshot = "Unknown"
	}
	return models.NewAuditLogInput(input)
}

func (s *AuditLogService) Record(scope repository.AccessScope, input models.AuditLogInput) error {
	entry, err := BuildAuditLog(scope, input)
	if err != nil {
		return err
	}
	return s.repo.Append(&entry)
}

func (s *AuditLogService) List(scope repository.AccessScope, filter repository.AuditLogQuery) ([]models.AuditLog, error) {
	if filter.CompanyID != nil && !scope.CanAccessCompany(*filter.CompanyID) {
		return nil, repository.ErrSalaryEmployeeDenied
	}
	return s.repo.ListScoped(scope, filter)
}

package service

import (
	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// ErrPositionNameRequired means the submitted name was empty or only whitespace.
var ErrPositionNameRequired = apperr.Validation("pos_name is required")

// ErrPositionNotFound is re-exported so handlers import a single package.
var ErrPositionNotFound = repository.ErrPositionNotFound

// PositionService owns the position rules: which company a row may belong to, and that its company never changes afterwards.
type PositionService struct {
	repo repository.PositionRepository
}

// NewPositionService wires the service to a repository implementation.
func NewPositionService(repo repository.PositionRepository) *PositionService {
	return &PositionService{repo: repo}
}

// List returns the positions in the caller's authorized companies, with relations loaded.
func (s *PositionService) List(scope repository.AccessScope, q repository.PageQuery) (repository.Page[models.Position], error) {
	return s.repo.ListScoped(scope, q)
}

// Options returns the same rows without relations, for cross-page dropdowns.
func (s *PositionService) Options(scope repository.AccessScope) ([]models.Position, error) {
	return s.repo.ListMinimalScoped(scope)
}

// Create adds a position under a company the caller is authorized for.
func (s *PositionService) Create(scope repository.AccessScope, input models.Position) (models.Position, error) {
	name := trimText(input.PosName)
	if name == "" {
		return models.Position{}, ErrPositionNameRequired
	}
	if err := validateText("name", name, 1, 255); err != nil {
		return models.Position{}, err
	}

	companyID, ok := scope.ResolveCompanyID(input.CompaniesID)
	if !ok {
		return models.Position{}, ErrCompanyAccessDenied
	}

	position := models.Position{CompaniesID: companyID, PosName: name}
	audit, err := masterAudit(scope, models.AuditEventPositionCreated, "Position created", "position", position.CompaniesID, &position.ID, nil, positionAuditData(position))
	if err != nil {
		return models.Position{}, err
	}
	if err := s.repo.Create(&position, &audit); err != nil {
		return models.Position{}, err
	}
	return position, nil
}

// Update renames a position. The company is immutable after creation, so it is never taken from input.
func (s *PositionService) Update(scope repository.AccessScope, id uint, name string) (models.Position, error) {
	trimmed := trimText(name)
	if trimmed == "" {
		return models.Position{}, ErrPositionNameRequired
	}
	if err := validateText("name", trimmed, 1, 255); err != nil {
		return models.Position{}, err
	}

	position, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.Position{}, err
	}

	before := positionAuditData(position)
	position.PosName = trimmed
	audit, err := masterAudit(scope, models.AuditEventPositionUpdated, "Position updated", "position", position.CompaniesID, &position.ID, before, positionAuditData(position))
	if err != nil {
		return models.Position{}, err
	}
	if err := s.repo.Save(&position, &audit); err != nil {
		return models.Position{}, err
	}
	return position, nil
}

// Delete removes a position the caller is scoped to see; reference checks run in middleware before this.
func (s *PositionService) Delete(scope repository.AccessScope, id uint) error {
	position, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return err
	}
	audit, err := masterAudit(scope, models.AuditEventPositionDeleted, "Position deleted", "position", position.CompaniesID, &position.ID, positionAuditData(position), positionAuditData(position))
	if err != nil {
		return err
	}
	return s.repo.Delete(&position, &audit)
}

func positionAuditData(position models.Position) map[string]any {
	return map[string]any{"name": position.PosName}
}

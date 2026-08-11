package service

import (
	"strings"

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
func (s *PositionService) List(scope repository.AccessScope) ([]models.Position, error) {
	return s.repo.ListScoped(scope)
}

// Options returns the same rows without relations, for cross-page dropdowns.
func (s *PositionService) Options(scope repository.AccessScope) ([]models.Position, error) {
	return s.repo.ListMinimalScoped(scope)
}

// Create adds a position under a company the caller is authorized for.
func (s *PositionService) Create(scope repository.AccessScope, input models.Position) (models.Position, error) {
	name := strings.TrimSpace(input.PosName)
	if name == "" {
		return models.Position{}, ErrPositionNameRequired
	}

	companyID, ok := scope.ResolveCompanyID(input.CompaniesID)
	if !ok {
		return models.Position{}, ErrCompanyAccessDenied
	}

	position := models.Position{CompaniesID: companyID, PosName: name}
	if err := s.repo.Create(&position); err != nil {
		return models.Position{}, err
	}
	return position, nil
}

// Update renames a position. The company is immutable after creation, so it is never taken from input.
func (s *PositionService) Update(scope repository.AccessScope, id uint, name string) (models.Position, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return models.Position{}, ErrPositionNameRequired
	}

	position, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.Position{}, err
	}

	position.PosName = trimmed
	if err := s.repo.Save(&position); err != nil {
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
	return s.repo.Delete(&position)
}

package service

import (
	"strings"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// ErrWorkLocationNameRequired means the submitted name was empty or only whitespace.
var ErrWorkLocationNameRequired = apperr.Validation("name is required")

// ErrWorkLocationNotFound is re-exported so handlers import a single package.
var ErrWorkLocationNotFound = repository.ErrWorkLocationNotFound

// WorkLocationService owns the work-location rules: which company a row may belong to, and that its company never changes afterwards.
type WorkLocationService struct {
	repo repository.WorkLocationRepository
}

// NewWorkLocationService wires the service to a repository implementation.
func NewWorkLocationService(repo repository.WorkLocationRepository) *WorkLocationService {
	return &WorkLocationService{repo: repo}
}

// List returns the work locations of the caller's authorized companies, with relations loaded.
func (s *WorkLocationService) List(scope repository.AccessScope) ([]models.WorkLocation, error) {
	return s.repo.ListScoped(scope)
}

// Options returns the same rows without relations, for cross-page dropdowns.
func (s *WorkLocationService) Options(scope repository.AccessScope) ([]models.WorkLocation, error) {
	return s.repo.ListMinimalScoped(scope)
}

// Create adds a work location under a company the caller is authorized for.
func (s *WorkLocationService) Create(scope repository.AccessScope, input models.WorkLocation) (models.WorkLocation, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return models.WorkLocation{}, ErrWorkLocationNameRequired
	}

	companyID, ok := scope.ResolveCompanyID(input.CompaniesID)
	if !ok {
		return models.WorkLocation{}, ErrCompanyAccessDenied
	}

	location := models.WorkLocation{CompaniesID: companyID, Name: name}
	if err := s.repo.Create(&location); err != nil {
		return models.WorkLocation{}, err
	}
	return location, nil
}

// Update renames a work location. The company is immutable after creation.
func (s *WorkLocationService) Update(scope repository.AccessScope, id uint, name string) (models.WorkLocation, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return models.WorkLocation{}, ErrWorkLocationNameRequired
	}

	location, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.WorkLocation{}, err
	}

	location.Name = trimmed
	if err := s.repo.Save(&location); err != nil {
		return models.WorkLocation{}, err
	}
	return location, nil
}

// Delete removes a work location the caller is scoped to see; reference checks run in middleware before this.
func (s *WorkLocationService) Delete(scope repository.AccessScope, id uint) error {
	location, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return err
	}
	return s.repo.Delete(&location)
}

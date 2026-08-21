package service

import (
	"strings"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// ErrEarningTypeNameRequired means the submitted name was empty or only whitespace.
var ErrEarningTypeNameRequired = apperr.Validation("name is required")

// Re-exported so handlers import a single package.
var (
	ErrEarningTypeDenied       = repository.ErrEarningTypeDenied
	ErrEarningTypeDeleteDenied = repository.ErrEarningTypeDeleteDenied
)

// EarningTypeService owns the earning-type rules: which company a row may belong to, and that its company never changes afterwards.
type EarningTypeService struct {
	repo repository.EarningTypeRepository
}

// NewEarningTypeService wires the service to a repository implementation.
func NewEarningTypeService(repo repository.EarningTypeRepository) *EarningTypeService {
	return &EarningTypeService{repo: repo}
}

// List returns the earning types of the caller's authorized companies, with relations loaded.
func (s *EarningTypeService) List(scope repository.AccessScope) ([]models.EarningType, error) {
	return s.repo.ListScoped(scope)
}

// Options returns the same rows without relations, for cross-page dropdowns.
func (s *EarningTypeService) Options(scope repository.AccessScope) ([]models.EarningType, error) {
	return s.repo.ListMinimalScoped(scope)
}

// Create adds an earning type to an explicitly named company; this domain has never defaulted to the caller's home company.
func (s *EarningTypeService) Create(scope repository.AccessScope, input models.EarningType) (models.EarningType, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return models.EarningType{}, ErrEarningTypeNameRequired
	}
	if !scope.CanAccessCompany(input.CompaniesID) {
		return models.EarningType{}, ErrCompanyAccessDenied
	}

	earningType := models.EarningType{
		CompaniesID: input.CompaniesID,
		Name:        name,
		IsTaxable:   input.IsTaxable,
		IsSso:       input.IsSso,
		IsPermanent: input.IsPermanent,
	}
	if err := s.repo.Create(&earningType); err != nil {
		return models.EarningType{}, err
	}
	return earningType, nil
}

// Update rewrites the flags and name.
func (s *EarningTypeService) Update(scope repository.AccessScope, id uint, input models.EarningType) (models.EarningType, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return models.EarningType{}, ErrEarningTypeNameRequired
	}

	earningType, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.EarningType{}, err
	}

	fields := map[string]any{
		"name":         name,
		"is_taxable":   input.IsTaxable,
		"is_sso":       input.IsSso,
		"is_permanent": input.IsPermanent,
	}
	if err := s.repo.UpdateFields(&earningType, fields); err != nil {
		return models.EarningType{}, err
	}
	return earningType, nil
}

// Delete removes an earning type the caller is scoped to see; reference checks run in middleware before this.
func (s *EarningTypeService) Delete(scope repository.AccessScope, id uint) error {
	earningType, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return ErrEarningTypeDeleteDenied
	}
	return s.repo.Delete(&earningType)
}

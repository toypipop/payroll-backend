package service

import (
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
func (s *EarningTypeService) List(scope repository.AccessScope, q repository.PageQuery) (repository.Page[models.EarningType], error) {
	return s.repo.ListScoped(scope, q)
}

// Options returns the same rows without relations, for cross-page dropdowns.
func (s *EarningTypeService) Options(scope repository.AccessScope) ([]models.EarningType, error) {
	return s.repo.ListMinimalScoped(scope)
}

// Create adds an earning type to an explicitly named company; this domain has never defaulted to the caller's home company.
func (s *EarningTypeService) Create(scope repository.AccessScope, input models.EarningType) (models.EarningType, error) {
	name := trimText(input.Name)
	if name == "" {
		return models.EarningType{}, ErrEarningTypeNameRequired
	}
	if err := validateText("name", name, 1, 200); err != nil {
		return models.EarningType{}, err
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
	audit, err := masterAudit(scope, models.AuditEventEarningTypeCreated, "Earning type created", "earning_type", earningType.CompaniesID, &earningType.ID, nil, earningTypeAuditData(earningType))
	if err != nil {
		return models.EarningType{}, err
	}
	if err := s.repo.Create(&earningType, &audit); err != nil {
		return models.EarningType{}, err
	}
	return earningType, nil
}

// Update rewrites the flags and name.
func (s *EarningTypeService) Update(scope repository.AccessScope, id uint, input models.EarningType) (models.EarningType, error) {
	name := trimText(input.Name)
	if name == "" {
		return models.EarningType{}, ErrEarningTypeNameRequired
	}
	if err := validateText("name", name, 1, 200); err != nil {
		return models.EarningType{}, err
	}

	earningType, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.EarningType{}, err
	}

	before := earningTypeAuditData(earningType)
	updated := earningType
	updated.Name, updated.IsTaxable, updated.IsSso, updated.IsPermanent = name, input.IsTaxable, input.IsSso, input.IsPermanent
	fields := map[string]any{
		"name":         name,
		"is_taxable":   input.IsTaxable,
		"is_sso":       input.IsSso,
		"is_permanent": input.IsPermanent,
	}
	audit, err := masterAudit(scope, models.AuditEventEarningTypeUpdated, "Earning type updated", "earning_type", earningType.CompaniesID, &earningType.ID, before, earningTypeAuditData(updated))
	if err != nil {
		return models.EarningType{}, err
	}
	if err := s.repo.UpdateFields(&earningType, fields, &audit); err != nil {
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
	audit, err := masterAudit(scope, models.AuditEventEarningTypeDeleted, "Earning type deleted", "earning_type", earningType.CompaniesID, &earningType.ID, earningTypeAuditData(earningType), earningTypeAuditData(earningType))
	if err != nil {
		return err
	}
	return s.repo.Delete(&earningType, &audit)
}

func earningTypeAuditData(value models.EarningType) map[string]any {
	return map[string]any{"name": value.Name, "is_taxable": value.IsTaxable, "is_sso": value.IsSso, "is_permanent": value.IsPermanent}
}

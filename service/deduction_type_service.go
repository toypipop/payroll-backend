package service

import (
	"strings"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// ErrDeductionTypeNameRequired means the submitted name was empty or only whitespace.
var ErrDeductionTypeNameRequired = apperr.Validation("name is required")

// Re-exported so handlers import a single package.
var (
	ErrDeductionTypeDenied       = repository.ErrDeductionTypeDenied
	ErrDeductionTypeDeleteDenied = repository.ErrDeductionTypeDeleteDenied
)

// DeductionTypeService owns the deduction-type rules: which company a row may belong to, and that its company never changes afterwards.
type DeductionTypeService struct {
	repo repository.DeductionTypeRepository
}

// NewDeductionTypeService wires the service to a repository implementation.
func NewDeductionTypeService(repo repository.DeductionTypeRepository) *DeductionTypeService {
	return &DeductionTypeService{repo: repo}
}

// List returns the deduction types of the caller's authorized companies, with relations loaded.
func (s *DeductionTypeService) List(scope repository.AccessScope, q repository.PageQuery) (repository.Page[models.DeductionType], error) {
	return s.repo.ListScoped(scope, q)
}

// Options returns the same rows without relations, for cross-page dropdowns.
func (s *DeductionTypeService) Options(scope repository.AccessScope) ([]models.DeductionType, error) {
	return s.repo.ListMinimalScoped(scope)
}

// Create adds a deduction type to an explicitly named company; this domain has never defaulted to the caller's home company.
func (s *DeductionTypeService) Create(scope repository.AccessScope, input models.DeductionType) (models.DeductionType, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return models.DeductionType{}, ErrDeductionTypeNameRequired
	}
	if !scope.CanAccessCompany(input.CompaniesID) {
		return models.DeductionType{}, ErrCompanyAccessDenied
	}

	deductionType := models.DeductionType{
		CompaniesID:     input.CompaniesID,
		Name:            name,
		IsTaxDeductible: input.IsTaxDeductible,
		IsSsoDeductible: input.IsSsoDeductible,
	}
	audit, err := masterAudit(scope, models.AuditEventDeductionTypeCreated, "Deduction type created", "deduction_type", deductionType.CompaniesID, &deductionType.ID, nil, deductionTypeAuditData(deductionType))
	if err != nil {
		return models.DeductionType{}, err
	}
	if err := s.repo.Create(&deductionType, &audit); err != nil {
		return models.DeductionType{}, err
	}
	return deductionType, nil
}

// Update rewrites the flags and name.
func (s *DeductionTypeService) Update(scope repository.AccessScope, id uint, input models.DeductionType) (models.DeductionType, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return models.DeductionType{}, ErrDeductionTypeNameRequired
	}

	deductionType, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.DeductionType{}, err
	}

	before := deductionTypeAuditData(deductionType)
	updated := deductionType
	updated.Name, updated.IsTaxDeductible, updated.IsSsoDeductible = name, input.IsTaxDeductible, input.IsSsoDeductible
	fields := map[string]any{
		"name":              name,
		"is_tax_deductible": input.IsTaxDeductible,
		"is_sso_deductible": input.IsSsoDeductible,
	}
	audit, err := masterAudit(scope, models.AuditEventDeductionTypeUpdated, "Deduction type updated", "deduction_type", deductionType.CompaniesID, &deductionType.ID, before, deductionTypeAuditData(updated))
	if err != nil {
		return models.DeductionType{}, err
	}
	if err := s.repo.UpdateFields(&deductionType, fields, &audit); err != nil {
		return models.DeductionType{}, err
	}
	return deductionType, nil
}

// Delete removes a deduction type the caller is scoped to see; reference checks run in middleware before this.
func (s *DeductionTypeService) Delete(scope repository.AccessScope, id uint) error {
	deductionType, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return ErrDeductionTypeDeleteDenied
	}
	audit, err := masterAudit(scope, models.AuditEventDeductionTypeDeleted, "Deduction type deleted", "deduction_type", deductionType.CompaniesID, &deductionType.ID, deductionTypeAuditData(deductionType), deductionTypeAuditData(deductionType))
	if err != nil {
		return err
	}
	return s.repo.Delete(&deductionType, &audit)
}

func deductionTypeAuditData(value models.DeductionType) map[string]any {
	return map[string]any{"name": value.Name, "is_tax_deductible": value.IsTaxDeductible, "is_sso_deductible": value.IsSsoDeductible}
}

package service

import (
	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// ErrEmploymentTypeNameRequired means the submitted name was empty or only whitespace.
var ErrEmploymentTypeNameRequired = apperr.Validation("name is required")

// ErrEmploymentTypeNotFound is re-exported so handlers import a single package.
var ErrEmploymentTypeNotFound = repository.ErrEmploymentTypeNotFound

// EmploymentTypeService owns the employment-type rules: which company a row may belong to, and that its company never changes afterwards.
type EmploymentTypeService struct {
	repo repository.EmploymentTypeRepository
}

// NewEmploymentTypeService wires the service to a repository implementation.
func NewEmploymentTypeService(repo repository.EmploymentTypeRepository) *EmploymentTypeService {
	return &EmploymentTypeService{repo: repo}
}

// List returns the employment types of the caller's authorized companies, with relations loaded.
func (s *EmploymentTypeService) List(scope repository.AccessScope, q repository.PageQuery) (repository.Page[models.EmploymentType], error) {
	return s.repo.ListScoped(scope, q)
}

// Options returns the same rows without relations, for cross-page dropdowns.
func (s *EmploymentTypeService) Options(scope repository.AccessScope) ([]models.EmploymentType, error) {
	return s.repo.ListMinimalScoped(scope)
}

// Create adds an employment type to an explicitly named company; this domain has never defaulted to the caller's home company.
func (s *EmploymentTypeService) Create(scope repository.AccessScope, input models.EmploymentType) (models.EmploymentType, error) {
	if err := validateText("description", trimText(input.Description), 0, 255); err != nil {
		return models.EmploymentType{}, err
	}
	name := trimText(input.Name)
	if name == "" {
		return models.EmploymentType{}, ErrEmploymentTypeNameRequired
	}
	if err := validateText("name", name, 1, 100); err != nil {
		return models.EmploymentType{}, err
	}
	if !scope.CanAccessCompany(input.CompaniesID) {
		return models.EmploymentType{}, ErrCompanyAccessDenied
	}

	employmentType := models.EmploymentType{
		CompaniesID: input.CompaniesID,
		Name:        name,
		Description: trimText(input.Description),
	}
	audit, err := masterAudit(scope, models.AuditEventEmploymentTypeCreated, "Employment type created", "employment_type", employmentType.CompaniesID, &employmentType.ID, nil, employmentTypeAuditData(employmentType))
	if err != nil {
		return models.EmploymentType{}, err
	}
	if err := s.repo.Create(&employmentType, &audit); err != nil {
		return models.EmploymentType{}, err
	}
	return employmentType, nil
}

// Update rewrites the name and description. The company is immutable after creation.
func (s *EmploymentTypeService) Update(scope repository.AccessScope, id uint, name string, description string) (models.EmploymentType, error) {
	if err := validateText("description", trimText(description), 0, 255); err != nil {
		return models.EmploymentType{}, err
	}
	trimmed := trimText(name)
	if trimmed == "" {
		return models.EmploymentType{}, ErrEmploymentTypeNameRequired
	}
	if err := validateText("name", trimmed, 1, 100); err != nil {
		return models.EmploymentType{}, err
	}

	employmentType, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.EmploymentType{}, err
	}

	before := employmentTypeAuditData(employmentType)
	employmentType.Name = trimmed
	employmentType.Description = trimText(description)
	audit, err := masterAudit(scope, models.AuditEventEmploymentTypeUpdated, "Employment type updated", "employment_type", employmentType.CompaniesID, &employmentType.ID, before, employmentTypeAuditData(employmentType))
	if err != nil {
		return models.EmploymentType{}, err
	}
	if err := s.repo.Save(&employmentType, &audit); err != nil {
		return models.EmploymentType{}, err
	}
	return employmentType, nil
}

// Delete removes an employment type the caller is scoped to see; reference checks run in middleware before this.
func (s *EmploymentTypeService) Delete(scope repository.AccessScope, id uint) error {
	employmentType, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return err
	}
	audit, err := masterAudit(scope, models.AuditEventEmploymentTypeDeleted, "Employment type deleted", "employment_type", employmentType.CompaniesID, &employmentType.ID, employmentTypeAuditData(employmentType), employmentTypeAuditData(employmentType))
	if err != nil {
		return err
	}
	return s.repo.Delete(&employmentType, &audit)
}

func employmentTypeAuditData(value models.EmploymentType) map[string]any {
	return map[string]any{"name": value.Name, "description": value.Description}
}

package service

import (
	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// Employee-company-access rule failures.
var (
	ErrAccessEmployeeMissing = apperr.Forbidden("employee not found")
	ErrAccessRoleDenied      = apperr.Forbidden("access denied to this role")
	ErrAccessRoleMismatch    = apperr.Forbidden("role does not belong to selected company")
)

// Required-field failures for the access mapping payload.
var (
	ErrAccessEmployeeRequired = apperr.Validation("employee_id is required")
	ErrAccessCompanyRequired  = apperr.Validation("company_id is required")
	ErrAccessRoleRequired     = apperr.Validation("role_id is required")
)

// ErrAccessMappingNotFound is re-exported so handlers import a single package.
var ErrAccessMappingNotFound = repository.ErrAccessMappingNotFound

// EmployeeCompanyAccessInput is the set of fields a caller may set on a mapping.
type EmployeeCompanyAccessInput struct {
	EmployeeID uint
	CompanyID  uint
	RoleID     uint
	IsActive   bool
}

// EmployeeCompanyAccessService owns the rules for granting an employee access to a company.
type EmployeeCompanyAccessService struct {
	repo  repository.EmployeeCompanyAccessRepository
	roles repository.RoleRepository
}

// NewEmployeeCompanyAccessService wires the service to repository implementations.
func NewEmployeeCompanyAccessService(
	repo repository.EmployeeCompanyAccessRepository,
	roles repository.RoleRepository,
) *EmployeeCompanyAccessService {
	return &EmployeeCompanyAccessService{repo: repo, roles: roles}
}

// List returns the mappings the caller's scope covers, with relations loaded.
func (s *EmployeeCompanyAccessService) List(scope repository.AccessScope) ([]models.EmployeeCompanyAccess, error) {
	return s.repo.ListScoped(scope)
}

// validate enforces every rule a mapping must satisfy before it is written.
func (s *EmployeeCompanyAccessService) validate(scope repository.AccessScope, input EmployeeCompanyAccessInput) error {
	if input.EmployeeID == 0 {
		return ErrAccessEmployeeRequired
	}
	if input.CompanyID == 0 {
		return ErrAccessCompanyRequired
	}
	if input.RoleID == 0 {
		return ErrAccessRoleRequired
	}

	if _, ok := scope.ResolveCompanyID(input.CompanyID); !ok {
		return ErrCompanyAccessDenied
	}
	employee, err := s.repo.FindEmployeeBrief(input.EmployeeID)
	if err != nil {
		return ErrAccessEmployeeMissing
	}
	if err := requireActiveEmployee(employee); err != nil {
		return err
	}

	role, err := s.roles.FindScoped(scope, input.RoleID)
	if err != nil {
		return ErrAccessRoleDenied
	}
	if role.CompaniesID != input.CompanyID {
		return ErrAccessRoleMismatch
	}
	return nil
}

// Create grants an employee access to a company under a role of that company.
func (s *EmployeeCompanyAccessService) Create(scope repository.AccessScope, input EmployeeCompanyAccessInput) (models.EmployeeCompanyAccess, error) {
	if err := s.validate(scope, input); err != nil {
		return models.EmployeeCompanyAccess{}, err
	}

	mapping := models.EmployeeCompanyAccess{
		EmployeeID: input.EmployeeID,
		CompanyID:  input.CompanyID,
		RoleID:     input.RoleID,
		IsActive:   input.IsActive,
	}
	audit, err := companyAccessAudit(scope, models.AuditEventEmployeeCompanyAccessSaved, "Employee company access granted", mapping, nil)
	if err != nil {
		return models.EmployeeCompanyAccess{}, err
	}
	if err := s.repo.Create(&mapping, &audit); err != nil {
		return models.EmployeeCompanyAccess{}, err
	}
	return s.repo.FindDetail(mapping.ID)
}

// Update repoints an existing mapping, re-running every rule against the new values.
func (s *EmployeeCompanyAccessService) Update(scope repository.AccessScope, id uint, input EmployeeCompanyAccessInput) (models.EmployeeCompanyAccess, error) {
	existing, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.EmployeeCompanyAccess{}, err
	}
	if err := s.validate(scope, input); err != nil {
		return models.EmployeeCompanyAccess{}, err
	}

	before := companyAccessAuditData(existing)
	updated := existing
	updated.EmployeeID, updated.CompanyID, updated.RoleID, updated.IsActive = input.EmployeeID, input.CompanyID, input.RoleID, input.IsActive
	fields := map[string]any{
		"employee_id": input.EmployeeID,
		"company_id":  input.CompanyID,
		"role_id":     input.RoleID,
		"is_active":   input.IsActive,
	}
	audit, err := companyAccessAudit(scope, models.AuditEventEmployeeCompanyAccessSaved, "Employee company access updated", updated, before)
	if err != nil {
		return models.EmployeeCompanyAccess{}, err
	}
	if err := s.repo.UpdateFields(&existing, fields, &audit); err != nil {
		return models.EmployeeCompanyAccess{}, err
	}
	return s.repo.FindDetail(existing.ID)
}

// Delete removes a mapping the caller's scope covers.
func (s *EmployeeCompanyAccessService) Delete(scope repository.AccessScope, id uint) error {
	mapping, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return err
	}
	audit, err := companyAccessAudit(scope, models.AuditEventEmployeeCompanyAccessDeleted, "Employee company access removed", mapping, companyAccessAuditData(mapping))
	if err != nil {
		return err
	}
	return s.repo.Delete(&mapping, &audit)
}

func companyAccessAudit(scope repository.AccessScope, event models.AuditEventType, summary string, mapping models.EmployeeCompanyAccess, before map[string]any) (models.AuditLog, error) {
	return masterAudit(scope, event, summary, "employee_company_access", mapping.CompanyID, &mapping.ID, before, companyAccessAuditData(mapping))
}
func companyAccessAuditData(value models.EmployeeCompanyAccess) map[string]any {
	return map[string]any{"employee_id": value.EmployeeID, "company_id": value.CompanyID, "role_id": value.RoleID, "is_active": value.IsActive}
}

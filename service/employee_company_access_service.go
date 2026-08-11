package service

import (
	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// Employee-company-access rule failures. All three answer 403, as they did
// before the split, because each means the caller aimed at something outside
// what they administer rather than sending a malformed request.
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

// EmployeeCompanyAccessService owns the rules for granting an employee access to
// a company: the caller must administer that company, the employee must exist,
// and the role must be one the caller can reach and must belong to that company.
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
	if _, err := s.repo.FindEmployeeBrief(input.EmployeeID); err != nil {
		return ErrAccessEmployeeMissing
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
	if err := s.repo.Create(&mapping); err != nil {
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

	fields := map[string]any{
		"employee_id": input.EmployeeID,
		"company_id":  input.CompanyID,
		"role_id":     input.RoleID,
		"is_active":   input.IsActive,
	}
	if err := s.repo.UpdateFields(&existing, fields); err != nil {
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
	return s.repo.Delete(&mapping)
}

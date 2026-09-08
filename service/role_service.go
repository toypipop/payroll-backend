package service

import (
	"strings"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// ErrRoleNameRequired means the submitted name was empty or only whitespace.
var ErrRoleNameRequired = apperr.Validation("roles_name is required")

// ErrRoleAccessDenied is re-exported so handlers import a single package.
var ErrRoleAccessDenied = repository.ErrRoleAccessDenied

// RoleService owns the role rules. A role is company-owned, which is what lets the employees domain reject a role from another company.
type RoleService struct {
	repo repository.RoleRepository
}

// NewRoleService wires the service to a repository implementation.
func NewRoleService(repo repository.RoleRepository) *RoleService {
	return &RoleService{repo: repo}
}

// List returns the roles in the caller's authorized companies.
func (s *RoleService) List(scope repository.AccessScope) ([]models.Roles, error) {
	return s.repo.ListScoped(scope)
}

// Options returns the same rows without relations, for cross-page dropdowns.
func (s *RoleService) Options(scope repository.AccessScope) ([]models.Roles, error) {
	return s.repo.ListMinimalScoped(scope)
}

// Create adds a role to an explicitly named company; unlike other master data it does not fall back to the caller's home company.
func (s *RoleService) Create(scope repository.AccessScope, input models.Roles) (models.Roles, error) {
	name := strings.TrimSpace(input.RolesName)
	if name == "" {
		return models.Roles{}, ErrRoleNameRequired
	}
	if !scope.CanAccessCompany(input.CompaniesID) {
		return models.Roles{}, ErrCompanyAccessDenied
	}

	role := models.Roles{CompaniesID: input.CompaniesID, RolesName: name}
	if err := s.repo.WithinTx(func(repo repository.RoleRepository) error {
		if err := repo.Create(&role); err != nil {
			return err
		}
		companyID, roleID := role.CompaniesID, role.ID
		audit, err := BuildAuditLog(scope, models.AuditLogInput{CompaniesID: &companyID, EntityID: &roleID, EventType: models.AuditEventRoleCreated, EntityType: "role", Summary: "Role created", AfterData: map[string]any{"roles_name": role.RolesName}})
		if err != nil {
			return err
		}
		return repo.AppendAuditLog(&audit)
	}); err != nil {
		return models.Roles{}, err
	}
	return role, nil
}

// Update renames a role. The company is immutable after creation, so it is never taken from input.
func (s *RoleService) Update(scope repository.AccessScope, id uint, name string) (models.Roles, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return models.Roles{}, ErrRoleNameRequired
	}

	role, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.Roles{}, err
	}

	before := role.RolesName
	if err := s.repo.WithinTx(func(repo repository.RoleRepository) error {
		if err := repo.UpdateName(&role, trimmed); err != nil {
			return err
		}
		companyID, roleID := role.CompaniesID, role.ID
		audit, err := BuildAuditLog(scope, models.AuditLogInput{CompaniesID: &companyID, EntityID: &roleID, EventType: models.AuditEventRoleUpdated, EntityType: "role", Summary: "Role updated", BeforeData: map[string]any{"roles_name": before}, AfterData: map[string]any{"roles_name": trimmed}})
		if err != nil {
			return err
		}
		return repo.AppendAuditLog(&audit)
	}); err != nil {
		return models.Roles{}, err
	}
	return role, nil
}

// Delete removes a role the caller is scoped to see; reference checks run in middleware before this.
func (s *RoleService) Delete(scope repository.AccessScope, id uint) error {
	role, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return err
	}
	return s.repo.WithinTx(func(repo repository.RoleRepository) error {
		if err := repo.Delete(&role); err != nil {
			return err
		}
		companyID, roleID := role.CompaniesID, role.ID
		audit, err := BuildAuditLog(scope, models.AuditLogInput{CompaniesID: &companyID, EntityID: &roleID, EventType: models.AuditEventRoleDeleted, EntityType: "role", Summary: "Role deleted", BeforeData: map[string]any{"roles_name": role.RolesName}})
		if err != nil {
			return err
		}
		return repo.AppendAuditLog(&audit)
	})
}

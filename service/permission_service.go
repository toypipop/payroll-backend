package service

import (
	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// Permission payload failures.
var (
	ErrPermissionModuleRequired = apperr.Validation("module_id is required")
	ErrPermissionScopeInvalid   = apperr.Validation("data_scope must be one of Own, Department, Companies, All")
)

// Re-exported so handlers import a single package.
var (
	ErrUnknownModule      = repository.ErrUnknownModule
	ErrPermissionRoleDeny = repository.ErrRoleAccessDenied
)

// PermissionEntry is one row of the submitted matrix.
type PermissionEntry struct {
	ModuleID  uint
	CanView   bool
	CanCreate bool
	CanEdit   bool
	CanDelete bool
	DataScope models.DataScope
}

// PermissionOverview is what the matrix screen needs to render itself.
type PermissionOverview struct {
	Modules []models.AppModule
	Access  []models.RolesModuleAccess
}

// PermissionService owns the rule that the database may only hold permission claims the permission check will actually honour.
type PermissionService struct {
	repo  repository.PermissionRepository
	roles repository.RoleRepository
}

// NewPermissionService wires the service to repository implementations.
func NewPermissionService(repo repository.PermissionRepository, roles repository.RoleRepository) *PermissionService {
	return &PermissionService{repo: repo, roles: roles}
}

// Overview returns the module tree plus the role's saved permission rows.
func (s *PermissionService) Overview(scope repository.AccessScope, rolesID uint) (PermissionOverview, error) {
	modules, err := s.repo.ListModulesOrdered()
	if err != nil {
		return PermissionOverview{}, err
	}
	access, err := s.repo.ListRoleAccessScoped(scope, rolesID)
	if err != nil {
		return PermissionOverview{}, err
	}
	return PermissionOverview{Modules: modules, Access: access}, nil
}

// validScope reports whether a submitted scope is one the model defines at all.
func validScope(scope models.DataScope) bool {
	switch scope {
	case models.ScopeOwn, models.ScopeDepartment, models.ScopeCompanies, models.ScopeAll:
		return true
	default:
		return false
	}
}

// Replace rewrites a role's permissions, coercing away anything the backend cannot enforce.
func (s *PermissionService) Replace(scope repository.AccessScope, rolesID uint, entries []PermissionEntry) error {
	role, err := s.roles.FindScoped(scope, rolesID)
	if err != nil {
		return ErrPermissionRoleDeny
	}

	moduleIDs := make([]uint, 0, len(entries))
	for _, entry := range entries {
		if entry.ModuleID == 0 {
			return ErrPermissionModuleRequired
		}
		if !validScope(entry.DataScope) {
			return ErrPermissionScopeInvalid
		}
		moduleIDs = append(moduleIDs, entry.ModuleID)
	}

	if len(entries) == 0 {
		return s.repo.ReplaceRoleAccess(scope, rolesID, nil)
	}

	resolved, err := s.repo.CapabilitiesFor(moduleIDs)
	if err != nil {
		return err
	}

	permissions := make([]models.RolesModuleAccess, 0, len(entries))
	for _, entry := range entries {
		// The matrix posts the whole tree back, headings included. A category is not a page, so it
		// carries no permission row — skipping it here is what keeps the whole save from being
		// rejected as an unknown module.
		if resolved.Categories[entry.ModuleID] {
			continue
		}
		capability := resolved.ByModule[entry.ModuleID]

		dataScope := entry.DataScope
		if !capability.AllowsScope(dataScope) {
			dataScope = models.ScopeCompanies
		}

		permissions = append(permissions, models.RolesModuleAccess{
			RolesID:     rolesID,
			CompaniesID: role.CompaniesID,
			ModuleID:    entry.ModuleID,
			CanView:     entry.CanView && capability.AllowsAction(models.ActionView),
			CanCreate:   entry.CanCreate && capability.AllowsAction(models.ActionCreate),
			CanEdit:     entry.CanEdit && capability.AllowsAction(models.ActionEdit),
			CanDelete:   entry.CanDelete && capability.AllowsAction(models.ActionDelete),
			DataScope:   dataScope,
		})
	}

	return s.repo.ReplaceRoleAccess(scope, rolesID, permissions)
}

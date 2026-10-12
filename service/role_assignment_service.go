package service

import (
	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

var ErrRoleAssignmentDenied = apperr.Forbidden("Editing System Permissions in this company is required to assign roles")

// RoleAssignmentService separates personnel edits from authority to grant access.
type RoleAssignmentService struct{ access repository.AccessRepository }

func NewRoleAssignmentService(access repository.AccessRepository) *RoleAssignmentService {
	return &RoleAssignmentService{access: access}
}

func (s *RoleAssignmentService) Require(scope repository.AccessScope, companyID uint) error {
	if s == nil || s.access == nil || !scope.CanAccessCompany(companyID) {
		return ErrRoleAssignmentDenied
	}
	access, err := s.access.ResolveAccess(scope.User)
	if err != nil {
		return err
	}
	var roleIDs []uint
	for _, grant := range access.CompanyRoles {
		if grant.CompanyID == companyID {
			roleIDs = append(roleIDs, grant.RoleID)
		}
	}
	if len(roleIDs) == 0 {
		return ErrRoleAssignmentDenied
	}
	grants, err := s.access.ModuleAccesses(roleIDs, "PermissionSettings")
	if err != nil {
		return err
	}
	capability, ok := models.CapabilityFor("PermissionSettings")
	if !ok || !capability.AllowsAction(models.ActionEdit) {
		return ErrRoleAssignmentDenied
	}
	for _, grant := range grants {
		for _, roleID := range roleIDs {
			if grant.RolesID == roleID && grant.CompaniesID == companyID && grant.CanEdit && capability.AllowsScope(grant.DataScope) {
				return nil
			}
		}
	}
	return ErrRoleAssignmentDenied
}

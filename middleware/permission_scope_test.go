package middleware

import (
	"reflect"
	"testing"

	"payroll-backend/models"
	"payroll-backend/repository"
)

func TestPermissionScopesStayWithinTheirCompanyAndAction(t *testing.T) {
	roles := []repository.CompanyRole{{CompanyID: 1, RoleID: 11}, {CompanyID: 2, RoleID: 22}, {CompanyID: 3, RoleID: 33}, {CompanyID: 4, RoleID: 44}}
	accesses := []models.RolesModuleAccess{
		{RolesID: 11, CanView: true, CanEdit: true, DataScope: models.ScopeCompanies},
		{RolesID: 22, CanView: true, CanEdit: true, DataScope: models.ScopeOwn},
		{RolesID: 33, CanView: true, DataScope: models.ScopeDepartment},
		{RolesID: 44, CanView: true, CanEdit: true, DataScope: "invalid"},
	}
	for _, tc := range []struct {
		action string
		want   map[uint]models.DataScope
	}{
		{"VIEW", map[uint]models.DataScope{1: models.ScopeCompanies, 2: models.ScopeOwn, 3: models.ScopeDepartment}},
		{"EDIT", map[uint]models.DataScope{1: models.ScopeCompanies, 2: models.ScopeOwn}},
		{"DELETE", map[uint]models.DataScope{}},
	} {
		t.Run(tc.action, func(t *testing.T) {
			ids, _, scopes, found := resolveAuthorizedCompanies(roles, accesses, tc.action)
			if !reflect.DeepEqual(scopes, tc.want) || len(ids) != len(tc.want) || found != (len(tc.want) > 0) {
				t.Fatalf("ids=%v scopes=%v found=%v; want %v", ids, scopes, found, tc.want)
			}
		})
	}
}

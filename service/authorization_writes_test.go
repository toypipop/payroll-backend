package service

import (
	"errors"
	"payroll-backend/models"
	"payroll-backend/repository"
	"reflect"
	"testing"
)

type roleGrantRepo struct {
	repository.AccessRepository
	grants []models.RolesModuleAccess
	err    error
}

func (r roleGrantRepo) ResolveAccess(models.Employees) (repository.UserAccess, error) {
	return repository.UserAccess{CompanyRoles: []repository.CompanyRole{{CompanyID: 1, RoleID: 11}, {CompanyID: 2, RoleID: 22}}}, r.err
}
func (r roleGrantRepo) ModuleAccesses(ids []uint, key string) ([]models.RolesModuleAccess, error) {
	if key != "PermissionSettings" {
		panic("wrong gate")
	}
	return r.grants, r.err
}
func rolePolicy(company uint) *RoleAssignmentService {
	return NewRoleAssignmentService(roleGrantRepo{grants: []models.RolesModuleAccess{{CompaniesID: company, RolesID: company * 11, CanEdit: true, DataScope: models.ScopeCompanies}}})
}
func TestRoleAssignmentRequiresCurrentCompanyPermission(t *testing.T) {
	scope := repository.NewAccessScope(models.Employees{ID: 9}, models.ScopeOwn, []uint{1, 2})
	for _, tc := range []struct {
		name    string
		grants  []models.RolesModuleAccess
		err     error
		allowed bool
	}{
		{name: "own employee editor"},
		{name: "other company grant", grants: []models.RolesModuleAccess{{CompaniesID: 2, RolesID: 22, CanEdit: true, DataScope: models.ScopeCompanies}}},
		{name: "view only", grants: []models.RolesModuleAccess{{CompaniesID: 1, RolesID: 11, CanView: true, DataScope: models.ScopeCompanies}}},
		{name: "invalid scope", grants: []models.RolesModuleAccess{{CompaniesID: 1, RolesID: 11, CanEdit: true, DataScope: models.ScopeOwn}}},
		{name: "lookup fails closed", err: errors.New("unavailable")},
		{name: "permission editor", grants: []models.RolesModuleAccess{{CompaniesID: 1, RolesID: 11, CanEdit: true, DataScope: models.ScopeCompanies}}, allowed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := NewRoleAssignmentService(roleGrantRepo{grants: tc.grants, err: tc.err}).Require(scope, 1)
			if (err == nil) != tc.allowed {
				t.Fatalf("err=%v allowed=%v", err, tc.allowed)
			}
		})
	}
	if err := rolePolicy(1).Require(scope, 3); !errors.Is(err, ErrRoleAssignmentDenied) {
		t.Fatalf("unauthorized company: %v", err)
	}
}

type employeeSecurityRepo struct {
	repository.EmployeesRepository
	row            models.Employees
	patch          map[string]any
	writes, audits int
	referenceErr   error
}

func (r *employeeSecurityRepo) FindScoped(repository.AccessScope, uint) (models.Employees, error) {
	return r.row, nil
}
func (r *employeeSecurityRepo) FindRole(id uint) (models.Roles, error) {
	return models.Roles{ID: id, CompaniesID: 1}, nil
}
func (r *employeeSecurityRepo) WithinTx(fn func(repository.EmployeesRepository) error) error {
	return fn(r)
}
func (r *employeeSecurityRepo) UpdateFields(_ *models.Employees, p map[string]any) error {
	r.patch = p
	r.writes++
	return nil
}
func (r *employeeSecurityRepo) FindDetail(uint) (models.Employees, error) { return r.row, nil }
func (r *employeeSecurityRepo) AppendAuditLog(*models.AuditLog) error     { r.audits++; return nil }
func (r *employeeSecurityRepo) ValidateCompanyReferences(models.Employees) error {
	return r.referenceErr
}
func TestEmployeeRoleAndScopeChangesRejectBeforeWrite(t *testing.T) {
	for _, tc := range []struct {
		name   string
		patch  map[string]any
		policy *RoleAssignmentService
		want   error
	}{
		{"self role promotion", map[string]any{"roles_id": float64(99)}, nil, ErrRoleAssignmentDenied},
		{"invalid role shape", map[string]any{"roles_id": "99"}, nil, nil},
		{"department scope promotion", map[string]any{"dept_id": float64(99)}, nil, ErrRoleAssignmentDenied},
		{"same role ordinary edit", map[string]any{"roles_id": float64(11), "first_name": "New"}, nil, nil},
		{"authorized assignment", map[string]any{"roles_id": float64(99)}, rolePolicy(1), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &employeeSecurityRepo{row: models.Employees{ID: 9, CompaniesID: 1, RolesID: 11, DeptID: 2}}
			scope := repository.NewAccessScope(r.row, models.ScopeOwn, []uint{1})
			_, err := NewEmployeesService(r, tc.policy).Update(scope, 9, tc.patch)
			denied := tc.want != nil || tc.name == "invalid role shape"
			if denied {
				if err == nil || r.writes != 0 || r.audits != 0 {
					t.Fatalf("denied mutation err=%v writes=%d audit=%d", err, r.writes, r.audits)
				}
				if tc.want != nil && !errors.Is(err, tc.want) {
					t.Fatal(err)
				}
			} else if err != nil || r.writes != 1 || r.audits != 1 {
				t.Fatalf("allowed mutation: %v writes=%d", err, r.writes)
			}
		})
	}
}

type scopedAccessRepo struct {
	repository.EmployeeCompanyAccessRepository
	scope                 repository.AccessScope
	employeeID, companyID uint
	denied                bool
	written               bool
}

func (r *scopedAccessRepo) FindEmployeeBriefScoped(s repository.AccessScope, id, company uint) (models.Employees, error) {
	r.scope = s
	r.employeeID = id
	r.companyID = company
	if r.denied {
		return models.Employees{}, repository.ErrEmployeeNotFound
	}
	return models.Employees{ID: id, CompaniesID: 2, IsActive: true}, nil
}
func (r *scopedAccessRepo) FindScoped(repository.AccessScope, uint) (models.EmployeeCompanyAccess, error) {
	return models.EmployeeCompanyAccess{ID: 3, EmployeeID: 9, CompanyID: 1}, nil
}
func (r *scopedAccessRepo) Create(m *models.EmployeeCompanyAccess, _ *models.AuditLog) error {
	r.written = true
	m.ID = 3
	return nil
}
func (r *scopedAccessRepo) UpdateFields(_ *models.EmployeeCompanyAccess, _ map[string]any, _ *models.AuditLog) error {
	r.written = true
	return nil
}
func (r *scopedAccessRepo) FindDetail(uint) (models.EmployeeCompanyAccess, error) {
	return models.EmployeeCompanyAccess{ID: 3, CompanyID: 1}, nil
}

type accessRoleRepo struct{ repository.RoleRepository }

func (accessRoleRepo) FindScoped(_ repository.AccessScope, id uint) (models.Roles, error) {
	return models.Roles{ID: id, CompaniesID: 1}, nil
}
func TestCompanyAccessValidatesDestinationScopeAndPreservesCrossCompanyAccess(t *testing.T) {
	for _, update := range []bool{false, true} {
		for _, denied := range []bool{false, true} {
			r := &scopedAccessRepo{denied: denied}
			scope := repository.NewAccessScope(models.Employees{ID: 9}, models.ScopeOwn, []uint{1, 2})
			scope.CompanyDataScopes = map[uint]models.DataScope{1: models.ScopeOwn, 2: models.ScopeCompanies}
			svc := NewEmployeeCompanyAccessService(r, accessRoleRepo{}, rolePolicy(1))
			input := EmployeeCompanyAccessInput{EmployeeID: 9, CompanyID: 1, RoleID: 11, IsActive: true}
			var err error
			if update {
				_, err = svc.Update(scope, 3, input)
			} else {
				_, err = svc.Create(scope, input)
			}
			if !reflect.DeepEqual(r.scope, scope) || r.companyID != 1 || r.employeeID != 9 {
				t.Fatal("target scope not preserved")
			}
			if denied {
				if !errors.Is(err, ErrAccessEmployeeMissing) || r.written {
					t.Fatalf("denied target wrote: %v", err)
				}
			} else if err != nil || !r.written {
				t.Fatalf("authorized cross-company grant failed: %v", err)
			}
		}
	}
}

func TestEmployeeReferencePatchRejectsCrossCompanyBeforeWrite(t *testing.T) {
	for _, field := range []string{"dept_id", "pos_id", "employment_type_id"} {
		t.Run(field, func(t *testing.T) {
			r := &employeeSecurityRepo{row: models.Employees{ID: 9, CompaniesID: 1, RolesID: 11}, referenceErr: repository.ErrEmployeeReferenceCompany}
			scope := repository.NewAccessScope(r.row, models.ScopeCompanies, []uint{1, 2})
			_, err := NewEmployeesService(r, rolePolicy(1)).Update(scope, 9, map[string]any{field: float64(8)})
			if !errors.Is(err, repository.ErrEmployeeReferenceCompany) || r.writes != 0 || r.audits != 0 {
				t.Fatalf("cross-company reference wrote: %v", err)
			}
		})
	}
	r := &employeeSecurityRepo{row: models.Employees{ID: 9, CompaniesID: 1, RolesID: 11}}
	scope := repository.NewAccessScope(r.row, models.ScopeCompanies, []uint{1})
	_, err := NewEmployeesService(r, nil).Update(scope, 9, map[string]any{"employment_type_id": nil})
	if err != nil || r.patch["employment_type_id"] != repository.ClearField {
		t.Fatalf("optional employment type cannot clear: %v", err)
	}
}
func TestCompanyAccessAssignmentWithoutPermissionCannotWrite(t *testing.T) {
	for _, update := range []bool{false, true} {
		r := &scopedAccessRepo{}
		scope := repository.NewAccessScope(models.Employees{ID: 9}, models.ScopeCompanies, []uint{1, 2})
		svc := NewEmployeeCompanyAccessService(r, accessRoleRepo{}, nil)
		input := EmployeeCompanyAccessInput{EmployeeID: 9, CompanyID: 1, RoleID: 11, IsActive: true}
		var err error
		if update {
			_, err = svc.Update(scope, 3, input)
		} else {
			_, err = svc.Create(scope, input)
		}
		if !errors.Is(err, ErrRoleAssignmentDenied) || r.written {
			t.Fatalf("access assignment wrote without permission: %v", err)
		}
	}
}

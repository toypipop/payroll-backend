package service

import (
	"errors"
	"reflect"
	"testing"

	"payroll-backend/models"
	"payroll-backend/repository"
)

type salaryDeleteEmployees struct {
	repository.EmployeesRepository
	t         *testing.T
	wantScope repository.AccessScope
	wantID    uint
	err       error
	called    bool
}

func (r *salaryDeleteEmployees) FindBriefScoped(scope repository.AccessScope, id uint) (models.Employees, error) {
	r.called = true
	if id != r.wantID || !reflect.DeepEqual(scope, r.wantScope) {
		r.t.Fatalf("lookup must authorize record owner with unchanged route scope: id=%d scope=%+v", id, scope)
	}
	// Deleting an authorized historical record must not require an active employee.
	return models.Employees{ID: id, CompaniesID: 1, IsActive: false}, r.err
}

func TestSalaryDeleteAuthorizesRecordOwnerBeforeMutation(t *testing.T) {
	for _, tc := range []struct {
		name               string
		dataScope          models.DataScope
		owner, company     uint
		lookupErr, wantErr error
	}{
		{"own denies other employee", models.ScopeOwn, 77, 1, repository.ErrEmployeeNotFound, ErrSalaryEmployeeDenied},
		{"department denies outside employee", models.ScopeDepartment, 77, 1, repository.ErrEmployeeNotFound, ErrSalaryEmployeeDenied},
		{"own allows self", models.ScopeOwn, 9, 1, nil, nil},
		{"department allows scoped employee", models.ScopeDepartment, 77, 1, nil, nil},
		{"company allows scoped employee", models.ScopeCompanies, 77, 1, nil, nil},
		{"unauthorized company", models.ScopeCompanies, 77, 2, nil, ErrSalaryCompanyDenied},
		{"lookup failure denies write", models.ScopeCompanies, 77, 1, errors.New("lookup unavailable"), ErrSalaryEmployeeDenied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scope := repository.NewAccessScope(models.Employees{ID: 9, CompaniesID: 1, DeptID: 5}, tc.dataScope, []uint{1})
			scope.CompanyDataScopes = map[uint]models.DataScope{1: tc.dataScope}
			repo := &fakeSalaryRepo{findRecord: models.SalariesStructure{ID: 123, EmployeesID: tc.owner, CompaniesID: tc.company}}
			employees := &salaryDeleteEmployees{t: t, wantScope: scope, wantID: tc.owner, err: tc.lookupErr}
			err := NewSalariesStructureService(repo, employees).DeleteRecord(scope, 123, "duplicate entry")
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error=%v, want %v", err, tc.wantErr)
			}
			if employees.called != (tc.company == 1) {
				t.Fatalf("unexpected employee authorization lookup: %v", employees.called)
			}
			if tc.wantErr != nil {
				if !reflect.DeepEqual(repo.calls, []string{"FindRecord"}) || len(repo.audits) != 0 {
					t.Fatalf("denied request reached transaction/write: %v", repo.calls)
				}
				return
			}
			if !reflect.DeepEqual(repo.calls, []string{"FindRecord", "LockCompanyCycles", "ClosedCyclesUsing", "DeleteRecord", "AppendAuditLog"}) || len(repo.audits) != 1 {
				t.Fatalf("authorized deletion lost transaction or audit protections: %v", repo.calls)
			}
		})
	}
}

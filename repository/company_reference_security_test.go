package repository

import (
	"database/sql/driver"
	"errors"
	"payroll-backend/models"
	"testing"
)

func TestEmployeeReferencesRequireMatchingCompanyAndLiveRow(t *testing.T) {
	for _, kind := range []string{"department", "position", "employment_type"} {
		for _, count := range []int64{0, 1} {
			t.Run(kind+string(rune('0'+count)), func(t *testing.T) {
				base, _ := lifecycleRepo(t, companySQLStep{contains: []string{"FROM \"" + kind + "\"", "id =", "companies_id =", "deleted_at"}, columns: []string{"count"}, values: []driver.Value{count}, args: []driver.Value{int64(8), int64(1), int64(0)}})
				employee := models.Employees{CompaniesID: 1}
				switch kind {
				case "department":
					employee.DeptID = 8
				case "position":
					employee.PosID = 8
				case "employment_type":
					id := uint(8)
					employee.EmploymentTypeID = &id
				}
				err := NewEmployeesRepository(base.(*gormCompanyRepository).db).ValidateCompanyReferences(employee)
				if count == 0 && !errors.Is(err, ErrEmployeeReferenceCompany) {
					t.Fatalf("foreign/deleted reference accepted: %v", err)
				}
				if count == 1 && err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
func TestCompanyAccessTargetScopeDoesNotBorrowBroaderHomeGrant(t *testing.T) {
	base, _ := lifecycleRepo(t, companySQLStep{contains: []string{"FROM \"employees\"", "(1 = 0 OR 2 IN", "employees.id =", "deleted_at"}, columns: []string{"id"}})
	scope := AccessScope{User: models.Employees{ID: 9}, AuthorizedCompanyIDs: []uint{1, 2}, CompanyDataScopes: map[uint]models.DataScope{1: models.ScopeCompanies, 2: models.ScopeOwn}}
	_, err := NewEmployeeCompanyAccessRepository(base.(*gormCompanyRepository).db).FindEmployeeBriefScoped(scope, 77, 2)
	if !errors.Is(err, ErrEmployeeNotFound) {
		t.Fatalf("out of scope target: %v", err)
	}
}
func TestCompanyBootstrapOmitsInjectedAssociationsAndRemainsAtomic(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "commit", true: "rollback"}[fail], func(t *testing.T) {
			auditFailure := errors.New("audit failed")
			audit := companySQLStep{contains: []string{"INSERT INTO \"audit_log\""}, columns: []string{"id"}, values: []driver.Value{int64(7)}}
			if fail {
				audit.err = auditFailure
			}
			base, boundary := lifecycleRepo(t,
				companySQLStep{contains: []string{"INSERT INTO \"companies\""}, columns: []string{"id"}, values: []driver.Value{int64(7)}},
				companySQLStep{contains: []string{"INSERT INTO \"roles\""}, columns: []string{"id"}, values: []driver.Value{int64(11)}},
				companySQLStep{contains: []string{"FROM \"app_module\""}, columns: []string{"id", "module_key", "is_active"}, values: []driver.Value{int64(3), "EmployeesListPage", true}},
				companySQLStep{contains: []string{"INSERT INTO \"roles_module_access\""}, columns: []string{"id"}, values: []driver.Value{int64(12)}},
				companySQLStep{contains: []string{"INSERT INTO \"employee_company_access\""}, columns: []string{"id"}, values: []driver.Value{int64(13)}}, audit)
			company := models.Companies{Name: "New", IsActive: true, PayrollConfig: &models.PayrollConfig{ID: 99}, AttendanceConfig: &models.AttendanceConfig{ID: 88}, EmployeeAccess: []models.EmployeeCompanyAccess{{ID: 77, EmployeeID: 100}}}
			err := base.CreateTenant(&company, TenantBootstrap{RoleName: "ADMIN", CreatorID: 9, Grant: models.RolesModuleAccess{CanView: true, CanEdit: true, DataScope: models.ScopeCompanies}}, &models.AuditLog{})
			if fail {
				if !errors.Is(err, auditFailure) || !boundary.rolledBack || boundary.committed {
					t.Fatalf("bootstrap not rolled back: %v", err)
				}
			} else if err != nil || !boundary.committed || boundary.rolledBack {
				t.Fatalf("bootstrap not committed: %v", err)
			}
		})
	}
}

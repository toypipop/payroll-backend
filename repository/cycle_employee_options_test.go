package repository

import (
	"testing"
	"time"

	"payroll-backend/models"
	"payroll-backend/timeutil"
)

func TestCycleEmployeeOptionsSQLScopesAndSalaryEligibility(t *testing.T) {
	for _, tc := range []struct {
		name  string
		scope AccessScope
		extra []string
	}{
		{"company", AccessScope{AuthorizedCompanyIDs: []uint{7}, DataScope: models.ScopeCompanies}, []string{"employees.companies_id IN"}},
		{"own", AccessScope{User: models.Employees{ID: 9}, AuthorizedCompanyIDs: []uint{7}, DataScope: models.ScopeOwn}, []string{"employees.id =", "employees.companies_id IN"}},
		{"no companies", AccessScope{}, []string{"1 = 0"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			contains := append([]string{
				"SELECT employees.id, employees.companies_id, employees.employees_code, employees.first_name, employees.last_name",
				"employees.companies_id =", "employees.is_active =",
				`employees.id IN (SELECT "employees_id"`,
				"is_active =", "effective_date <=",
				`"salaries_structure"."deleted_at" =`,
				`"employees"."deleted_at" =`,
			}, tc.extra...)
			base, _ := lifecycleRepo(t, companySQLStep{
				contains: contains,
				excludes: []string{"password", "base_salaries", "payroll_transaction"},
				columns:  []string{"id", "companies_id", "employees_code", "first_name", "last_name"},
			})
			repo := NewPayrollRunRepository(base.(*gormCompanyRepository).db)
			options, err := repo.ListCycleEmployeeOptions(tc.scope, models.PayrollCycle{
				ID: 4, CompaniesID: 7,
				EndDate: time.Date(2026, 9, 30, 23, 59, 59, 0, timeutil.BangkokLocation),
			})
			if err != nil || options == nil || len(options) != 0 {
				t.Fatalf("options=%v err=%v", options, err)
			}
		})
	}
}

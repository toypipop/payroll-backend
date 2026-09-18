package repository

import (
	"database/sql/driver"
	"testing"

	"payroll-backend/models"
)

func TestEmployeeOptionsFilterInactiveWithoutHidingHistoricalEmployeeLookup(t *testing.T) {
	base, _ := lifecycleRepo(t,
		companySQLStep{
			contains: []string{"employees.is_active =", "employees.companies_id IN", `"employees"."deleted_at" =`},
			excludes: []string{"password", "email", "base_salaries"},
			columns:  []string{"id", "companies_id", "employees_code", "first_name", "last_name", "is_active"},
		},
		companySQLStep{
			contains: []string{"first_name, last_name, is_active", `FROM "employees"`},
			excludes: []string{"is_active ="},
			columns:  []string{"id", "companies_id", "is_active"},
			values:   []driver.Value{int64(9), int64(7), false},
		},
	)
	repo := NewEmployeesRepository(base.(*gormCompanyRepository).db)
	scope := AccessScope{AuthorizedCompanyIDs: []uint{7}, DataScope: models.ScopeCompanies}
	options, err := repo.ListActiveOptions(scope)
	if err != nil || options == nil || len(options) != 0 {
		t.Fatalf("options=%v err=%v", options, err)
	}
	employee, err := repo.FindBriefScoped(scope, 9)
	if err != nil || employee.ID != 9 || employee.IsActive {
		t.Fatalf("history lookup=%+v err=%v", employee, err)
	}
}

func TestPayrollCleanupOnlyRemovesTransactionsOutsideCurrentCycleEmployees(t *testing.T) {
	for _, ids := range [][]uint{{42}, {}} {
		contains := []string{`FROM "payroll_transaction"`, "cycle_id =", `"payroll_transaction"."deleted_at" =`}
		var excludes []string
		if len(ids) > 0 {
			contains = append(contains, "employees_id NOT IN")
		} else {
			excludes = []string{"NOT IN"}
		}
		base, _ := lifecycleRepo(t,
			companySQLStep{contains: contains, excludes: excludes, columns: []string{"id"}, values: []driver.Value{int64(91)}},
			companySQLStep{contains: []string{`UPDATE "payroll_transaction_detail"`, "transaction_id IN", `"deleted_at"`}},
			companySQLStep{contains: []string{`UPDATE "payroll_transaction"`, "id IN", `"deleted_at"`}},
		)
		repo := NewPayrollRunRepository(base.(*gormCompanyRepository).db)
		if err := repo.WithinTx(func(tx PayrollRunRepository) error { return tx.DeleteTransactionsOutsideEmployees(7, ids) }); err != nil {
			t.Fatal(err)
		}
	}
}

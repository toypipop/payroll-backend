package repository

import (
	"database/sql/driver"
	"reflect"
	"strings"
	"testing"

	"payroll-backend/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestDepartmentGrantUsesOnlyItsAuthorizedCompanies(t *testing.T) {
	base, _ := lifecycleRepo(t,
		companySQLStep{contains: []string{"SELECT category_id", "FROM \"department\""}, columns: []string{"category_id"}, values: []driver.Value{int64(8)}},
		companySQLStep{
			contains: []string{"employees.id = $1 AND (1 = 0 OR employees.companies_id IN ($2) OR (employees.companies_id IN ($3) AND employees.id IN (SELECT employees.id", "department.category_id = $4 AND employees.companies_id IN ($5)"},
			columns:  []string{"id"},
			checkArgs: func(args []driver.NamedValue) {
				want := []int64{77, 1, 2, 8, 2}
				if len(args) < len(want) {
					t.Fatalf("missing scope bindings: %v", args)
				}
				for i, value := range want {
					if args[i].Value != value {
						t.Fatalf("binding %d=%v; want %v", i, args[i].Value, value)
					}
				}
			},
		},
	)
	scope := AccessScope{
		User: models.Employees{ID: 9, DeptID: 5}, DataScope: models.ScopeCompanies,
		AuthorizedCompanyIDs: []uint{1, 2},
		CompanyDataScopes:    map[uint]models.DataScope{1: models.ScopeCompanies, 2: models.ScopeDepartment},
	}
	var rows []models.Employees
	err := base.(*gormCompanyRepository).db.Where("employees.id = ?", 77).
		Scopes(scope.ApplyEmployee("employees.companies_id", "employees.id")).Find(&rows).Error
	if err != nil {
		t.Fatal(err)
	}
}

func TestEmployeeScopeGroupsCompanyGrantsUnderTargetID(t *testing.T) {
	db, err := gorm.Open(postgres.Open("host=localhost dbname=unused"), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	scope := AccessScope{
		User: models.Employees{ID: 9}, DataScope: models.ScopeCompanies,
		AuthorizedCompanyIDs: []uint{1, 2, 3, 4},
		CompanyDataScopes:    map[uint]models.DataScope{1: models.ScopeCompanies, 2: models.ScopeOwn, 4: "invalid", 99: models.ScopeCompanies},
	}
	var rows []models.Employees
	query := db.Where("employees.id = ?", 77).Scopes(scope.ApplyEmployee("employees.companies_id", "employees.id")).Find(&rows)
	sql := query.Statement.SQL.String()
	want := "employees.id = $1 AND (1 = 0 OR employees.companies_id IN ($2) OR (employees.companies_id IN ($3) AND employees.id = $4))"
	if query.Error != nil || !strings.Contains(sql, want) {
		t.Fatalf("unscoped or ungrouped target query: %s; err=%v", sql, query.Error)
	}
	if !reflect.DeepEqual(query.Statement.Vars[:4], []any{77, uint(1), uint(2), uint(9)}) {
		t.Fatalf("unexpected grant bindings: %v", query.Statement.Vars)
	}
}

func TestEmployeeScopeEmptyCompanyGrantsDenyAll(t *testing.T) {
	db, err := gorm.Open(postgres.Open("host=localhost dbname=unused"), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, grants := range []map[uint]models.DataScope{{}, {99: models.ScopeCompanies}, {1: "invalid"}} {
		scope := AccessScope{DataScope: models.ScopeCompanies, AuthorizedCompanyIDs: []uint{1}, CompanyDataScopes: grants}
		var rows []models.Employees
		query := db.Scopes(scope.ApplyEmployee("employees.companies_id", "employees.id")).Find(&rows)
		if query.Error != nil || !strings.Contains(query.Statement.SQL.String(), "1 = 0") || strings.Contains(query.Statement.SQL.String(), " OR ") {
			t.Fatalf("empty/invalid grants must deny all: %s; err=%v", query.Statement.SQL.String(), query.Error)
		}
	}
}

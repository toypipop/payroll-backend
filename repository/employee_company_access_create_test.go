package repository

import (
	"database/sql/driver"
	"testing"

	"payroll-backend/models"
)

func TestCompanyAccessCreatePreservesExplicitStatus(t *testing.T) {
	for _, active := range []bool{false, true} {
		t.Run(map[bool]string{false: "inactive", true: "active"}[active], func(t *testing.T) {
			base, boundary := lifecycleRepo(t,
				companySQLStep{contains: []string{`FROM "employees"`, "FOR SHARE"}, columns: []string{"id", "is_active"}, values: []driver.Value{int64(9), true}},
				companySQLStep{
					contains: []string{`INSERT INTO "employee_company_access"`, `"is_active"`}, columns: []string{"id"}, values: []driver.Value{int64(42)},
					checkArgs: func(args []driver.NamedValue) {
						found := false
						for _, arg := range args {
							if value, ok := arg.Value.(bool); ok {
								found = true
								if value != active {
									t.Errorf("inserted is_active=%v, want %v", value, active)
								}
							}
						}
						if !found {
							t.Error("insert omitted explicit status")
						}
					},
				},
				companySQLStep{contains: []string{`INSERT INTO "audit_log"`}, columns: []string{"id"}, values: []driver.Value{int64(50)}},
			)
			mapping := models.EmployeeCompanyAccess{EmployeeID: 9, CompanyID: 7, RoleID: 2, IsActive: active}
			entityID := uint(0)
			audit := models.AuditLog{EntityID: &entityID}
			if err := NewEmployeeCompanyAccessRepository(base.(*gormCompanyRepository).db).Create(&mapping, &audit); err != nil {
				t.Fatal(err)
			}
			if mapping.ID != 42 || mapping.IsActive != active || entityID != 42 || !boundary.committed {
				t.Fatalf("created mapping=%+v audit entity=%d committed=%v", mapping, entityID, boundary.committed)
			}
		})
	}
}

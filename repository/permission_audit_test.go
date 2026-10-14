package repository

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"payroll-backend/models"
)

func TestPermissionSaveSnapshotsBeforeStateAndRollsBackOnAuditFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "commit", true: "rollback"}[fail], func(t *testing.T) {
			auditStep := companySQLStep{contains: []string{`INSERT INTO "audit_log"`}, columns: []string{"id"}, values: []driver.Value{int64(9)}, checkArgs: func(args []driver.NamedValue) {
				found := false
				for _, arg := range args {
					var value string
					switch v := arg.Value.(type) {
					case string:
						value = v
					case []byte:
						value = string(v)
					}
					if strings.Contains(value, `"permissions"`) && strings.Contains(value, `"data_scope":"Own"`) {
						found = true
					}
				}
				if !found {
					t.Fatal("before grants were not captured inside transaction")
				}
			}}
			if fail {
				auditStep.err = errors.New("audit unavailable")
			}
			boundary := &companySQL{t: t, steps: []companySQLStep{
				{contains: []string{`FROM "roles"`, `FOR UPDATE`}, columns: []string{"id", "companies_id"}, values: []driver.Value{int64(1), int64(1)}},
				{contains: []string{`FROM "roles_module_access"`}, columns: []string{"id", "companies_id", "roles_id", "module_id", "can_view", "data_scope"}, values: []driver.Value{int64(7), int64(1), int64(1), int64(2), true, "Own"}},
				{contains: []string{`UPDATE "roles_module_access"`}},
				{contains: []string{`INSERT INTO "roles_module_access"`}, columns: []string{"id"}, values: []driver.Value{int64(8)}},
				auditStep,
			}}
			conn := sql.OpenDB(boundary)
			t.Cleanup(func() {
				_ = conn.Close()
				if len(boundary.steps) != 0 {
					t.Errorf("%d missing SQL steps", len(boundary.steps))
				}
			})
			db, err := gorm.Open(postgres.New(postgres.Config{Conn: conn}), &gorm.Config{NamingStrategy: schema.NamingStrategy{SingularTable: true}, Logger: logger.Discard})
			if err != nil {
				t.Fatal(err)
			}
			rows := []models.RolesModuleAccess{{CompaniesID: 1, RolesID: 1, ModuleID: 2, CanView: true, CanDelete: true, DataScope: models.ScopeCompanies}}
			audit, err := models.NewAuditLogInput(models.AuditLogInput{EventType: models.AuditEventPermissionUpdated, AfterData: models.PermissionAuditData(rows)})
			if err != nil {
				t.Fatal(err)
			}
			scope := NewAccessScope(models.Employees{ID: 5}, models.ScopeCompanies, []uint{1})
			err = NewPermissionRepository(db).ReplaceRoleAccess(scope, 1, rows, &audit)
			if fail {
				if err == nil || !boundary.rolledBack || boundary.committed {
					t.Fatal("audit failure did not roll back grants")
				}
			} else if err != nil || !boundary.committed {
				t.Fatalf("save did not commit: %v", err)
			}
		})
	}
}

package repository

import (
	"database/sql/driver"
	"testing"

	"payroll-backend/models"
)

// Even a model carrying relations must write only time_log, not employees,
// employee_company_access, or work_location. Unexpected SQL fails the boundary.
func TestTimeLogRepositoryNeverWritesAssociations(t *testing.T) {
	for _, operation := range []string{"create", "bulk", "save"} {
		t.Run(operation, func(t *testing.T) {
			write := companySQLStep{contains: []string{`INSERT INTO "time_log"`}, columns: []string{"id"}, values: []driver.Value{int64(3)}}
			if operation == "save" {
				write = companySQLStep{contains: []string{`UPDATE "time_log"`}}
			}
			base, boundary := lifecycleRepo(t,
				companySQLStep{contains: []string{`FROM "employees"`, `FOR SHARE`}, columns: []string{"id", "is_active"}, values: []driver.Value{int64(9), true}},
				write,
				companySQLStep{contains: []string{`INSERT INTO "audit_log"`}, columns: []string{"id"}, values: []driver.Value{int64(1)}},
			)
			repo := NewTimeLogRepository(base.(*gormCompanyRepository).db)
			location := uint(4)
			log := models.TimeLog{
				CompaniesID: 1, EmployeesID: 9, WorkLocationID: &location,
				Employees:    models.Employees{ID: 77, CompanyAccess: []models.EmployeeCompanyAccess{{ID: 200}}},
				WorkLocation: &models.WorkLocation{ID: 999},
			}
			entityID := uint(0)
			audit := &models.AuditLog{EntityID: &entityID}
			var err error
			switch operation {
			case "create":
				err = repo.Create(&log, audit)
			case "bulk":
				err = repo.CreateMany([]models.TimeLog{log}, audit)
			case "save":
				log.ID = 3
				err = repo.Save(&log, audit)
			}
			if err != nil {
				t.Fatal(err)
			}
			if log.EmployeesID != 9 || *log.WorkLocationID != 4 {
				t.Fatalf("association changed validated IDs: %+v", log)
			}
			if !boundary.committed || boundary.rolledBack {
				t.Fatal("write and audit did not commit together")
			}
			if operation == "create" && entityID != 3 {
				t.Fatalf("audit entity_id=%d, want 3", entityID)
			}
		})
	}
}

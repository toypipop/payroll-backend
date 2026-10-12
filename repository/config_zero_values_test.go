package repository

import (
	"database/sql/driver"
	"testing"

	"payroll-backend/models"
)

func TestPayrollConfigInitialInsertKeepsExplicitZero(t *testing.T) {
	base, boundary := lifecycleRepo(t, companySQLStep{contains: []string{"INSERT INTO \"payroll_config\"", "ON CONFLICT"}, columns: []string{"id"}, values: []driver.Value{int64(10)}, checkArgs: func(args []driver.NamedValue) {
		for _, arg := range args {
			if v, ok := arg.Value.(float64); ok && v != 0 {
				t.Fatalf("default substituted zero: %v", args)
			}
		}
	}}, companySQLStep{contains: []string{"INSERT INTO \"audit_log\""}, columns: []string{"id"}, values: []driver.Value{int64(11)}})
	cfg := models.PayrollConfig{CompaniesID: 1, TaxCalculation: "None"}
	err := NewPayrollConfigRepository(base.(*gormCompanyRepository).db).UpsertForCompany(&cfg, &models.AuditLog{})
	if err != nil || cfg.ID != 10 || cfg.SsoRate != 0 || cfg.SsoMaxAmount != 0 || cfg.SsoMinSalary != 0 || !boundary.committed {
		t.Fatalf("zero config failed: %+v err=%v", cfg, err)
	}
}
func TestAttendanceConfigInitialInsertKeepsDisabledDeductions(t *testing.T) {
	base, boundary := lifecycleRepo(t, companySQLStep{contains: []string{"FROM \"attendance_config\""}, columns: []string{"id"}}, companySQLStep{contains: []string{"INSERT INTO \"attendance_config\""}, columns: []string{"id"}, values: []driver.Value{int64(10)}, checkArgs: func(args []driver.NamedValue) {
		for _, arg := range args {
			if v, ok := arg.Value.(bool); ok && v {
				t.Fatalf("default substituted false: %v", args)
			}
		}
	}}, companySQLStep{contains: []string{"INSERT INTO \"audit_log\""}, columns: []string{"id"}, values: []driver.Value{int64(11)}})
	cfg := models.AttendanceConfig{CompaniesID: 1}
	err := NewAttendanceConfigRepository(base.(*gormCompanyRepository).db).UpsertForCompany(&cfg, &models.AuditLog{})
	if err != nil || cfg.ID != 10 || cfg.IsLateDeductionEnabled || cfg.IsAbsentDeductionEnabled || !boundary.committed {
		t.Fatalf("false config lost: %+v err=%v", cfg, err)
	}
}

package repository

import (
	"database/sql/driver"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	"payroll-backend/apperr"
	"payroll-backend/models"
)

func TestEmployeeCreatePreservesExplicitStatus(t *testing.T) {
	for _, active := range []bool{false, true} {
		t.Run(map[bool]string{false: "inactive", true: "active"}[active], func(t *testing.T) {
			base, _ := lifecycleRepo(t, companySQLStep{
				contains: []string{`INSERT INTO "employees"`, `"is_active"`}, columns: []string{"id"}, values: []driver.Value{int64(9)},
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
			})
			employee := models.Employees{CompaniesID: 7, IsActive: active}
			if err := NewEmployeesRepository(base.(*gormCompanyRepository).db).Create(&employee); err != nil {
				t.Fatal(err)
			}
			if employee.ID != 9 || employee.IsActive != active {
				t.Fatalf("created employee id=%d active=%v", employee.ID, employee.IsActive)
			}
		})
	}
}

func TestEmployeeWritesRecheckStatusUnderTransactionLock(t *testing.T) {
	cases := []struct {
		name  string
		write func(*gorm.DB) error
	}{
		{"time log create", func(db *gorm.DB) error { return NewTimeLogRepository(db).Create(&models.TimeLog{EmployeesID: 9}, nil) }},
		{"time log update", func(db *gorm.DB) error {
			return NewTimeLogRepository(db).Save(&models.TimeLog{ID: 3, EmployeesID: 9}, nil)
		}},
		{"time log bulk", func(db *gorm.DB) error {
			return NewTimeLogRepository(db).CreateMany([]models.TimeLog{{EmployeesID: 9}}, nil)
		}},
		{"leave create", func(db *gorm.DB) error {
			return NewLeaveRequestRepository(db).Create(&models.LeaveRequest{EmployeesID: 9}, nil)
		}},
		{"leave update target", func(db *gorm.DB) error {
			return NewLeaveRequestRepository(db).UpdateFields(&models.LeaveRequest{ID: 3, EmployeesID: 8}, map[string]any{"employees_id": uint(9)}, nil)
		}},
		{"earning create", func(db *gorm.DB) error {
			return NewEmployeeEarningRepository(db).Create(&models.EmployeeEarning{EmployeesID: 9}, nil)
		}},
		{"earning update target", func(db *gorm.DB) error {
			return NewEmployeeEarningRepository(db).UpdateFields(&models.EmployeeEarning{ID: 3, EmployeesID: 8}, map[string]any{"employees_id": uint(9)}, nil)
		}},
		{"deduction create", func(db *gorm.DB) error {
			return NewEmployeeDeductionRepository(db).Create(&models.EmployeeDeduction{EmployeesID: 9}, nil)
		}},
		{"deduction update target", func(db *gorm.DB) error {
			return NewEmployeeDeductionRepository(db).UpdateFields(&models.EmployeeDeduction{ID: 3, EmployeesID: 8}, map[string]any{"employees_id": uint(9)}, nil)
		}},
		{"cycle earning create", func(db *gorm.DB) error {
			return NewCycleEarningRepository(db).Create(&models.PayrollCycleEarning{EmployeesID: 9}, nil)
		}},
		{"cycle earning update target", func(db *gorm.DB) error {
			return NewCycleEarningRepository(db).UpdateFields(&models.PayrollCycleEarning{ID: 3, EmployeesID: 8}, map[string]any{"employees_id": uint(9)}, nil)
		}},
		{"cycle deduction create", func(db *gorm.DB) error {
			return NewCycleDeductionRepository(db).Create(&models.PayrollCycleDeduction{EmployeesID: 9}, nil)
		}},
		{"cycle deduction update target", func(db *gorm.DB) error {
			return NewCycleDeductionRepository(db).UpdateFields(&models.PayrollCycleDeduction{ID: 3, EmployeesID: 8}, map[string]any{"employees_id": uint(9)}, nil)
		}},
		{"company access create", func(db *gorm.DB) error {
			return NewEmployeeCompanyAccessRepository(db).Create(&models.EmployeeCompanyAccess{EmployeeID: 9}, nil)
		}},
		{"company access update target", func(db *gorm.DB) error {
			return NewEmployeeCompanyAccessRepository(db).UpdateFields(&models.EmployeeCompanyAccess{ID: 3, EmployeeID: 8}, map[string]any{"employee_id": uint(9)}, nil)
		}},
		{"salary create", func(db *gorm.DB) error {
			return NewSalariesStructureRepository(db).CreateRecord(&models.SalariesStructure{EmployeesID: 9})
		}},
		{"salary update", func(db *gorm.DB) error {
			return NewSalariesStructureRepository(db).UpdateAmount(models.SalariesStructure{ID: 3, EmployeesID: 9}, 100, true)
		}},
		{"salary move", func(db *gorm.DB) error {
			return NewSalariesStructureRepository(db).MoveRecord(models.SalariesStructure{ID: 3, EmployeesID: 9}, time.Now(), 100)
		}},
		{"salary switch off", func(db *gorm.DB) error { return NewSalariesStructureRepository(db).SwitchOffAll(9) }},
	}
	for _, tc := range cases {
		for _, active := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{true: "/active", false: "/inactive"}[active], func(t *testing.T) {
				steps := []companySQLStep{{
					contains: []string{`FROM "employees"`, "FOR SHARE", `"employees"."deleted_at" =`},
					excludes: []string{"password"}, columns: []string{"id", "is_active"}, values: []driver.Value{int64(9), active},
					checkArgs: func(args []driver.NamedValue) {
						if len(args) == 0 || args[0].Value != int64(9) {
							t.Errorf("must lock target employee 9: %v", args)
						}
					},
				}}
				if active {
					operation := "UPDATE"
					if strings.Contains(tc.name, "create") || strings.Contains(tc.name, "bulk") {
						operation = "INSERT INTO"
					}
					steps = append(steps, companySQLStep{contains: []string{operation}, columns: []string{"id"}, values: []driver.Value{int64(3)}})
				}
				base, boundary := lifecycleRepo(t, steps...)
				err := base.(*gormCompanyRepository).db.Session(&gorm.Session{DisableNestedTransaction: true}).Transaction(tc.write)
				if active {
					if err != nil || !boundary.committed || boundary.rolledBack {
						t.Fatalf("active write must commit: %v", err)
					}
				} else if apperr.KindOf(err) != apperr.KindConflict || !boundary.rolledBack || boundary.committed {
					t.Fatalf("inactive write must roll back with conflict: %v", err)
				}
			})
		}
	}
}

func TestLeaveApprovalRechecksInactiveEmployeeEvenWithoutQuota(t *testing.T) {
	base, boundary := lifecycleRepo(t,
		companySQLStep{contains: []string{`FROM "leave_request"`}, excludes: []string{"FOR UPDATE"}, columns: []string{"id", "employees_id", "status"}, values: []driver.Value{int64(3), int64(9), string(models.LeaveStatusPending)}},
		companySQLStep{contains: []string{`FROM "employees"`, "FOR UPDATE", "is_active"}, columns: []string{"id", "is_active"}, values: []driver.Value{int64(9), false}},
	)
	err := NewLeaveRequestRepository(base.(*gormCompanyRepository).db).ApproveWithinQuota(3, 1, nil)
	if apperr.KindOf(err) != apperr.KindConflict || !boundary.rolledBack || boundary.committed {
		t.Fatalf("approval must reject inactive employee: %v", err)
	}
}

func TestLeaveApprovalLocksEmployeeBeforeRequestAndDetectsReassignment(t *testing.T) {
	base, boundary := lifecycleRepo(t,
		companySQLStep{contains: []string{`FROM "leave_request"`}, excludes: []string{"FOR UPDATE"}, columns: []string{"id", "employees_id", "status"}, values: []driver.Value{int64(3), int64(9), string(models.LeaveStatusPending)}},
		companySQLStep{contains: []string{`FROM "employees"`, "FOR UPDATE"}, columns: []string{"id", "is_active"}, values: []driver.Value{int64(9), true}},
		companySQLStep{contains: []string{`FROM "leave_request"`, "FOR UPDATE"}, columns: []string{"id", "employees_id", "status"}, values: []driver.Value{int64(3), int64(10), string(models.LeaveStatusPending)}},
	)
	err := NewLeaveRequestRepository(base.(*gormCompanyRepository).db).ApproveWithinQuota(3, 1, nil)
	if apperr.KindOf(err) != apperr.KindConflict || !boundary.rolledBack {
		t.Fatalf("concurrent reassignment must require reload: %v", err)
	}
}

func TestEmployeeDeleteLocksCyclesThenEmployeeBeforeSalaries(t *testing.T) {
	base, boundary := lifecycleRepo(t,
		companySQLStep{contains: []string{`FROM "payroll_cycle"`, "FOR SHARE", "ORDER BY id"}, columns: []string{"id"}},
		companySQLStep{contains: []string{`FROM "employees"`, "FOR UPDATE"}, columns: []string{"id"}, values: []driver.Value{int64(9)}},
		companySQLStep{contains: []string{`UPDATE "salaries_structure"`}},
		companySQLStep{contains: []string{`UPDATE "employees"`}},
	)
	err := NewEmployeesRepository(base.(*gormCompanyRepository).db).Delete(&models.Employees{ID: 9, CompaniesID: 7})
	if err != nil || !boundary.committed {
		t.Fatalf("retirement must commit: %v", err)
	}
}

func TestSalaryBatchRollsBackEarlierWritesWhenEmployeeBecameInactive(t *testing.T) {
	base, boundary := lifecycleRepo(t,
		companySQLStep{contains: []string{`FROM "employees"`, "FOR SHARE"}, columns: []string{"id", "is_active"}, values: []driver.Value{int64(8), true}},
		companySQLStep{contains: []string{`INSERT INTO "salaries_structure"`}, columns: []string{"id"}, values: []driver.Value{int64(30)}},
		companySQLStep{contains: []string{`INSERT INTO "audit_log"`}, columns: []string{"id"}, values: []driver.Value{int64(31)}},
		companySQLStep{contains: []string{`FROM "employees"`, "FOR SHARE"}, columns: []string{"id", "is_active"}, values: []driver.Value{int64(9), false}},
	)
	err := NewSalariesStructureRepository(base.(*gormCompanyRepository).db).WithinTx(func(tx SalariesStructureRepository) error {
		if err := tx.CreateRecord(&models.SalariesStructure{EmployeesID: 8}); err != nil {
			return err
		}
		if err := tx.AppendAuditLog(&models.AuditLog{}); err != nil {
			return err
		}
		return tx.CreateRecord(&models.SalariesStructure{EmployeesID: 9})
	})
	if apperr.KindOf(err) != apperr.KindConflict || !boundary.rolledBack || boundary.committed {
		t.Fatalf("salary batch must roll back with its audit: %v", err)
	}
}

func TestActiveLeaveApprovalCommitsAfterLockedRecheck(t *testing.T) {
	leave := companySQLStep{contains: []string{`FROM "leave_request"`}, columns: []string{"id", "employees_id", "status"}, values: []driver.Value{int64(3), int64(9), string(models.LeaveStatusPending)}}
	lockedLeave := leave
	lockedLeave.contains = []string{`FROM "leave_request"`, "FOR UPDATE"}
	base, boundary := lifecycleRepo(t, leave,
		companySQLStep{contains: []string{`FROM "employees"`, "FOR UPDATE"}, columns: []string{"id", "is_active"}, values: []driver.Value{int64(9), true}},
		lockedLeave,
		companySQLStep{contains: []string{`UPDATE "leave_request"`}},
		companySQLStep{contains: []string{`INSERT INTO "audit_log"`}, columns: []string{"id"}, values: []driver.Value{int64(31)}},
	)
	err := NewLeaveRequestRepository(base.(*gormCompanyRepository).db).ApproveWithinQuota(3, 1, &models.AuditLog{})
	if err != nil || !boundary.committed || boundary.rolledBack {
		t.Fatalf("active approval and audit must commit: %v", err)
	}
}

func TestInactiveLeaveCanStillBeRejected(t *testing.T) {
	base, boundary := lifecycleRepo(t, companySQLStep{contains: []string{`UPDATE "leave_request"`}})
	err := NewLeaveRequestRepository(base.(*gormCompanyRepository).db).UpdateFields(&models.LeaveRequest{ID: 3, EmployeesID: 9}, map[string]any{"status": string(models.LeaveStatusRejected)}, nil)
	if err != nil || !boundary.committed {
		t.Fatalf("cleanup rejection must remain available: %v", err)
	}
}

func TestEmployeeWriteRejectsDeletedOrZeroEmployee(t *testing.T) {
	for _, id := range []uint{0, 9} {
		t.Run(map[uint]string{0: "zero", 9: "deleted"}[id], func(t *testing.T) {
			var steps []companySQLStep
			if id != 0 {
				steps = append(steps, companySQLStep{contains: []string{`FROM "employees"`, "FOR SHARE", `"employees"."deleted_at" =`}, columns: []string{"id", "is_active"}})
			}
			base, boundary := lifecycleRepo(t, steps...)
			err := NewTimeLogRepository(base.(*gormCompanyRepository).db).Create(&models.TimeLog{EmployeesID: id}, nil)
			if apperr.KindOf(err) != apperr.KindNotFound || !boundary.rolledBack {
				t.Fatalf("missing employee write must roll back: %v", err)
			}
		})
	}
}

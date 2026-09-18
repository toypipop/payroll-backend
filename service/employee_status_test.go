package service

import (
	"errors"
	"testing"

	"payroll-backend/models"
	"payroll-backend/repository"
)

func TestInactiveEmployeeRejectedByOperationalServices(t *testing.T) {
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1, IsActive: false}}
	scope := salaryScope()
	tests := []struct {
		name string
		call func() error
	}{
		{"time log", func() error { _, err := NewTimeLogService(nil, employees).resolveEmployee(scope, 5, 1); return err }},
		{"leave", func() error {
			_, _, err := NewLeaveRequestService(nil, employees, nil).resolveTarget(scope, LeaveInput{EmployeesID: 5, CompaniesID: 1})
			return err
		}},
		{"earning", func() error {
			_, err := NewEmployeeEarningService(nil, employees, nil).resolveTarget(scope, 5, 1, 2)
			return err
		}},
		{"deduction", func() error {
			_, err := NewEmployeeDeductionService(nil, employees, nil).resolveTarget(scope, 5, 1, 2)
			return err
		}},
		{"salary", func() error {
			_, err := NewSalariesStructureService(nil, employees).Save(scope, SalaryInput{EmployeesID: 5, CompaniesID: 1, EffectiveDate: "2026-09-01", Reason: "Test", BaseSalaries: 100})
			return err
		}},
		{"bulk salary", func() error {
			_, err := NewSalariesStructureService(nil, employees).resolveBulkPeople(scope, []BulkRaiseTarget{{EmployeesID: 5}})
			return err
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !errors.Is(err, ErrEmployeeInactive) {
				t.Fatalf("error=%v want employee inactive", err)
			}
		})
	}
}

func TestPayrollPrunesExcludedEmployeesEvenWhenNobodyIsActive(t *testing.T) {
	for _, ids := range [][]uint{{42}, {}} {
		repo := &fakeRunRepo{payrollConfig: models.PayrollConfig{TaxCalculation: "None"}}
		for _, id := range ids {
			repo.employees = append(repo.employees, models.Employees{ID: id, CompaniesID: 1})
		}
		cycle := models.PayrollCycle{ID: 7, CompaniesID: 1, StartDate: adjustmentDate(1), EndDate: adjustmentDate(30)}
		if _, err := NewPayrollRunService(repo).Snapshot(cycle); err != nil {
			t.Fatal(err)
		}
		if repo.prunedCycle != 7 || len(repo.retainedEmployees) != len(ids) || repo.sawSaveOutsideTx {
			t.Fatalf("cycle=%d retained=%v outsideTx=%v", repo.prunedCycle, repo.retainedEmployees, repo.sawSaveOutsideTx)
		}
	}
}

func TestPayrollCleanupErrorAbortsRun(t *testing.T) {
	failed := errors.New("cleanup failed")
	repo := &fakeRunRepo{payrollConfig: models.PayrollConfig{TaxCalculation: "None"}, pruneErr: failed}
	_, err := NewPayrollRunService(repo).Snapshot(models.PayrollCycle{ID: 7, CompaniesID: 1, StartDate: adjustmentDate(1), EndDate: adjustmentDate(30)})
	if !errors.Is(err, failed) || len(repo.saved) != 0 {
		t.Fatalf("err=%v writes=%d", err, len(repo.saved))
	}
}

func TestClosedPayrollNeverPrunesHistory(t *testing.T) {
	repo := &fakeRunRepo{}
	_, err := NewPayrollRunService(repo).Snapshot(models.PayrollCycle{ID: 7, IsClosed: true})
	if !errors.Is(err, ErrPayrollCycleIsClosed) || repo.prunedCycle != 0 {
		t.Fatalf("err=%v prunedCycle=%d", err, repo.prunedCycle)
	}
}

type inactiveAdjustmentEmployees struct{ repository.EmployeesRepository }

func (inactiveAdjustmentEmployees) FindBriefScoped(repository.AccessScope, uint) (models.Employees, error) {
	return models.Employees{ID: 9, CompaniesID: 7, IsActive: false}, nil
}

func TestCycleAdjustmentsRejectInactiveEmployeeWithSalary(t *testing.T) {
	for _, update := range []bool{false, true} {
		cycles := newAdjustmentSalaryRepo(t)
		cycles.salary = models.SalariesStructure{EmployeesID: 9, IsActive: true, EffectiveDate: adjustmentDate(1)}
		scope := repository.AccessScope{User: models.Employees{ID: 1}, AuthorizedCompanyIDs: []uint{7}}
		earningRepo := &adjustmentEarningRepo{cycles: cycles}
		earning := NewCycleEarningService(earningRepo, cycles, inactiveAdjustmentEmployees{}, adjustmentEarningTypes{})
		e := models.PayrollCycleEarning{CycleID: 4, CompaniesID: 7, EmployeesID: 9, EarningTypeID: 3, Amount: 100}
		var err error
		if update {
			_, err = earning.Update(scope, 1, e)
		} else {
			_, err = earning.Create(scope, e)
		}
		if !errors.Is(err, ErrEmployeeInactive) || earningRepo.writes != 0 {
			t.Fatalf("earning update=%v err=%v", update, err)
		}
		deductionRepo := &adjustmentDeductionRepo{cycles: cycles}
		deduction := NewCycleDeductionService(deductionRepo, cycles, inactiveAdjustmentEmployees{}, adjustmentDeductionTypes{})
		d := models.PayrollCycleDeduction{CycleID: 4, CompaniesID: 7, EmployeesID: 9, DeductionTypeID: 3, Amount: 100}
		if update {
			_, err = deduction.Update(scope, 1, d)
		} else {
			_, err = deduction.Create(scope, d)
		}
		if !errors.Is(err, ErrEmployeeInactive) || deductionRepo.writes != 0 {
			t.Fatalf("deduction update=%v err=%v", update, err)
		}
	}
}

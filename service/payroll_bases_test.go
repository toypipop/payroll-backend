package service

import (
	"testing"
	"time"

	"payroll-backend/models"
)

// unpaidLeaveRun runs one employee on a 30000 base through a January cycle with the given unpaid leave days.
func unpaidLeaveRun(t *testing.T, unpaidDays float64, payroll models.PayrollConfig) savedSnapshot {
	t.Helper()
	cycle := models.PayrollCycle{
		ID: 21, CompaniesID: 1,
		StartDate: utcDay(2026, time.January, 1), EndDate: utcDay(2026, time.January, 31),
	}
	repo := &fakeRunRepo{
		payrollConfig: payroll,
		employees:     []models.Employees{{ID: 91, CompaniesID: 1, FirstName: "Un", LastName: "Paid"}},
		activeSalary: map[uint]models.SalariesStructure{
			91: {ID: 930, BaseSalaries: 30000, EffectiveDate: utcDay(2025, time.December, 1)},
		},
		approvedLeaves: []models.LeaveRequest{
			{LeaveType: models.LeaveTypeUnpaid, TotalDays: unpaidDays, StartDate: utcDay(2026, time.January, 5), EndDate: utcDay(2026, time.January, 7)},
		},
	}
	if _, err := NewPayrollRunService(repo).Snapshot(cycle); err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	return repo.saved[0]
}

// BR-113: unpaid leave comes off the SSO and tax bases like late and absent do.
func TestUnpaidLeaveReducesSsoAndTaxBases(t *testing.T) {
	saved := unpaidLeaveRun(t, 3, models.PayrollConfig{SsoRate: 5, SsoMaxAmount: 100000, SsoMinSalary: 1650, TaxCalculation: "None"})
	got := saved.snapshot
	assertFloat(t, "SsoAmount", got.SsoAmount, 1350)          // 5% of 30000-3000
	assertFloat(t, "TaxableIncome", got.TaxableIncome, 25650) // 27000 base less the SSO contribution
	assertFloat(t, "TotalDeduction", got.TotalDeduction, 3000)
}

func TestUnpaidLeaveBelowSsoMinimumGivesNoSso(t *testing.T) {
	saved := unpaidLeaveRun(t, 30, models.PayrollConfig{SsoRate: 5, SsoMaxAmount: 100000, SsoMinSalary: 1650, TaxCalculation: "None"})
	assertFloat(t, "SsoAmount", saved.snapshot.SsoAmount, 0)
	assertFloat(t, "TaxableIncome", saved.snapshot.TaxableIncome, 0)
}

func TestUnpaidLeaveLargerThanBaseFloorsBasesAtZero(t *testing.T) {
	saved := unpaidLeaveRun(t, 40, models.PayrollConfig{SsoRate: 5, SsoMaxAmount: 100000, SsoMinSalary: 1650, TaxCalculation: "None"})
	assertFloat(t, "SsoAmount", saved.snapshot.SsoAmount, 0)
	assertFloat(t, "TaxableIncome", saved.snapshot.TaxableIncome, 0)
}

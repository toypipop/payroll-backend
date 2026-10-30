package service

import (
	"testing"
	"time"

	"payroll-backend/models"
	"payroll-backend/repository"
)

// BR-114 boundaries and rounding for the stored SSO wage base.
func TestStatutoryForSsoWageBaseBoundaries(t *testing.T) {
	cases := []struct {
		name   string
		config models.PayrollConfig
		base   float64
		want   float64
	}{
		{"exactly the minimum", models.PayrollConfig{SsoRate: 5, SsoMaxAmount: 750, SsoMinSalary: 1650}, 1650, 1650},
		{"just below the minimum", models.PayrollConfig{SsoRate: 5, SsoMaxAmount: 750, SsoMinSalary: 1650}, 1649.99, 0},
		{"exactly the cap", models.PayrollConfig{SsoRate: 5, SsoMaxAmount: 750, SsoMinSalary: 1650}, 15000, 15000},
		{"rate zero keeps the base", models.PayrollConfig{SsoRate: 0, SsoMaxAmount: 750, SsoMinSalary: 1650}, 30000, 30000},
		{"cap rounded to 2dp", models.PayrollConfig{SsoRate: 7, SsoMaxAmount: 750, SsoMinSalary: 1650}, 30000, 10714.29},
		{"zero base with zero minimum", models.PayrollConfig{SsoRate: 5, SsoMaxAmount: 750, SsoMinSalary: 0}, 0, 0},
	}
	for _, tc := range cases {
		run := runContext{payrollConfig: tc.config}
		got := run.statutoryFor(runBases{taxable: tc.base, sso: tc.base}, 0)
		assertFloat(t, tc.name, got.ssoWage, tc.want)
	}
}

func TestStatutoryForRateZeroGivesNoSso(t *testing.T) {
	run := runContext{payrollConfig: models.PayrollConfig{SsoRate: 0, SsoMaxAmount: 750, SsoMinSalary: 1650}}
	assertFloat(t, "sso", run.statutoryFor(runBases{taxable: 30000, sso: 30000}, 0).sso, 0)
}

// BR-113: SSO-deductible items and unpaid leave both come off the SSO base.
func TestUnpaidLeaveExactlyOneDayBelowMinimumBoundary(t *testing.T) {
	saved := unpaidLeaveRun(t, 29, models.PayrollConfig{SsoRate: 5, SsoMaxAmount: 100000, SsoMinSalary: 1000, TaxCalculation: "None"})
	assertFloat(t, "SsoWageBase", *saved.snapshot.SsoWageBase, 1000) // 30000 - 29000, exactly the minimum
	assertFloat(t, "SsoAmount", saved.snapshot.SsoAmount, 50)
}

// BR-112: a month with no logs at all, and a company with no attendance configuration.
func TestDeductionSummaryNoLogsCountsEveryWorkingDay(t *testing.T) {
	repo := newSummaryRepo(models.AttendanceConfig{AbsentRatePerDay: 1000, IsAbsentDeductionEnabled: true})
	repo.dayOffs = []models.DayOff{{CompaniesID: 1, Weekday: int(time.Saturday)}, {CompaniesID: 1, Weekday: int(time.Sunday)}}
	rows, err := NewAttendanceSummaryService(repo).Deductions(repository.AccessScope{}, summaryWindow(1, 31))
	if err != nil {
		t.Fatal(err)
	}
	// January 2026 has 22 weekdays.
	if rows[0].TotalAbsent != 22 {
		t.Errorf("TotalAbsent = %d, want 22", rows[0].TotalAbsent)
	}
	assertFloat(t, "AbsentAmount", rows[0].AbsentAmount, 22000)
}

func TestSummariesWithoutConfigUseDefaultScheduleAndPriceNothing(t *testing.T) {
	repo := &fakeSummaryRepo{employees: []models.Employees{{ID: 5, CompaniesID: 1}}}
	repo.logs = []models.TimeLog{timeLog(bangkokAt(2026, time.January, 2, 10, 0), bangkokAt(2026, time.January, 2, 20, 0))}
	svc := NewAttendanceSummaryService(repo)
	ded, err := svc.Deductions(repository.AccessScope{}, summaryWindow(2, 2))
	if err != nil {
		t.Fatal(err)
	}
	earn, err := svc.Earnings(repository.AccessScope{}, summaryWindow(2, 2))
	if err != nil {
		t.Fatal(err)
	}
	assertFloat(t, "late", ded[0].TotalAmount, 0)
	assertFloat(t, "overtime", earn[0].TotalAmount, 0)
}

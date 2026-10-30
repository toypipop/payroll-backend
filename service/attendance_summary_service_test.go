package service

import (
	"testing"
	"time"

	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/timeutil"
)

// fakeSummaryRepo is an in-memory AttendanceSummaryRepository.
type fakeSummaryRepo struct {
	employees []models.Employees
	configs   []models.AttendanceConfig
	holidays  []models.Holiday
	dayOffs   []models.DayOff
	leaves    []models.LeaveRequest
	logs      []models.TimeLog
}

func (f *fakeSummaryRepo) ListEmployeesScoped(repository.AccessScope) ([]models.Employees, error) {
	return f.employees, nil
}
func (f *fakeSummaryRepo) ListConfigsForCompanies([]uint) ([]models.AttendanceConfig, error) {
	return f.configs, nil
}
func (f *fakeSummaryRepo) ListHolidaysBetween([]uint, time.Time, time.Time) ([]models.Holiday, error) {
	return f.holidays, nil
}
func (f *fakeSummaryRepo) ListDayOffsForCompanies([]uint) ([]models.DayOff, error) {
	return f.dayOffs, nil
}
func (f *fakeSummaryRepo) ListApprovedLeavesOverlapping([]uint, time.Time, time.Time) ([]models.LeaveRequest, error) {
	return f.leaves, nil
}
func (f *fakeSummaryRepo) ListTimeLogsBetween([]uint, time.Time, time.Time) ([]models.TimeLog, error) {
	return f.logs, nil
}

func summaryWindow(fromDay int, toDay int) SummaryRange {
	return SummaryRange{
		From: time.Date(2026, time.January, fromDay, 0, 0, 0, 0, timeutil.BangkokLocation),
		To:   time.Date(2026, time.January, toDay, 23, 59, 59, 0, timeutil.BangkokLocation),
	}
}

func newSummaryRepo(config models.AttendanceConfig) *fakeSummaryRepo {
	config.CompaniesID = 1
	return &fakeSummaryRepo{
		employees: []models.Employees{{ID: 5, CompaniesID: 1, FirstName: "A", LastName: "B"}},
		configs:   []models.AttendanceConfig{config},
	}
}

func timeLog(in, out *time.Time) models.TimeLog {
	return models.TimeLog{EmployeesID: 5, CompaniesID: 1, CheckIn: in, CheckOut: out}
}

// BR-112: a log marked late whose check-in is on time costs nothing (payroll never counts a minute for it).
func TestDeductionSummaryLateStatusOnTimeIsNotAMinute(t *testing.T) {
	repo := newSummaryRepo(models.AttendanceConfig{LateRatePerMinute: 2, AbsentRatePerDay: 1000, IsLateDeductionEnabled: true})
	log := timeLog(bangkokAt(2026, time.January, 2, 8, 55), bangkokAt(2026, time.January, 2, 18, 0))
	log.Status = models.StatusLate
	repo.logs = []models.TimeLog{log}
	rows, err := NewAttendanceSummaryService(repo).Deductions(repository.AccessScope{}, summaryWindow(2, 2))
	if err != nil {
		t.Fatal(err)
	}
	assertFloat(t, "TotalLateMin", rows[0].TotalLateMin, 0)
	assertFloat(t, "LateAmount", rows[0].LateAmount, 0)
}

// BR-112: lateness is capped per day, not once for the window.
func TestDeductionSummaryCapsLatenessPerDay(t *testing.T) {
	repo := newSummaryRepo(models.AttendanceConfig{LateRatePerMinute: 2, AbsentRatePerDay: 1000, IsLateDeductionEnabled: true})
	repo.logs = []models.TimeLog{
		timeLog(bangkokAt(2026, time.January, 1, 19, 0), bangkokAt(2026, time.January, 1, 19, 30)),
		timeLog(bangkokAt(2026, time.January, 2, 19, 0), bangkokAt(2026, time.January, 2, 19, 30)),
	}
	rows, err := NewAttendanceSummaryService(repo).Deductions(repository.AccessScope{}, summaryWindow(1, 2))
	if err != nil {
		t.Fatal(err)
	}
	assertFloat(t, "LateAmount", rows[0].LateAmount, 2000)
	assertFloat(t, "TotalLateMin", rows[0].TotalLateMin, 1200)
}

// BR-75 / BR-76: an approved-leave day is neither absent nor late.
func TestDeductionSummarySkipsApprovedLeaveDays(t *testing.T) {
	repo := newSummaryRepo(models.AttendanceConfig{AbsentRatePerDay: 1000, IsAbsentDeductionEnabled: true})
	repo.leaves = []models.LeaveRequest{{EmployeesID: 5, StartDate: utcDay(2026, time.January, 2), EndDate: utcDay(2026, time.January, 2)}}
	rows, err := NewAttendanceSummaryService(repo).Deductions(repository.AccessScope{}, summaryWindow(2, 2))
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].TotalAbsent != 0 {
		t.Errorf("TotalAbsent = %d, want 0", rows[0].TotalAbsent)
	}
}

// BR-112: overtime is summed per log, not taken from the latest check-out of the day.
func TestEarningSummaryOvertimePerLog(t *testing.T) {
	repo := newSummaryRepo(models.AttendanceConfig{OvertimeRatePerMinute: 1, IsOvertimeEarningsEnabled: true})
	repo.logs = []models.TimeLog{
		timeLog(bangkokAt(2026, time.January, 2, 9, 0), bangkokAt(2026, time.January, 2, 19, 0)),  // 60 min past 18:00
		timeLog(bangkokAt(2026, time.January, 2, 20, 0), bangkokAt(2026, time.January, 2, 21, 0)), // 180 min past 18:00
	}
	rows, err := NewAttendanceSummaryService(repo).Earnings(repository.AccessScope{}, summaryWindow(2, 2))
	if err != nil {
		t.Fatal(err)
	}
	assertFloat(t, "TotalOvertimeMinutes", rows[0].TotalOvertimeMinutes, 240)
	assertFloat(t, "OvertimeAmount", rows[0].OvertimeAmount, 240)
}

// BR-112: each paired log on a holiday is one extra day, not one per distinct date.
func TestEarningSummaryExtraDayPerLog(t *testing.T) {
	repo := newSummaryRepo(models.AttendanceConfig{ExtraRatePerDay: 500, IsExtraEarningsEnabled: true})
	repo.holidays = []models.Holiday{{CompaniesID: 1, Date: utcDay(2026, time.January, 2)}}
	repo.logs = []models.TimeLog{
		timeLog(bangkokAt(2026, time.January, 2, 9, 0), bangkokAt(2026, time.January, 2, 12, 0)),
		timeLog(bangkokAt(2026, time.January, 2, 13, 0), bangkokAt(2026, time.January, 2, 17, 0)),
	}
	rows, err := NewAttendanceSummaryService(repo).Earnings(repository.AccessScope{}, summaryWindow(2, 2))
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].TotalExtraDays != 2 {
		t.Errorf("TotalExtraDays = %d, want 2", rows[0].TotalExtraDays)
	}
	assertFloat(t, "ExtraAmount", rows[0].ExtraAmount, 1000)
}

// Overnight shifts keep working: a 22:00-06:00 shift ends the next morning.
func TestEarningSummaryOvernightShift(t *testing.T) {
	start := time.Date(2026, 1, 1, 22, 0, 0, 0, timeutil.BangkokLocation)
	end := time.Date(2026, 1, 1, 6, 0, 0, 0, timeutil.BangkokLocation)
	repo := newSummaryRepo(models.AttendanceConfig{WorkStartTime: &start, WorkEndTime: &end, OvertimeRatePerMinute: 1, IsOvertimeEarningsEnabled: true})
	repo.logs = []models.TimeLog{
		timeLog(bangkokAt(2026, time.January, 2, 22, 0), bangkokAt(2026, time.January, 3, 7, 0)), // 60 min past the 06:00 end next day
	}
	rows, err := NewAttendanceSummaryService(repo).Earnings(repository.AccessScope{}, summaryWindow(2, 3))
	if err != nil {
		t.Fatal(err)
	}
	assertFloat(t, "TotalOvertimeMinutes", rows[0].TotalOvertimeMinutes, 60)
}

// BR-112 parity: the summary and a payroll run agree on the same fixture.
func TestSummaryMatchesPayrollRun(t *testing.T) {
	logs := []models.TimeLog{
		timeLog(bangkokAt(2026, time.January, 1, 19, 0), bangkokAt(2026, time.January, 1, 19, 30)),
		timeLog(bangkokAt(2026, time.January, 2, 9, 0), bangkokAt(2026, time.January, 2, 19, 0)),
		timeLog(bangkokAt(2026, time.January, 2, 20, 0), bangkokAt(2026, time.January, 2, 21, 0)),
	}
	config := models.AttendanceConfig{
		CompaniesID: 1, LateRatePerMinute: 2, AbsentRatePerDay: 1000, OvertimeRatePerMinute: 1,
		IsLateDeductionEnabled: true, IsAbsentDeductionEnabled: true, IsOvertimeEarningsEnabled: true,
	}
	summary := newSummaryRepo(config)
	summary.logs = logs
	service := NewAttendanceSummaryService(summary)
	ded, _ := service.Deductions(repository.AccessScope{}, summaryWindow(1, 2))
	earn, _ := service.Earnings(repository.AccessScope{}, summaryWindow(1, 2))

	cycle := models.PayrollCycle{ID: 1, CompaniesID: 1, StartDate: utcDay(2026, time.January, 1), EndDate: utcDay(2026, time.January, 2)}
	run := &fakeRunRepo{
		payrollConfig:    models.PayrollConfig{SsoMinSalary: 999999, TaxCalculation: "None"},
		employees:        []models.Employees{{ID: 5, CompaniesID: 1}},
		attendanceConfig: config,
		activeSalary:     map[uint]models.SalariesStructure{5: {ID: 1, BaseSalaries: 30000, EffectiveDate: utcDay(2025, time.December, 1)}},
		timeLogs:         logs,
	}
	if _, err := NewPayrollRunService(run).Snapshot(cycle); err != nil {
		t.Fatal(err)
	}
	var late, overtime float64
	for _, d := range run.saved[0].details {
		switch d.ItemName {
		case "Late Amount":
			late = d.Amount
		case "Overtime":
			overtime = d.Amount
		}
	}
	assertFloat(t, "late", ded[0].LateAmount, late)
	assertFloat(t, "overtime", earn[0].OvertimeAmount, overtime)
}

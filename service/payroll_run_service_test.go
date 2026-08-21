package service

import (
	"errors"
	"testing"
	"time"

	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/timeutil"
)

// errNoRow stands in for the not-found the real repository returns when a lookup misses.
var errNoRow = errors.New("no row")

// fakeRunRepo is an in-memory PayrollRunRepository, so the calculation can be exercised without a database.
type fakeRunRepo struct {
	payrollConfig     models.PayrollConfig
	payrollConfigErr  error
	employees         []models.Employees
	attendanceConfig  models.AttendanceConfig
	attendanceCfgErr  error
	holidays          []models.Holiday
	dayOffs           []models.DayOff
	activeSalary      map[uint]models.SalariesStructure
	activeSalaryErr   error
	earliestSalary    map[uint]models.SalariesStructure
	earliestSalaryErr error
	employeeEarnings  []models.EmployeeEarning
	cycleEarnings     []models.PayrollCycleEarning
	employeeDeducts   []models.EmployeeDeduction
	cycleDeducts      []models.PayrollCycleDeduction
	timeLogs          []models.TimeLog
	approvedLeaves    []models.LeaveRequest
	saved             []savedSnapshot
	deletedEmployees  []uint
	saveSnapshotError error
	lockedCycle       models.PayrollCycle
	txDepth           int
	sawSaveOutsideTx  bool
}

// savedSnapshot is one call the run made to SaveSnapshot.
type savedSnapshot struct {
	employeeID uint
	snapshot   repository.TransactionSnapshot
	details    []models.PayrollTransactionDetail
}

func (f *fakeRunRepo) FindLatestPayrollConfig(uint) (models.PayrollConfig, error) {
	return f.payrollConfig, f.payrollConfigErr
}

func (f *fakeRunRepo) ListActiveEmployees(uint) ([]models.Employees, error) {
	return f.employees, nil
}

func (f *fakeRunRepo) FindAttendanceConfigForRun(uint) (models.AttendanceConfig, error) {
	return f.attendanceConfig, f.attendanceCfgErr
}

func (f *fakeRunRepo) ListHolidaysInRange(uint, time.Time, time.Time) ([]models.Holiday, error) {
	return f.holidays, nil
}

func (f *fakeRunRepo) ListDayOffs(uint) ([]models.DayOff, error) {
	return f.dayOffs, nil
}

func (f *fakeRunRepo) FindActiveSalaryOnOrBefore(employeeID uint, _ time.Time) (models.SalariesStructure, error) {
	if f.activeSalaryErr != nil {
		return models.SalariesStructure{}, f.activeSalaryErr
	}
	record, ok := f.activeSalary[employeeID]
	if !ok {
		return models.SalariesStructure{}, repository.ErrPayrollSalaryMissing
	}
	return record, nil
}

func (f *fakeRunRepo) FindEarliestActiveSalaryOnOrBefore(employeeID uint, _ time.Time) (models.SalariesStructure, error) {
	if f.earliestSalaryErr != nil {
		return models.SalariesStructure{}, f.earliestSalaryErr
	}
	record, ok := f.earliestSalary[employeeID]
	if !ok {
		return models.SalariesStructure{}, repository.ErrPayrollSalaryMissing
	}
	return record, nil
}

func (f *fakeRunRepo) DeleteTransactionForEmployee(_ uint, employeeID uint) error {
	f.deletedEmployees = append(f.deletedEmployees, employeeID)
	return nil
}

func (f *fakeRunRepo) ListEmployeeEarnings(uint) ([]models.EmployeeEarning, error) {
	return f.employeeEarnings, nil
}

func (f *fakeRunRepo) ListCycleEarnings(uint, uint) ([]models.PayrollCycleEarning, error) {
	return f.cycleEarnings, nil
}

func (f *fakeRunRepo) ListEmployeeDeductions(uint) ([]models.EmployeeDeduction, error) {
	return f.employeeDeducts, nil
}

func (f *fakeRunRepo) ListCycleDeductions(uint, uint) ([]models.PayrollCycleDeduction, error) {
	return f.cycleDeducts, nil
}

func (f *fakeRunRepo) ListPairedTimeLogs(uint, time.Time, time.Time) ([]models.TimeLog, error) {
	return f.timeLogs, nil
}

func (f *fakeRunRepo) ListApprovedLeavesInRange(uint, time.Time, time.Time) ([]models.LeaveRequest, error) {
	return f.approvedLeaves, nil
}

// WithinTx runs the body inline and records that a transaction was open, so a test can assert the run wrote inside one.
func (f *fakeRunRepo) WithinTx(fn func(repository.PayrollRunRepository) error) error {
	f.txDepth++
	defer func() { f.txDepth-- }()
	return fn(f)
}

func (f *fakeRunRepo) LockCycleForUpdate(uint) (models.PayrollCycle, error) {
	return f.lockedCycle, nil
}

func (f *fakeRunRepo) SaveSnapshot(_ uint, employeeID uint, snapshot repository.TransactionSnapshot, details []models.PayrollTransactionDetail) error {
	if f.txDepth == 0 {
		f.sawSaveOutsideTx = true
	}
	if f.saveSnapshotError != nil {
		return f.saveSnapshotError
	}
	f.saved = append(f.saved, savedSnapshot{employeeID: employeeID, snapshot: snapshot, details: details})
	return nil
}

// The remaining methods are outside the calculation path and are never called by these tests.
func (f *fakeRunRepo) FindCycleScoped(repository.AccessScope, uint) (models.PayrollCycle, error) {
	return models.PayrollCycle{}, errNoRow
}

func (f *fakeRunRepo) FindOpenCycleScoped(repository.AccessScope, uint) (models.PayrollCycle, error) {
	return models.PayrollCycle{}, errNoRow
}

func (f *fakeRunRepo) ListCyclesScoped(repository.AccessScope) ([]models.PayrollCycle, error) {
	return nil, nil
}
func (f *fakeRunRepo) CreateCycle(*models.PayrollCycle) error      { return nil }
func (f *fakeRunRepo) MarkCycleClosed(*models.PayrollCycle) error  { return nil }
func (f *fakeRunRepo) CountLivePayrollConfigs(uint) (int64, error) { return 0, nil }

func (f *fakeRunRepo) ListActiveEmployeesBrief(uint) ([]models.Employees, error) { return nil, nil }

func (f *fakeRunRepo) ListEmployeeIDsWithSalaryUpTo([]uint, time.Time) ([]uint, error) {
	return nil, nil
}

func (f *fakeRunRepo) AggregateCycle(uint) (repository.CycleAggregate, error) {
	return repository.CycleAggregate{}, nil
}

func (f *fakeRunRepo) FindPreviousClosedCycle(uint, time.Time) (models.PayrollCycle, bool, error) {
	return models.PayrollCycle{}, false, nil
}

func (f *fakeRunRepo) ListTransactionsForCycle(uint) ([]models.PayrollTransaction, error) {
	return nil, nil
}

func (f *fakeRunRepo) FindTransactionDetail(uint) (models.PayrollTransaction, error) {
	return models.PayrollTransaction{}, errNoRow
}

func (f *fakeRunRepo) FindCompany(uint) (models.Companies, error) { return models.Companies{}, nil }

func (f *fakeRunRepo) ListClosedCycleTransactionsForEmployee(uint) ([]models.PayrollTransaction, error) {
	return nil, nil
}

func (f *fakeRunRepo) ListCyclesByIDs([]uint) ([]models.PayrollCycle, error) { return nil, nil }

// utcDay is a stored calendar date, the shape the cycle and leave columns hold.
func utcDay(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// bangkokAt is a wall-clock instant in the business timezone, the shape a time log holds.
func bangkokAt(year int, month time.Month, day int, hour int, minute int) *time.Time {
	value := time.Date(year, month, day, hour, minute, 0, 0, timeutil.BangkokLocation)
	return &value
}

// assertFloat compares a money figure against its expected value.
func assertFloat(t *testing.T, label string, got float64, want float64) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

// TestSnapshotComputesFullPayslip pins the arithmetic of a run that exercises every input the calculation reads.
func TestSnapshotComputesFullPayslip(t *testing.T) {
	cycle := models.PayrollCycle{
		ID:          7,
		CompaniesID: 1,
		StartDate:   utcDay(2026, time.January, 1),
		EndDate:     utcDay(2026, time.January, 5),
	}

	employmentType := models.EmploymentType{Name: "Full Time"}
	bank := models.Bank{BankName: "Test Bank"}
	employee := models.Employees{
		ID:          42,
		CompaniesID: 1,
		FirstName:   "Somchai",
		LastName:    "Jaidee",
		AccountName: "Somchai Jaidee",
		AccountNo:   "1234567890",
		Position:    models.Position{PosName: "Engineer"},
		Department:  models.Department{DeptName: "IT"},

		EmploymentType: &employmentType,
		Bank:           &bank,
	}

	repo := &fakeRunRepo{
		payrollConfig: models.PayrollConfig{
			SsoRate: 5, SsoMaxAmount: 750, SsoMinSalary: 1650, TaxCalculation: "Progressive",
		},
		employees: []models.Employees{employee},
		attendanceConfig: models.AttendanceConfig{
			LateRatePerMinute:         2,
			AbsentRatePerDay:          1000,
			IsLateDeductionEnabled:    true,
			IsAbsentDeductionEnabled:  true,
			OvertimeRatePerMinute:     5,
			IsOvertimeEarningsEnabled: true,
			IsExtraEarningsEnabled:    false,
		},
		activeSalary: map[uint]models.SalariesStructure{
			42: {ID: 900, BaseSalaries: 30000, EffectiveDate: utcDay(2025, time.December, 1)},
		},
		employeeEarnings: []models.EmployeeEarning{
			{EarningTypeID: 11, Amount: 2000, EarningType: models.EarningType{Name: "Allowance", IsTaxable: true, IsSso: true}},
		},
		cycleEarnings: []models.PayrollCycleEarning{
			{EarningTypeID: 12, Amount: 1000, EarningType: models.EarningType{Name: "Bonus", IsTaxable: true, IsSso: false}},
		},
		employeeDeducts: []models.EmployeeDeduction{
			{DeductionTypeID: 21, Amount: 500, DeductionType: models.DeductionType{Name: "Loan", IsTaxDeductible: false, IsSsoDeductible: false}},
		},
		cycleDeducts: []models.PayrollCycleDeduction{
			{DeductionTypeID: 22, Amount: 200, DeductionType: models.DeductionType{Name: "Fine", IsTaxDeductible: true, IsSsoDeductible: false}},
		},
		timeLogs: []models.TimeLog{
			// 15 minutes late against the 09:00 default start, an hour past the 18:00 default end.
			{CheckIn: bangkokAt(2026, time.January, 1, 9, 15), CheckOut: bangkokAt(2026, time.January, 1, 19, 0)},
			{CheckIn: bangkokAt(2026, time.January, 2, 8, 50), CheckOut: bangkokAt(2026, time.January, 2, 18, 0)},
		},
		approvedLeaves: []models.LeaveRequest{
			{LeaveType: models.LeaveTypeUnpaid, TotalDays: 1, StartDate: utcDay(2026, time.January, 5), EndDate: utcDay(2026, time.January, 5)},
		},
	}

	result, err := NewPayrollRunService(repo).Snapshot(cycle)
	if err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	if result.ProcessedCount != 1 || result.SkippedCount != 0 {
		t.Fatalf("result = %+v, want 1 processed and 0 skipped", result)
	}
	if len(result.Prorated) != 0 {
		t.Errorf("Prorated = %+v, want empty for a salary in force before the cycle", result.Prorated)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("SaveSnapshot called %d times, want 1", len(repo.saved))
	}

	got := repo.saved[0].snapshot
	// Base 30000 + allowance 2000 + bonus 1000 + overtime 60min * 5.
	assertFloat(t, "TotalEarning", got.TotalEarning, 33300)
	// Unpaid leave 30000/30 + loan 500 + fine 200 + late 15min * 2 + 2 absent days * 1000.
	assertFloat(t, "TotalDeduction", got.TotalDeduction, 3730)
	// 5% of a 30270 base exceeds the 750 ceiling.
	assertFloat(t, "SsoAmount", got.SsoAmount, 750)
	// Taxable 31270 less the tax-deductible fine and the employee's own SSO.
	assertFloat(t, "TaxableIncome", got.TaxableIncome, 30320)
	assertFloat(t, "TaxAmount", got.TaxAmount, 224.33)
	assertFloat(t, "NetPay", got.NetPay, 28595.67)
	assertFloat(t, "SalariesAtTime", got.SalariesAtTime, 30000)
	if got.SalariesStructureID != 900 {
		t.Errorf("SalariesStructureID = %d, want 900", got.SalariesStructureID)
	}
	if got.PositionName != "Engineer" || got.DepartmentName != "IT" {
		t.Errorf("position/department = %q/%q, want Engineer/IT", got.PositionName, got.DepartmentName)
	}
	if got.EmploymentTypeName != "Full Time" || got.BankName != "Test Bank" {
		t.Errorf("employment/bank = %q/%q, want Full Time/Test Bank", got.EmploymentTypeName, got.BankName)
	}

	wantDetails := []struct {
		name   string
		amount float64
		kind   string
	}{
		{"Base Salary", 30000, "earning"},
		{"Allowance", 2000, "earning"},
		{"Bonus", 1000, "earning"},
		{"Overtime", 300, "earning"},
		{"Loan", 500, "deduction"},
		{"Fine", 200, "deduction"},
		{"Unpaid Leave", 1000, "deduction"},
		{"Late Amount", 30, "deduction"},
		{"Absent Amount", 2000, "deduction"},
		{"Social Security (SSO)", 750, "deduction"},
		{"Income Tax", 224.33, "deduction"},
	}
	details := repo.saved[0].details
	if len(details) != len(wantDetails) {
		t.Fatalf("details = %d lines, want %d: %+v", len(details), len(wantDetails), details)
	}
	for i, want := range wantDetails {
		if details[i].ItemName != want.name || details[i].Amount != want.amount || details[i].Type != want.kind {
			t.Errorf("detail %d = %q %v %q, want %q %v %q",
				i, details[i].ItemName, details[i].Amount, details[i].Type, want.name, want.amount, want.kind)
		}
	}
}

// TestSnapshotProratesMidCycleSalary covers the employee whose only salary record takes effect after the cycle opens.
func TestSnapshotProratesMidCycleSalary(t *testing.T) {
	cycle := models.PayrollCycle{
		ID:          8,
		CompaniesID: 1,
		StartDate:   utcDay(2026, time.January, 1),
		EndDate:     utcDay(2026, time.January, 31),
	}

	repo := &fakeRunRepo{
		// A floor above the base keeps social security out of this case, and "None" keeps tax out.
		payrollConfig: models.PayrollConfig{SsoMinSalary: 999999, TaxCalculation: "None"},
		employees: []models.Employees{
			{ID: 55, CompaniesID: 1, FirstName: "Malee", LastName: "Rakdee"},
		},
		earliestSalary: map[uint]models.SalariesStructure{
			55: {ID: 901, BaseSalaries: 31000, EffectiveDate: utcDay(2026, time.January, 16)},
		},
	}

	result, err := NewPayrollRunService(repo).Snapshot(cycle)
	if err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	if result.ProcessedCount != 1 {
		t.Fatalf("ProcessedCount = %d, want 1", result.ProcessedCount)
	}
	if len(result.Prorated) != 1 {
		t.Fatalf("Prorated = %+v, want one entry", result.Prorated)
	}
	if result.Prorated[0] != (ProratedEmployee{Name: "Malee Rakdee", Days: 16, Total: 31}) {
		t.Errorf("Prorated[0] = %+v, want Malee Rakdee 16/31", result.Prorated[0])
	}

	// 31000 paid for 16 of the cycle's 31 days.
	assertFloat(t, "SalariesAtTime", repo.saved[0].snapshot.SalariesAtTime, 16000)
	assertFloat(t, "NetPay", repo.saved[0].snapshot.NetPay, 16000)

	details := repo.saved[0].details
	if len(details) != 1 || details[0].ItemName != "Base Salary (Prorated 16/31 days)" {
		t.Errorf("details = %+v, want a single pro-rated base salary line", details)
	}
}

// TestSnapshotSkipsEmployeeWithoutSalary covers the employee no salary record applies to, whose stale transaction must be cleared.
func TestSnapshotSkipsEmployeeWithoutSalary(t *testing.T) {
	cycle := models.PayrollCycle{
		ID:          9,
		CompaniesID: 1,
		StartDate:   utcDay(2026, time.January, 1),
		EndDate:     utcDay(2026, time.January, 31),
	}

	repo := &fakeRunRepo{
		payrollConfig: models.PayrollConfig{SsoMinSalary: 999999, TaxCalculation: "None"},
		employees: []models.Employees{
			{ID: 61, CompaniesID: 1, FirstName: "Anan", LastName: "Nopadol"},
		},
	}

	result, err := NewPayrollRunService(repo).Snapshot(cycle)
	if err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	if result.ProcessedCount != 0 || result.SkippedCount != 1 {
		t.Fatalf("result = %+v, want 0 processed and 1 skipped", result)
	}
	if len(result.SkippedNames) != 1 || result.SkippedNames[0] != "Anan Nopadol" {
		t.Errorf("SkippedNames = %v, want [Anan Nopadol]", result.SkippedNames)
	}
	if len(repo.saved) != 0 {
		t.Errorf("SaveSnapshot called %d times, want 0", len(repo.saved))
	}
	if len(repo.deletedEmployees) != 1 || repo.deletedEmployees[0] != 61 {
		t.Errorf("deletedEmployees = %v, want [61]", repo.deletedEmployees)
	}
}

// TestLateDeductionCapsEachDayNotTheCycle covers two heavily late days: each is capped at the absence rate on its own, so the cycle's lateness costs twice the cap rather than once.
func TestLateDeductionCapsEachDayNotTheCycle(t *testing.T) {
	cycle := models.PayrollCycle{
		ID:          11,
		CompaniesID: 1,
		StartDate:   utcDay(2026, time.January, 1),
		EndDate:     utcDay(2026, time.January, 2),
	}

	repo := &fakeRunRepo{
		payrollConfig: models.PayrollConfig{SsoMinSalary: 999999, TaxCalculation: "None"},
		employees: []models.Employees{
			{ID: 81, CompaniesID: 1, FirstName: "Late", LastName: "Everyday"},
		},
		attendanceConfig: models.AttendanceConfig{
			LateRatePerMinute:        2,
			AbsentRatePerDay:         1000,
			IsLateDeductionEnabled:   true,
			IsAbsentDeductionEnabled: true,
		},
		activeSalary: map[uint]models.SalariesStructure{
			81: {ID: 920, BaseSalaries: 30000, EffectiveDate: utcDay(2025, time.December, 1)},
		},
		timeLogs: []models.TimeLog{
			// 600 minutes past the 09:00 default start on each day: 1200 raw, capped to 1000 per day.
			{CheckIn: bangkokAt(2026, time.January, 1, 19, 0), CheckOut: bangkokAt(2026, time.January, 1, 19, 30)},
			{CheckIn: bangkokAt(2026, time.January, 2, 19, 0), CheckOut: bangkokAt(2026, time.January, 2, 19, 30)},
		},
	}

	if _, err := NewPayrollRunService(repo).Snapshot(cycle); err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}

	got := repo.saved[0].snapshot
	assertFloat(t, "TotalDeduction", got.TotalDeduction, 2000)
	assertFloat(t, "NetPay", got.NetPay, 28000)

	details := repo.saved[0].details
	if len(details) != 2 || details[1].ItemName != "Late Amount" {
		t.Fatalf("details = %+v, want a base salary line and a late line", details)
	}
	assertFloat(t, "Late Amount", details[1].Amount, 2000)
}

// TestSnapshotReportsSalaryLookupFailure covers a database failure while reading a salary: it must surface as an error, never as an employee with no salary.
func TestSnapshotReportsSalaryLookupFailure(t *testing.T) {
	cycle := models.PayrollCycle{
		ID:          12,
		CompaniesID: 1,
		StartDate:   utcDay(2026, time.January, 1),
		EndDate:     utcDay(2026, time.January, 31),
	}

	lookupErr := errors.New("connection reset")
	repo := &fakeRunRepo{
		payrollConfig: models.PayrollConfig{SsoMinSalary: 999999, TaxCalculation: "None"},
		employees: []models.Employees{
			{ID: 91, CompaniesID: 1, FirstName: "Unread", LastName: "Employee"},
		},
		activeSalaryErr: lookupErr,
	}

	result, err := NewPayrollRunService(repo).Snapshot(cycle)
	if !errors.Is(err, lookupErr) {
		t.Fatalf("Snapshot error = %v, want %v", err, lookupErr)
	}
	if result.SkippedCount != 0 || len(result.SkippedNames) != 0 {
		t.Errorf("result = %+v, want the employee reported as a failure, not skipped", result)
	}
	if len(repo.deletedEmployees) != 0 {
		t.Errorf("deletedEmployees = %v, want none: an unreadable salary must not clear a stored transaction", repo.deletedEmployees)
	}
	if len(repo.saved) != 0 {
		t.Errorf("SaveSnapshot called %d times, want 0", len(repo.saved))
	}
}

// TestSnapshotAbortsRunOnFirstSaveFailure pins the run's atomicity: a save failure stops the run there and is reported, instead of letting the employees after it write while the caller sees only an error.
func TestSnapshotAbortsRunOnFirstSaveFailure(t *testing.T) {
	cycle := models.PayrollCycle{
		ID:          10,
		CompaniesID: 1,
		StartDate:   utcDay(2026, time.January, 1),
		EndDate:     utcDay(2026, time.January, 31),
	}

	saveErr := errors.New("write failed")
	repo := &fakeRunRepo{
		payrollConfig: models.PayrollConfig{SsoMinSalary: 999999, TaxCalculation: "None"},
		employees: []models.Employees{
			{ID: 71, CompaniesID: 1, FirstName: "First", LastName: "Employee"},
			{ID: 72, CompaniesID: 1, FirstName: "Second", LastName: "Employee"},
		},
		activeSalary: map[uint]models.SalariesStructure{
			71: {ID: 910, BaseSalaries: 20000, EffectiveDate: utcDay(2025, time.December, 1)},
			72: {ID: 911, BaseSalaries: 20000, EffectiveDate: utcDay(2025, time.December, 1)},
		},
		saveSnapshotError: saveErr,
	}

	result, err := NewPayrollRunService(repo).Snapshot(cycle)
	if !errors.Is(err, saveErr) {
		t.Fatalf("Snapshot error = %v, want %v", err, saveErr)
	}
	if result.ProcessedCount != 0 {
		t.Errorf("ProcessedCount = %d, want 0 when the run aborted", result.ProcessedCount)
	}
	if repo.sawSaveOutsideTx {
		t.Error("SaveSnapshot ran outside a transaction; the whole run must commit or roll back as one unit")
	}
}

// TestSnapshotWritesInsideOneTransaction guards the property the unique index and the row lock both rely on: every employee of a cycle is written inside a single transaction.
func TestSnapshotWritesInsideOneTransaction(t *testing.T) {
	cycle := models.PayrollCycle{
		ID:          13,
		CompaniesID: 1,
		StartDate:   utcDay(2026, time.January, 1),
		EndDate:     utcDay(2026, time.January, 31),
	}

	repo := &fakeRunRepo{
		payrollConfig: models.PayrollConfig{SsoMinSalary: 999999, TaxCalculation: "None"},
		employees: []models.Employees{
			{ID: 101, CompaniesID: 1, FirstName: "First", LastName: "Employee"},
			{ID: 102, CompaniesID: 1, FirstName: "Second", LastName: "Employee"},
		},
		activeSalary: map[uint]models.SalariesStructure{
			101: {ID: 930, BaseSalaries: 20000, EffectiveDate: utcDay(2025, time.December, 1)},
			102: {ID: 931, BaseSalaries: 20000, EffectiveDate: utcDay(2025, time.December, 1)},
		},
	}

	if _, err := NewPayrollRunService(repo).Snapshot(cycle); err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	if len(repo.saved) != 2 {
		t.Fatalf("SaveSnapshot called %d times, want 2", len(repo.saved))
	}
	if repo.sawSaveOutsideTx {
		t.Error("SaveSnapshot ran outside a transaction")
	}
}

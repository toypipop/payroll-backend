package service

import (
	"fmt"
	"math"
	"time"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/timeutil"
)

// Fallback schedule when a company has no attendance configuration.
const (
	runDefaultStartHour = 9
	runDefaultStartMin  = 0
	runDefaultEndHour   = 18
	runDefaultEndMin    = 0
)

// unpaidLeaveDivisor prices a day of unpaid leave as a thirtieth of base salary.
const unpaidLeaveDivisor = 30

// Cycle rule failures.
var (
	ErrPayrollCycleEndBeforeStart  = apperr.Validation("end_date must be after start_date")
	ErrPayrollCycleCompanyDenied   = apperr.Forbidden("Access denied to the specified company")
	ErrPayrollCycleNotFoundForList = apperr.NotFound("Cycle not found")
)

// Re-exported so handlers import a single package.
var (
	ErrPayrollCycleMissing       = repository.ErrPayrollCycleMissing
	ErrPayrollCycleIsClosed      = repository.ErrPayrollCycleIsClosed
	ErrPayrollCycleAlreadyClosed = repository.ErrPayrollCycleAlreadyClosed
	ErrPayrollTransactionMissing = repository.ErrPayrollTransactionMissing
	ErrPayrollSlipDenied         = repository.ErrPayrollSlipDenied
	ErrPayrollSlipCycleOpen      = repository.ErrPayrollSlipCycleOpen
)

// ProratedEmployee names an employee whose base salary this run pro-rated by calendar days.
type ProratedEmployee struct {
	Name  string `json:"name"`
	Days  int    `json:"days"`
	Total int    `json:"total_days"`
}

// RunResult reports what a calculation covered.
type RunResult struct {
	ProcessedCount int
	SkippedCount   int
	SkippedNames   []string
	Prorated       []ProratedEmployee
}

// PayrollRunService owns the payroll calculation, from the salary that applies through to tax and social security.
type PayrollRunService struct {
	repo repository.PayrollRunRepository
}

// NewPayrollRunService wires the service to a repository implementation.
func NewPayrollRunService(repo repository.PayrollRunRepository) *PayrollRunService {
	return &PayrollRunService{repo: repo}
}

// dayOf truncates to the start of a stored calendar date.
func dayOf(value time.Time) time.Time {
	return value.Truncate(24 * time.Hour)
}

// dateKey renders a stored calendar date as the map key both the log and the day-by-day loops use.
func dateKey(value time.Time) string {
	return value.Format("2006-01-02")
}

// scheduleWindow is a company's working hours, resolved once per run.
type scheduleWindow struct {
	startHour, startMin int
	endHour, endMin     int
}

// scheduledStart is the shift start on a given calendar day, in the business timezone.
func (w scheduleWindow) scheduledStart(day time.Time) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), w.startHour, w.startMin, 0, 0, timeutil.BangkokLocation)
}

// scheduledEnd is the shift end on a day, pushed to the next day when the shift runs past midnight.
func (w scheduleWindow) scheduledEnd(day time.Time) time.Time {
	end := time.Date(day.Year(), day.Month(), day.Day(), w.endHour, w.endMin, 0, 0, timeutil.BangkokLocation)
	if w.endHour < w.startHour || (w.endHour == w.startHour && w.endMin <= w.startMin) {
		end = end.AddDate(0, 0, 1)
	}
	return end
}

// logDay collapses an employee's paired logs for one calendar day.
type logDay struct {
	hasCheckPair    bool
	earliestCheckIn time.Time
}

// Cycles returns the cycles the caller's role authorizes, newest first.
func (s *PayrollRunService) Cycles(scope repository.AccessScope) ([]models.PayrollCycle, error) {
	return s.repo.ListCyclesScoped(scope)
}

// CreateCycle opens a new cycle for a company the caller is authorized for.
func (s *PayrollRunService) CreateCycle(scope repository.AccessScope, name string, start time.Time, end time.Time, companiesID uint) (models.PayrollCycle, error) {
	if !end.After(start) {
		return models.PayrollCycle{}, ErrPayrollCycleEndBeforeStart
	}
	companyID, ok := scope.ResolveCompanyID(companiesID)
	if !ok {
		return models.PayrollCycle{}, ErrPayrollCycleCompanyDenied
	}

	cycle := models.PayrollCycle{
		CompaniesID: companyID,
		CycleName:   name,
		StartDate:   start,
		EndDate:     end,
	}
	if err := s.repo.CreateCycle(&cycle); err != nil {
		return models.PayrollCycle{}, err
	}
	return cycle, nil
}

// Close snapshots the cycle's transactions and then marks it closed, so a closed cycle always has the figures it was closed on.
func (s *PayrollRunService) Close(scope repository.AccessScope, cycleID uint) (models.PayrollCycle, RunResult, error) {
	cycle, err := s.repo.FindCycleScoped(scope, cycleID)
	if err != nil {
		return models.PayrollCycle{}, RunResult{}, err
	}
	if cycle.IsClosed {
		return models.PayrollCycle{}, RunResult{}, ErrPayrollCycleAlreadyClosed
	}

	result, err := s.Snapshot(cycle)
	if err != nil {
		return models.PayrollCycle{}, RunResult{}, err
	}
	if err := s.repo.MarkCycleClosed(&cycle); err != nil {
		return models.PayrollCycle{}, RunResult{}, err
	}
	cycle.IsClosed = true
	return cycle, result, nil
}

// Recalculate reruns an open cycle. A closed cycle is immutable.
func (s *PayrollRunService) Recalculate(scope repository.AccessScope, cycleID uint) (models.PayrollCycle, RunResult, error) {
	cycle, err := s.repo.FindCycleScoped(scope, cycleID)
	if err != nil {
		return models.PayrollCycle{}, RunResult{}, err
	}
	if cycle.IsClosed {
		return models.PayrollCycle{}, RunResult{}, ErrPayrollCycleIsClosed
	}

	result, err := s.Snapshot(cycle)
	if err != nil {
		return models.PayrollCycle{}, RunResult{}, err
	}
	return cycle, result, nil
}

// Snapshot recalculates and persists the transactions and detail lines for one cycle.
func (s *PayrollRunService) Snapshot(cycle models.PayrollCycle) (RunResult, error) {
	companiesID := cycle.CompaniesID

	payrollConfig, err := s.repo.FindLatestPayrollConfig(companiesID)
	if err != nil {
		return RunResult{}, err
	}
	employees, err := s.repo.ListActiveEmployees(companiesID)
	if err != nil {
		return RunResult{}, err
	}

	// No attendance configuration leaves the deduction flags enabled so payroll still runs, which is the long-standing default.
	attendanceConfig, err := s.repo.FindAttendanceConfigForRun(companiesID)
	if err != nil {
		attendanceConfig = models.AttendanceConfig{
			IsLateDeductionEnabled:   true,
			IsAbsentDeductionEnabled: true,
		}
	}

	holidays, err := s.repo.ListHolidaysInRange(companiesID, cycle.StartDate, cycle.EndDate)
	if err != nil {
		return RunResult{}, err
	}
	holidaySet := make(map[string]bool, len(holidays))
	for _, holiday := range holidays {
		holidaySet[dateKey(holiday.Date)] = true
	}

	dayOffs, err := s.repo.ListDayOffs(companiesID)
	if err != nil {
		return RunResult{}, err
	}
	dayOffWeekdays := make(map[int]bool, len(dayOffs))
	for _, dayOff := range dayOffs {
		dayOffWeekdays[dayOff.Weekday] = true
	}

	schedule := scheduleWindow{
		startHour: runDefaultStartHour, startMin: runDefaultStartMin,
		endHour: runDefaultEndHour, endMin: runDefaultEndMin,
	}
	if attendanceConfig.WorkStartTime != nil {
		start := timeutil.ToBangkok(*attendanceConfig.WorkStartTime)
		schedule.startHour, schedule.startMin = start.Hour(), start.Minute()
	}
	if attendanceConfig.WorkEndTime != nil {
		end := timeutil.ToBangkok(*attendanceConfig.WorkEndTime)
		schedule.endHour, schedule.endMin = end.Hour(), end.Minute()
	}

	result := RunResult{}
	var lastErr error

	for _, employee := range employees {
		salaryRecord, isProrated, found, err := s.salaryForCycle(employee.ID, cycle)
		if err != nil {
			lastErr = err
			continue
		}
		if !found {
			result.SkippedCount++
			result.SkippedNames = append(result.SkippedNames, employee.FirstName+" "+employee.LastName)
			// Clear a transaction an earlier run wrote when this employee did have a usable salary record, so a stale figure does not linger.
			if err := s.repo.DeleteTransactionForEmployee(cycle.ID, employee.ID); err != nil {
				lastErr = err
			}
			continue
		}

		baseSalary := salaryRecord.BaseSalaries
		proratedDays, proratedTotalDays := 0, 0
		if isProrated {
			periodStart := dayOf(cycle.StartDate)
			periodEnd := dayOf(cycle.EndDate)
			effectiveStart := dayOf(salaryRecord.EffectiveDate)
			proratedTotalDays = int(periodEnd.Sub(periodStart).Hours()/24) + 1
			proratedDays = int(periodEnd.Sub(effectiveStart).Hours()/24) + 1
			baseSalary = round2(baseSalary * float64(proratedDays) / float64(proratedTotalDays))
			result.Prorated = append(result.Prorated, ProratedEmployee{
				Name:  employee.FirstName + " " + employee.LastName,
				Days:  proratedDays,
				Total: proratedTotalDays,
			})
		}

		snapshot, details, err := s.computeEmployee(cycle, employee, salaryRecord, baseSalary, isProrated, proratedDays, proratedTotalDays, payrollConfig, attendanceConfig, schedule, holidaySet, dayOffWeekdays)
		if err != nil {
			lastErr = err
			continue
		}

		if err := s.repo.SaveSnapshot(cycle.ID, employee.ID, snapshot, details); err != nil {
			lastErr = err
			continue
		}
		result.ProcessedCount++
	}

	return result, lastErr
}

// salaryForCycle picks the salary a cycle should be paid on.
func (s *PayrollRunService) salaryForCycle(employeeID uint, cycle models.PayrollCycle) (models.SalariesStructure, bool, bool, error) {
	record, err := s.repo.FindActiveSalaryOnOrBefore(employeeID, cycle.StartDate)
	if err == nil {
		return record, false, true, nil
	}

	record, err = s.repo.FindEarliestActiveSalaryOnOrBefore(employeeID, cycle.EndDate)
	if err != nil {
		return models.SalariesStructure{}, false, false, nil
	}
	return record, true, true, nil
}

// computeEmployee turns one employee's inputs into the figures to store and the detail lines that explain them.
func (s *PayrollRunService) computeEmployee(
	cycle models.PayrollCycle,
	employee models.Employees,
	salaryRecord models.SalariesStructure,
	baseSalary float64,
	isProrated bool,
	proratedDays int,
	proratedTotalDays int,
	payrollConfig models.PayrollConfig,
	attendanceConfig models.AttendanceConfig,
	schedule scheduleWindow,
	holidaySet map[string]bool,
	dayOffWeekdays map[int]bool,
) (repository.TransactionSnapshot, []models.PayrollTransactionDetail, error) {
	taxableBase := baseSalary
	ssoBase := baseSalary

	type lineItem struct {
		name       string
		amount     float64
		typeID     *uint
		taxRelated bool
	}

	additionalEarning := 0.0
	var earningItems []lineItem

	employeeEarnings, err := s.repo.ListEmployeeEarnings(employee.ID)
	if err != nil {
		return repository.TransactionSnapshot{}, nil, err
	}
	for _, item := range employeeEarnings {
		additionalEarning += item.Amount
		if item.EarningType.IsTaxable {
			taxableBase += item.Amount
		}
		if item.EarningType.IsSso {
			ssoBase += item.Amount
		}
		typeID := item.EarningTypeID
		earningItems = append(earningItems, lineItem{item.EarningType.Name, item.Amount, &typeID, item.EarningType.IsTaxable})
	}

	cycleEarnings, err := s.repo.ListCycleEarnings(employee.ID, cycle.ID)
	if err != nil {
		return repository.TransactionSnapshot{}, nil, err
	}
	for _, item := range cycleEarnings {
		additionalEarning += item.Amount
		if item.EarningType.IsTaxable {
			taxableBase += item.Amount
		}
		if item.EarningType.IsSso {
			ssoBase += item.Amount
		}
		typeID := item.EarningTypeID
		earningItems = append(earningItems, lineItem{item.EarningType.Name, item.Amount, &typeID, item.EarningType.IsTaxable})
	}

	endOfCycle := dayOf(cycle.EndDate).AddDate(0, 0, 1)
	timeLogs, err := s.repo.ListPairedTimeLogs(employee.ID, cycle.StartDate, endOfCycle)
	if err != nil {
		return repository.TransactionSnapshot{}, nil, err
	}

	logsByDay := make(map[string]*logDay)
	overtimeMinutes := 0.0
	extraDays := 0
	for _, log := range timeLogs {
		if log.CheckIn == nil || log.CheckOut == nil {
			continue
		}
		day := timeutil.DayStart(*log.CheckIn)
		key := dateKey(day)

		// Working a holiday or a configured day off earns the extra-day rate; those days are not measured for overtime or lateness.
		if holidaySet[key] || dayOffWeekdays[int(day.Weekday())] {
			extraDays++
			continue
		}

		record := logsByDay[key]
		if record == nil {
			record = &logDay{earliestCheckIn: *log.CheckIn}
			logsByDay[key] = record
		}
		record.hasCheckPair = true
		if log.CheckIn.Before(record.earliestCheckIn) {
			record.earliestCheckIn = *log.CheckIn
		}

		if scheduledEnd := schedule.scheduledEnd(day); log.CheckOut.After(scheduledEnd) {
			overtimeMinutes += log.CheckOut.Sub(scheduledEnd).Minutes()
		}
	}

	overtimeAmount := 0.0
	if attendanceConfig.IsOvertimeEarningsEnabled {
		overtimeAmount = round2(overtimeMinutes * attendanceConfig.OvertimeRatePerMinute)
	}
	taxableBase += overtimeAmount
	ssoBase += overtimeAmount

	extraAmount := 0.0
	if attendanceConfig.IsExtraEarningsEnabled {
		extraAmount = round2(float64(extraDays) * attendanceConfig.ExtraRatePerDay)
	}
	taxableBase += extraAmount
	ssoBase += extraAmount

	additionalDeduction := 0.0
	taxDeductible := 0.0
	ssoDeductible := 0.0
	var deductionItems []lineItem

	employeeDeductions, err := s.repo.ListEmployeeDeductions(employee.ID)
	if err != nil {
		return repository.TransactionSnapshot{}, nil, err
	}
	for _, item := range employeeDeductions {
		additionalDeduction += item.Amount
		if item.DeductionType.IsTaxDeductible {
			taxDeductible += item.Amount
		}
		if item.DeductionType.IsSsoDeductible {
			ssoDeductible += item.Amount
		}
		typeID := item.DeductionTypeID
		deductionItems = append(deductionItems, lineItem{item.DeductionType.Name, item.Amount, &typeID, item.DeductionType.IsTaxDeductible})
	}

	cycleDeductions, err := s.repo.ListCycleDeductions(employee.ID, cycle.ID)
	if err != nil {
		return repository.TransactionSnapshot{}, nil, err
	}
	for _, item := range cycleDeductions {
		additionalDeduction += item.Amount
		if item.DeductionType.IsTaxDeductible {
			taxDeductible += item.Amount
		}
		if item.DeductionType.IsSsoDeductible {
			ssoDeductible += item.Amount
		}
		typeID := item.DeductionTypeID
		deductionItems = append(deductionItems, lineItem{item.DeductionType.Name, item.Amount, &typeID, item.DeductionType.IsTaxDeductible})
	}

	approvedLeaves, err := s.repo.ListApprovedLeavesInRange(employee.ID, cycle.StartDate, cycle.EndDate)
	if err != nil {
		return repository.TransactionSnapshot{}, nil, err
	}
	leaveSet := make(map[string]bool)
	unpaidDays := 0.0
	for _, leave := range approvedLeaves {
		if leave.LeaveType == models.LeaveTypeUnpaid {
			unpaidDays += leave.TotalDays
		}
		for day := dayOf(leave.StartDate); !day.After(dayOf(leave.EndDate)); day = day.AddDate(0, 0, 1) {
			leaveSet[dateKey(day)] = true
		}
	}
	unpaidLeaveDeduction := round2(unpaidDays * (baseSalary / unpaidLeaveDivisor))

	lateMinutes := 0.0
	absentDays := 0
	for day := dayOf(cycle.StartDate); !day.After(dayOf(cycle.EndDate)); day = day.AddDate(0, 0, 1) {
		key := dateKey(day)
		if holidaySet[key] || dayOffWeekdays[int(day.Weekday())] {
			continue
		}
		if leaveSet[key] {
			continue
		}
		record := logsByDay[key]
		if record == nil || !record.hasCheckPair {
			absentDays++
			continue
		}
		if scheduledStart := schedule.scheduledStart(day); record.earliestCheckIn.After(scheduledStart) {
			lateMinutes += record.earliestCheckIn.Sub(scheduledStart).Minutes()
		}
	}

	lateDeduction := 0.0
	if attendanceConfig.IsLateDeductionEnabled {
		lateDeduction = round2(lateMinutes * attendanceConfig.LateRatePerMinute)
		// A day of lateness can never cost more than a day of absence.
		if lateDeduction > attendanceConfig.AbsentRatePerDay {
			lateDeduction = attendanceConfig.AbsentRatePerDay
		}
	}
	absentDeduction := 0.0
	if attendanceConfig.IsAbsentDeductionEnabled {
		absentDeduction = round2(float64(absentDays) * attendanceConfig.AbsentRatePerDay)
	}

	// Thai law prices tax and social security on income actually earned, so time not worked comes off the base first.
	taxableBase = math.Max(0, taxableBase-lateDeduction-absentDeduction)
	ssoBase = math.Max(0, ssoBase-ssoDeductible-lateDeduction-absentDeduction)

	ssoAmount := 0.0
	if ssoBase >= payrollConfig.SsoMinSalary {
		computed := ssoBase * payrollConfig.SsoRate / 100
		if computed > payrollConfig.SsoMaxAmount {
			computed = payrollConfig.SsoMaxAmount
		}
		ssoAmount = round2(computed)
	}

	// The employee's own social security contribution is deductible from taxable income, on top of whatever deduction items are tax-deductible.
	netTaxableIncome := round2(math.Max(0, taxableBase-(taxDeductible+ssoAmount)))
	monthlyTax := MonthlyTax(payrollConfig.TaxCalculation, taxableBase, taxDeductible+ssoAmount)

	totalEarning := round2(baseSalary + additionalEarning + overtimeAmount + extraAmount)
	totalDeduction := round2(unpaidLeaveDeduction + additionalDeduction + lateDeduction + absentDeduction)
	netPay := round2(totalEarning - totalDeduction - ssoAmount - monthlyTax)

	snapshot := repository.TransactionSnapshot{
		SalariesStructureID: salaryRecord.ID,
		SalariesAtTime:      baseSalary,
		TotalEarning:        totalEarning,
		TotalDeduction:      totalDeduction,
		SsoAmount:           ssoAmount,
		TaxableIncome:       netTaxableIncome,
		TaxAmount:           monthlyTax,
		NetPay:              netPay,
		PositionName:        employee.Position.PosName,
		DepartmentName:      employee.Department.DeptName,
		BankAccountName:     employee.AccountName,
		BankAccountNo:       employee.AccountNo,
	}
	if employee.EmploymentType != nil {
		snapshot.EmploymentTypeName = employee.EmploymentType.Name
	}
	if employee.Bank != nil {
		snapshot.BankName = employee.Bank.BankName
	}

	baseSalaryLabel := "Base Salary"
	if isProrated {
		baseSalaryLabel = fmt.Sprintf("Base Salary (Prorated %d/%d days)", proratedDays, proratedTotalDays)
	}

	details := []models.PayrollTransactionDetail{
		{ItemName: baseSalaryLabel, Amount: baseSalary, Type: "earning", IsTaxRelevant: true},
	}
	for _, item := range earningItems {
		details = append(details, models.PayrollTransactionDetail{
			ItemName: item.name, Amount: item.amount, Type: "earning",
			EarningTypeID: item.typeID, IsTaxRelevant: item.taxRelated,
		})
	}
	if overtimeAmount > 0 {
		details = append(details, models.PayrollTransactionDetail{ItemName: "Overtime", Amount: overtimeAmount, Type: "earning", IsTaxRelevant: true})
	}
	if extraAmount > 0 {
		details = append(details, models.PayrollTransactionDetail{ItemName: "Extra Day", Amount: extraAmount, Type: "earning", IsTaxRelevant: true})
	}
	for _, item := range deductionItems {
		details = append(details, models.PayrollTransactionDetail{
			ItemName: item.name, Amount: item.amount, Type: "deduction",
			DeductionTypeID: item.typeID, IsTaxRelevant: item.taxRelated,
		})
	}
	if unpaidLeaveDeduction > 0 {
		details = append(details, models.PayrollTransactionDetail{ItemName: "Unpaid Leave", Amount: unpaidLeaveDeduction, Type: "deduction", IsTaxRelevant: false})
	}
	if lateDeduction > 0 {
		details = append(details, models.PayrollTransactionDetail{ItemName: "Late Amount", Amount: lateDeduction, Type: "deduction", IsTaxRelevant: true})
	}
	if absentDeduction > 0 {
		details = append(details, models.PayrollTransactionDetail{ItemName: "Absent Amount", Amount: absentDeduction, Type: "deduction", IsTaxRelevant: true})
	}
	if ssoAmount > 0 {
		details = append(details, models.PayrollTransactionDetail{ItemName: "Social Security (SSO)", Amount: ssoAmount, Type: "deduction", IsTaxRelevant: true})
	}
	if monthlyTax > 0 {
		details = append(details, models.PayrollTransactionDetail{ItemName: "Income Tax", Amount: monthlyTax, Type: "deduction", IsTaxRelevant: true})
	}

	return snapshot, details, nil
}

// PreCheckIssue is one thing worth telling the operator before a run.
type PreCheckIssue struct {
	Type      string
	Message   string
	Employees []string
}

// Issue types the pre-check can report.
const (
	IssueNoPayrollConfig = "no_payroll_config"
	IssueNoEmployees     = "no_employees"
	IssueNoSalary        = "no_salary"
	IssueNoBank          = "no_bank"
)

// PreCheck reports what would go wrong or be skipped if the cycle were run now.
func (s *PayrollRunService) PreCheck(scope repository.AccessScope, cycleID uint) ([]PreCheckIssue, error) {
	cycle, err := s.repo.FindCycleScoped(scope, cycleID)
	if err != nil {
		return nil, err
	}

	var issues []PreCheckIssue

	configCount, err := s.repo.CountLivePayrollConfigs(cycle.CompaniesID)
	if err != nil {
		return nil, err
	}
	if configCount == 0 {
		issues = append(issues, PreCheckIssue{
			Type:    IssueNoPayrollConfig,
			Message: "No payroll config found — configure it in Payroll Setup before running",
		})
	}

	employees, err := s.repo.ListActiveEmployeesBrief(cycle.CompaniesID)
	if err != nil {
		return nil, err
	}
	// With nobody to pay, the remaining checks have nothing to say.
	if len(employees) == 0 {
		return append(issues, PreCheckIssue{
			Type:    IssueNoEmployees,
			Message: "No active employees found for this company",
		}), nil
	}

	employeeIDs := make([]uint, 0, len(employees))
	for _, employee := range employees {
		employeeIDs = append(employeeIDs, employee.ID)
	}
	withSalary, err := s.repo.ListEmployeeIDsWithSalaryUpTo(employeeIDs, cycle.EndDate)
	if err != nil {
		return nil, err
	}
	hasSalary := make(map[uint]bool, len(withSalary))
	for _, id := range withSalary {
		hasSalary[id] = true
	}

	var noSalary []string
	var noBank []string
	for _, employee := range employees {
		if !hasSalary[employee.ID] {
			noSalary = append(noSalary, employee.FirstName+" "+employee.LastName)
		}
		if employee.BankID == nil || *employee.BankID == 0 {
			noBank = append(noBank, employee.FirstName+" "+employee.LastName)
		}
	}

	if len(noSalary) > 0 {
		issues = append(issues, PreCheckIssue{
			Type:      IssueNoSalary,
			Message:   "These employees have no salary structure and will be skipped",
			Employees: noSalary,
		})
	}
	if len(noBank) > 0 {
		issues = append(issues, PreCheckIssue{
			Type:      IssueNoBank,
			Message:   "These employees have no bank account assigned",
			Employees: noBank,
		})
	}
	return issues, nil
}

// CycleTotals is one cycle's totals, named for the comparison view.
type CycleTotals struct {
	CycleID        uint
	CycleName      string
	EmployeeCount  int
	TotalEarning   float64
	TotalDeduction float64
	SsoAmount      float64
	TaxAmount      float64
	NetPay         float64
}

// CycleDiff compares a cycle against the company's previous closed one.
type CycleDiff struct {
	Current     CycleTotals
	Previous    *CycleTotals
	HasPrevious bool
}

// totalsFor reads one cycle's aggregate and labels it.
func (s *PayrollRunService) totalsFor(cycle models.PayrollCycle) (CycleTotals, error) {
	aggregate, err := s.repo.AggregateCycle(cycle.ID)
	if err != nil {
		return CycleTotals{}, err
	}
	return CycleTotals{
		CycleID:        cycle.ID,
		CycleName:      cycle.CycleName,
		EmployeeCount:  aggregate.EmployeeCount,
		TotalEarning:   aggregate.TotalEarning,
		TotalDeduction: aggregate.TotalDeduction,
		SsoAmount:      aggregate.SsoAmount,
		TaxAmount:      aggregate.TaxAmount,
		NetPay:         aggregate.NetPay,
	}, nil
}

// Diff totals a cycle and, when there is one, the company's most recent closed cycle ending before it, so an operator can sanity-check a run against the last one.
func (s *PayrollRunService) Diff(scope repository.AccessScope, cycleID uint) (CycleDiff, error) {
	cycle, err := s.repo.FindCycleScoped(scope, cycleID)
	if err != nil {
		return CycleDiff{}, err
	}

	current, err := s.totalsFor(cycle)
	if err != nil {
		return CycleDiff{}, err
	}

	previousCycle, found, err := s.repo.FindPreviousClosedCycle(cycle.CompaniesID, cycle.StartDate)
	if err != nil {
		return CycleDiff{}, err
	}
	if !found {
		return CycleDiff{Current: current}, nil
	}

	previous, err := s.totalsFor(previousCycle)
	if err != nil {
		return CycleDiff{}, err
	}
	return CycleDiff{Current: current, Previous: &previous, HasPrevious: true}, nil
}

// Transactions returns a cycle's transactions for the summary table.
func (s *PayrollRunService) Transactions(scope repository.AccessScope, cycleID uint) (models.PayrollCycle, []models.PayrollTransaction, error) {
	cycle, err := s.repo.FindCycleScoped(scope, cycleID)
	if err != nil {
		return models.PayrollCycle{}, nil, ErrPayrollCycleNotFoundForList
	}
	transactions, err := s.repo.ListTransactionsForCycle(cycle.ID)
	if err != nil {
		return models.PayrollCycle{}, nil, err
	}
	return cycle, transactions, nil
}

// Slip returns one payslip.
func (s *PayrollRunService) Slip(scope repository.AccessScope, transactionID uint) (models.PayrollCycle, models.PayrollTransaction, models.Companies, error) {
	transaction, err := s.repo.FindTransactionDetail(transactionID)
	if err != nil {
		return models.PayrollCycle{}, models.PayrollTransaction{}, models.Companies{}, err
	}

	cycle, err := s.repo.FindCycleScoped(scope, transaction.CycleID)
	if err != nil {
		return models.PayrollCycle{}, models.PayrollTransaction{}, models.Companies{}, ErrPayrollSlipDenied
	}
	if !cycle.IsClosed {
		return models.PayrollCycle{}, models.PayrollTransaction{}, models.Companies{}, ErrPayrollSlipCycleOpen
	}

	company, err := s.repo.FindCompany(cycle.CompaniesID)
	if err != nil {
		return models.PayrollCycle{}, models.PayrollTransaction{}, models.Companies{}, err
	}
	return cycle, transaction, company, nil
}

// MySlip is one row of an employee's own payslip list.
type MySlip struct {
	Transaction models.PayrollTransaction
	CycleName   string
	StartDate   string
	EndDate     string
}

// MySlips returns the caller's own payslips from closed cycles.
func (s *PayrollRunService) MySlips(employeeID uint) ([]MySlip, error) {
	transactions, err := s.repo.ListClosedCycleTransactionsForEmployee(employeeID)
	if err != nil {
		return nil, err
	}

	cycleIDs := make([]uint, 0, len(transactions))
	for _, transaction := range transactions {
		cycleIDs = append(cycleIDs, transaction.CycleID)
	}
	cycles, err := s.repo.ListCyclesByIDs(cycleIDs)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint]models.PayrollCycle, len(cycles))
	for _, cycle := range cycles {
		byID[cycle.ID] = cycle
	}

	slips := make([]MySlip, 0, len(transactions))
	for _, transaction := range transactions {
		cycle := byID[transaction.CycleID]
		slip := MySlip{Transaction: transaction, CycleName: cycle.CycleName}
		if !cycle.StartDate.IsZero() {
			slip.StartDate = cycle.StartDate.Format("2006-01-02")
		}
		if !cycle.EndDate.IsZero() {
			slip.EndDate = cycle.EndDate.Format("2006-01-02")
		}
		slips = append(slips, slip)
	}
	return slips, nil
}

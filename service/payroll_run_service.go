package service

import (
	"errors"
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

// fullName is how a run names an employee in its skipped and pro-rated lists.
func fullName(employee models.Employees) string {
	return employee.FirstName + " " + employee.LastName
}

// scheduleWindow is a company's working hours, resolved once per run.
type scheduleWindow struct {
	startHour, startMin int
	endHour, endMin     int
}

// newScheduleWindow takes the company's configured hours, falling back to the default shift for anything left unset.
func newScheduleWindow(config models.AttendanceConfig) scheduleWindow {
	window := scheduleWindow{
		startHour: runDefaultStartHour, startMin: runDefaultStartMin,
		endHour: runDefaultEndHour, endMin: runDefaultEndMin,
	}
	if config.WorkStartTime != nil {
		start := timeutil.ToBangkok(*config.WorkStartTime)
		window.startHour, window.startMin = start.Hour(), start.Minute()
	}
	if config.WorkEndTime != nil {
		end := timeutil.ToBangkok(*config.WorkEndTime)
		window.endHour, window.endMin = end.Hour(), end.Minute()
	}
	return window
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

// runContext is everything a run resolves once for the whole company, then shares across every employee in the cycle.
type runContext struct {
	cycle            models.PayrollCycle
	payrollConfig    models.PayrollConfig
	attendanceConfig models.AttendanceConfig
	schedule         scheduleWindow
	holidays         map[string]bool
	dayOffWeekdays   map[int]bool
}

// runProration records how a salary that only took effect mid-cycle scaled the base pay.
type runProration struct {
	applied bool
	days    int
	total   int
}

// runPay is one employee's resolved basis for the cycle: which salary record applies, and the base it pays out.
type runPay struct {
	employee     models.Employees
	salaryRecord models.SalariesStructure
	baseSalary   float64
	proration    runProration
}

// runBases are the two running bases an item can move: the income tax is priced on, and the income social security is priced on.
type runBases struct {
	taxable float64
	sso     float64
}

// runLineItem is one earning or deduction before it becomes a stored detail line.
type runLineItem struct {
	name       string
	amount     float64
	typeID     *uint
	taxRelated bool
}

// runEarningInput is the shape both standing and cycle-only earnings collapse to before they are totalled.
type runEarningInput struct {
	name      string
	amount    float64
	typeID    uint
	isTaxable bool
	isSso     bool
}

// runDeductionInput is the shape both standing and cycle-only deductions collapse to before they are totalled.
type runDeductionInput struct {
	name            string
	amount          float64
	typeID          uint
	isTaxDeductible bool
	isSsoDeductible bool
}

// runEarnings totals the additional earnings and keeps the lines that explain them.
type runEarnings struct {
	total float64
	items []runLineItem
}

// runDeductions totals the additional deductions, how much of them the tax and SSO bases may be reduced by, and the lines that explain them.
type runDeductions struct {
	total         float64
	taxDeductible float64
	ssoDeductible float64
	items         []runLineItem
}

// runAttendance is what a cycle's paired time logs say: overtime past the shift end, days worked on a holiday or day off, and each working day's earliest check-in.
type runAttendance struct {
	overtimeMinutes float64
	extraDays       int
	byDay           map[string]*logDay
}

// runLeave is the approved leave overlapping a cycle: the unpaid days it costs, and every calendar day it excuses.
type runLeave struct {
	unpaidDays float64
	days       map[string]bool
}

// runAbsence is what the schedule was not met by: working days with no attendance at all, and one entry per day that was worked late. Lateness stays split by day because its deduction is capped per day, not across the cycle.
type runAbsence struct {
	lateMinutesPerDay []float64
	days              int
}

// runStatutory is the social security and income tax pair, priced on income actually earned.
type runStatutory struct {
	sso              float64
	tax              float64
	netTaxableIncome float64
}

// runFigures is everything one employee's calculation produced, before it is split into a stored row and its detail lines.
type runFigures struct {
	overtime       float64
	extra          float64
	unpaidLeave    float64
	late           float64
	absent         float64
	totalEarning   float64
	totalDeduction float64
	netPay         float64
	statutory      runStatutory
}

// runOutcome is what one employee contributed to the run's tally.
type runOutcome struct {
	skipped  bool
	prorated *ProratedEmployee
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
	run, err := s.newRunContext(cycle)
	if err != nil {
		return RunResult{}, err
	}
	employees, err := s.repo.ListActiveEmployees(cycle.CompaniesID)
	if err != nil {
		return RunResult{}, err
	}

	result := RunResult{}
	var lastErr error
	for _, employee := range employees {
		outcome, err := s.snapshotEmployee(run, employee)
		if outcome.prorated != nil {
			result.Prorated = append(result.Prorated, *outcome.prorated)
		}
		if outcome.skipped {
			result.SkippedCount++
			result.SkippedNames = append(result.SkippedNames, fullName(employee))
		}
		if err != nil {
			lastErr = err
			continue
		}
		if !outcome.skipped {
			result.ProcessedCount++
		}
	}

	return result, lastErr
}

// newRunContext resolves the company-wide inputs a cycle's run shares across every employee.
func (s *PayrollRunService) newRunContext(cycle models.PayrollCycle) (runContext, error) {
	companiesID := cycle.CompaniesID

	payrollConfig, err := s.repo.FindLatestPayrollConfig(companiesID)
	if err != nil {
		return runContext{}, err
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
		return runContext{}, err
	}
	holidaySet := make(map[string]bool, len(holidays))
	for _, holiday := range holidays {
		holidaySet[dateKey(holiday.Date)] = true
	}

	dayOffs, err := s.repo.ListDayOffs(companiesID)
	if err != nil {
		return runContext{}, err
	}
	dayOffWeekdays := make(map[int]bool, len(dayOffs))
	for _, dayOff := range dayOffs {
		dayOffWeekdays[dayOff.Weekday] = true
	}

	return runContext{
		cycle:            cycle,
		payrollConfig:    payrollConfig,
		attendanceConfig: attendanceConfig,
		schedule:         newScheduleWindow(attendanceConfig),
		holidays:         holidaySet,
		dayOffWeekdays:   dayOffWeekdays,
	}, nil
}

// isNonWorking reports whether a calendar day is a company holiday or a configured weekly day off.
func (run runContext) isNonWorking(day time.Time, key string) bool {
	return run.holidays[key] || run.dayOffWeekdays[int(day.Weekday())]
}

// snapshotEmployee computes and stores one employee's figures, or clears a stale transaction when no salary record applies.
func (s *PayrollRunService) snapshotEmployee(run runContext, employee models.Employees) (runOutcome, error) {
	pay, found, err := s.resolvePay(run.cycle, employee)
	if err != nil {
		// A salary that could not be read is a failure, not an employee to skip: skipping would drop them from the run and clear the transaction an earlier run wrote.
		return runOutcome{}, err
	}
	if !found {
		// Clear a transaction an earlier run wrote when this employee did have a usable salary record, so a stale figure does not linger.
		return runOutcome{skipped: true}, s.repo.DeleteTransactionForEmployee(run.cycle.ID, employee.ID)
	}

	outcome := runOutcome{}
	if pay.proration.applied {
		outcome.prorated = &ProratedEmployee{Name: fullName(employee), Days: pay.proration.days, Total: pay.proration.total}
	}

	snapshot, details, err := s.computeEmployee(run, pay)
	if err != nil {
		return outcome, err
	}
	return outcome, s.repo.SaveSnapshot(run.cycle.ID, employee.ID, snapshot, details)
}

// resolvePay picks the salary a cycle pays on and pro-rates the base by calendar days when that salary only took effect mid-cycle.
func (s *PayrollRunService) resolvePay(cycle models.PayrollCycle, employee models.Employees) (runPay, bool, error) {
	record, isProrated, found, err := s.salaryForCycle(employee.ID, cycle)
	if err != nil {
		return runPay{}, false, err
	}
	if !found {
		return runPay{}, false, nil
	}

	pay := runPay{employee: employee, salaryRecord: record, baseSalary: record.BaseSalaries}
	if !isProrated {
		return pay, true, nil
	}

	periodStart := dayOf(cycle.StartDate)
	periodEnd := dayOf(cycle.EndDate)
	effectiveStart := dayOf(record.EffectiveDate)
	total := int(periodEnd.Sub(periodStart).Hours()/24) + 1
	days := int(periodEnd.Sub(effectiveStart).Hours()/24) + 1

	pay.proration = runProration{applied: true, days: days, total: total}
	pay.baseSalary = round2(record.BaseSalaries * float64(days) / float64(total))
	return pay, true, nil
}

// salaryForCycle picks the salary a cycle should be paid on, reporting whether it only took effect mid-cycle. Only a genuinely missing record reads as "no salary"; a database failure is returned, so a run never silently drops an employee it could not read.
func (s *PayrollRunService) salaryForCycle(employeeID uint, cycle models.PayrollCycle) (models.SalariesStructure, bool, bool, error) {
	record, err := s.repo.FindActiveSalaryOnOrBefore(employeeID, cycle.StartDate)
	if err == nil {
		return record, false, true, nil
	}
	if !errors.Is(err, repository.ErrPayrollSalaryMissing) {
		return models.SalariesStructure{}, false, false, err
	}

	record, err = s.repo.FindEarliestActiveSalaryOnOrBefore(employeeID, cycle.EndDate)
	if err == nil {
		return record, true, true, nil
	}
	if !errors.Is(err, repository.ErrPayrollSalaryMissing) {
		return models.SalariesStructure{}, false, false, err
	}
	return models.SalariesStructure{}, false, false, nil
}

// computeEmployee turns one employee's inputs into the figures to store and the detail lines that explain them.
func (s *PayrollRunService) computeEmployee(run runContext, pay runPay) (repository.TransactionSnapshot, []models.PayrollTransactionDetail, error) {
	earningInputs, err := s.earningInputs(pay.employee.ID, run.cycle.ID)
	if err != nil {
		return repository.TransactionSnapshot{}, nil, err
	}
	attendance, err := s.attendanceFor(pay.employee.ID, run)
	if err != nil {
		return repository.TransactionSnapshot{}, nil, err
	}
	deductionInputs, err := s.deductionInputs(pay.employee.ID, run.cycle.ID)
	if err != nil {
		return repository.TransactionSnapshot{}, nil, err
	}
	leave, err := s.leaveFor(pay.employee.ID, run)
	if err != nil {
		return repository.TransactionSnapshot{}, nil, err
	}

	bases := runBases{taxable: pay.baseSalary, sso: pay.baseSalary}
	earnings, bases := totalEarnings(earningInputs, bases)

	figures := runFigures{
		overtime: run.overtimeAmount(attendance),
		extra:    run.extraAmount(attendance),
	}
	bases.taxable += figures.overtime
	bases.sso += figures.overtime
	bases.taxable += figures.extra
	bases.sso += figures.extra

	deductions := totalDeductions(deductionInputs)
	figures.unpaidLeave = round2(leave.unpaidDays * (pay.baseSalary / unpaidLeaveDivisor))

	absence := run.absenceFor(attendance, leave)
	figures.late = run.lateDeduction(absence)
	figures.absent = run.absentDeduction(absence)

	// Thai law prices tax and social security on income actually earned, so time not worked comes off the base first.
	bases.taxable = math.Max(0, bases.taxable-figures.late-figures.absent)
	bases.sso = math.Max(0, bases.sso-deductions.ssoDeductible-figures.late-figures.absent)

	figures.statutory = run.statutoryFor(bases, deductions.taxDeductible)
	figures.totalEarning = round2(pay.baseSalary + earnings.total + figures.overtime + figures.extra)
	figures.totalDeduction = round2(figures.unpaidLeave + deductions.total + figures.late + figures.absent)
	figures.netPay = round2(figures.totalEarning - figures.totalDeduction - figures.statutory.sso - figures.statutory.tax)

	return snapshotFor(pay, figures), detailsFor(pay, earnings, deductions, figures), nil
}

// earningInputs collects the employee's standing earnings and then this cycle's one-off ones, in the order the run totals them.
func (s *PayrollRunService) earningInputs(employeeID uint, cycleID uint) ([]runEarningInput, error) {
	standing, err := s.repo.ListEmployeeEarnings(employeeID)
	if err != nil {
		return nil, err
	}
	cycleOnly, err := s.repo.ListCycleEarnings(employeeID, cycleID)
	if err != nil {
		return nil, err
	}

	inputs := make([]runEarningInput, 0, len(standing)+len(cycleOnly))
	for _, item := range standing {
		inputs = append(inputs, runEarningInput{
			name: item.EarningType.Name, amount: item.Amount, typeID: item.EarningTypeID,
			isTaxable: item.EarningType.IsTaxable, isSso: item.EarningType.IsSso,
		})
	}
	for _, item := range cycleOnly {
		inputs = append(inputs, runEarningInput{
			name: item.EarningType.Name, amount: item.Amount, typeID: item.EarningTypeID,
			isTaxable: item.EarningType.IsTaxable, isSso: item.EarningType.IsSso,
		})
	}
	return inputs, nil
}

// deductionInputs collects the employee's standing deductions and then this cycle's one-off ones, in the order the run totals them.
func (s *PayrollRunService) deductionInputs(employeeID uint, cycleID uint) ([]runDeductionInput, error) {
	standing, err := s.repo.ListEmployeeDeductions(employeeID)
	if err != nil {
		return nil, err
	}
	cycleOnly, err := s.repo.ListCycleDeductions(employeeID, cycleID)
	if err != nil {
		return nil, err
	}

	inputs := make([]runDeductionInput, 0, len(standing)+len(cycleOnly))
	for _, item := range standing {
		inputs = append(inputs, runDeductionInput{
			name: item.DeductionType.Name, amount: item.Amount, typeID: item.DeductionTypeID,
			isTaxDeductible: item.DeductionType.IsTaxDeductible, isSsoDeductible: item.DeductionType.IsSsoDeductible,
		})
	}
	for _, item := range cycleOnly {
		inputs = append(inputs, runDeductionInput{
			name: item.DeductionType.Name, amount: item.Amount, typeID: item.DeductionTypeID,
			isTaxDeductible: item.DeductionType.IsTaxDeductible, isSsoDeductible: item.DeductionType.IsSsoDeductible,
		})
	}
	return inputs, nil
}

// totalEarnings folds the additional earnings onto the pay bases in listed order, so the arithmetic matches a single accumulation pass.
func totalEarnings(inputs []runEarningInput, bases runBases) (runEarnings, runBases) {
	result := runEarnings{}
	for _, input := range inputs {
		result.total += input.amount
		if input.isTaxable {
			bases.taxable += input.amount
		}
		if input.isSso {
			bases.sso += input.amount
		}
		typeID := input.typeID
		result.items = append(result.items, runLineItem{input.name, input.amount, &typeID, input.isTaxable})
	}
	return result, bases
}

// totalDeductions sums the additional deductions and how much of them the tax and SSO bases may be reduced by.
func totalDeductions(inputs []runDeductionInput) runDeductions {
	result := runDeductions{}
	for _, input := range inputs {
		result.total += input.amount
		if input.isTaxDeductible {
			result.taxDeductible += input.amount
		}
		if input.isSsoDeductible {
			result.ssoDeductible += input.amount
		}
		typeID := input.typeID
		result.items = append(result.items, runLineItem{input.name, input.amount, &typeID, input.isTaxDeductible})
	}
	return result
}

// attendanceFor collapses a cycle's paired time logs into one record per working day, and totals what falls outside the schedule.
func (s *PayrollRunService) attendanceFor(employeeID uint, run runContext) (runAttendance, error) {
	endOfCycle := dayOf(run.cycle.EndDate).AddDate(0, 0, 1)
	timeLogs, err := s.repo.ListPairedTimeLogs(employeeID, run.cycle.StartDate, endOfCycle)
	if err != nil {
		return runAttendance{}, err
	}

	result := runAttendance{byDay: make(map[string]*logDay)}
	for _, log := range timeLogs {
		if log.CheckIn == nil || log.CheckOut == nil {
			continue
		}
		day := timeutil.DayStart(*log.CheckIn)
		key := dateKey(day)

		// Working a holiday or a configured day off earns the extra-day rate; those days are not measured for overtime or lateness.
		if run.isNonWorking(day, key) {
			result.extraDays++
			continue
		}

		record := result.byDay[key]
		if record == nil {
			record = &logDay{earliestCheckIn: *log.CheckIn}
			result.byDay[key] = record
		}
		record.hasCheckPair = true
		if log.CheckIn.Before(record.earliestCheckIn) {
			record.earliestCheckIn = *log.CheckIn
		}

		if scheduledEnd := run.schedule.scheduledEnd(day); log.CheckOut.After(scheduledEnd) {
			result.overtimeMinutes += log.CheckOut.Sub(scheduledEnd).Minutes()
		}
	}
	return result, nil
}

// leaveFor reads the approved leave overlapping a cycle: what it costs in unpaid days, and which days it excuses from the absence count.
func (s *PayrollRunService) leaveFor(employeeID uint, run runContext) (runLeave, error) {
	approved, err := s.repo.ListApprovedLeavesInRange(employeeID, run.cycle.StartDate, run.cycle.EndDate)
	if err != nil {
		return runLeave{}, err
	}

	result := runLeave{days: make(map[string]bool)}
	for _, item := range approved {
		if item.LeaveType == models.LeaveTypeUnpaid {
			result.unpaidDays += item.TotalDays
		}
		for day := dayOf(item.StartDate); !day.After(dayOf(item.EndDate)); day = day.AddDate(0, 0, 1) {
			result.days[dateKey(day)] = true
		}
	}
	return result, nil
}

// absenceFor walks the cycle day by day, counting the working days with no check pair and the minutes lost on the days that had one.
func (run runContext) absenceFor(attendance runAttendance, leave runLeave) runAbsence {
	result := runAbsence{}
	for day := dayOf(run.cycle.StartDate); !day.After(dayOf(run.cycle.EndDate)); day = day.AddDate(0, 0, 1) {
		key := dateKey(day)
		if run.isNonWorking(day, key) || leave.days[key] {
			continue
		}
		record := attendance.byDay[key]
		if record == nil || !record.hasCheckPair {
			result.days++
			continue
		}
		if scheduledStart := run.schedule.scheduledStart(day); record.earliestCheckIn.After(scheduledStart) {
			result.lateMinutesPerDay = append(result.lateMinutesPerDay, record.earliestCheckIn.Sub(scheduledStart).Minutes())
		}
	}
	return result
}

// overtimeAmount prices the minutes worked past the shift end, when the company pays for them.
func (run runContext) overtimeAmount(attendance runAttendance) float64 {
	if !run.attendanceConfig.IsOvertimeEarningsEnabled {
		return 0
	}
	return round2(attendance.overtimeMinutes * run.attendanceConfig.OvertimeRatePerMinute)
}

// extraAmount prices the days worked on a holiday or a configured day off, when the company pays for them.
func (run runContext) extraAmount(attendance runAttendance) float64 {
	if !run.attendanceConfig.IsExtraEarningsEnabled {
		return 0
	}
	return round2(float64(attendance.extraDays) * run.attendanceConfig.ExtraRatePerDay)
}

// lateDeduction prices the minutes lost to lateness, capping each day at the absence rate so a day of lateness can never cost more than a day of absence. The cap belongs to the day, not the cycle: capping the total instead would let a month of daily lateness cost no more than a single late day.
func (run runContext) lateDeduction(absence runAbsence) float64 {
	if !run.attendanceConfig.IsLateDeductionEnabled {
		return 0
	}

	total := 0.0
	for _, minutes := range absence.lateMinutesPerDay {
		amount := round2(minutes * run.attendanceConfig.LateRatePerMinute)
		if amount > run.attendanceConfig.AbsentRatePerDay {
			amount = run.attendanceConfig.AbsentRatePerDay
		}
		total += amount
	}
	return round2(total)
}

// absentDeduction prices the working days with no attendance at all.
func (run runContext) absentDeduction(absence runAbsence) float64 {
	if !run.attendanceConfig.IsAbsentDeductionEnabled {
		return 0
	}
	return round2(float64(absence.days) * run.attendanceConfig.AbsentRatePerDay)
}

// statutoryFor prices social security within its configured floor and ceiling, then taxes what is left after the employee's own contribution comes off.
func (run runContext) statutoryFor(bases runBases, taxDeductible float64) runStatutory {
	sso := 0.0
	if bases.sso >= run.payrollConfig.SsoMinSalary {
		computed := bases.sso * run.payrollConfig.SsoRate / 100
		if computed > run.payrollConfig.SsoMaxAmount {
			computed = run.payrollConfig.SsoMaxAmount
		}
		sso = round2(computed)
	}

	// The employee's own social security contribution is deductible from taxable income, on top of whatever deduction items are tax-deductible.
	return runStatutory{
		sso:              sso,
		netTaxableIncome: round2(math.Max(0, bases.taxable-(taxDeductible+sso))),
		tax:              MonthlyTax(run.payrollConfig.TaxCalculation, bases.taxable, taxDeductible+sso),
	}
}

// snapshotFor is the row a run stores for one employee, carrying the position, department and bank details as they stand at calculation time.
func snapshotFor(pay runPay, figures runFigures) repository.TransactionSnapshot {
	snapshot := repository.TransactionSnapshot{
		SalariesStructureID: pay.salaryRecord.ID,
		SalariesAtTime:      pay.baseSalary,
		TotalEarning:        figures.totalEarning,
		TotalDeduction:      figures.totalDeduction,
		SsoAmount:           figures.statutory.sso,
		TaxableIncome:       figures.statutory.netTaxableIncome,
		TaxAmount:           figures.statutory.tax,
		NetPay:              figures.netPay,
		PositionName:        pay.employee.Position.PosName,
		DepartmentName:      pay.employee.Department.DeptName,
		BankAccountName:     pay.employee.AccountName,
		BankAccountNo:       pay.employee.AccountNo,
	}
	if pay.employee.EmploymentType != nil {
		snapshot.EmploymentTypeName = pay.employee.EmploymentType.Name
	}
	if pay.employee.Bank != nil {
		snapshot.BankName = pay.employee.Bank.BankName
	}
	return snapshot
}

// detailsFor lays out the lines that explain a stored transaction, in the order a payslip reads them.
func detailsFor(pay runPay, earnings runEarnings, deductions runDeductions, figures runFigures) []models.PayrollTransactionDetail {
	baseSalaryLabel := "Base Salary"
	if pay.proration.applied {
		baseSalaryLabel = fmt.Sprintf("Base Salary (Prorated %d/%d days)", pay.proration.days, pay.proration.total)
	}

	details := []models.PayrollTransactionDetail{
		{ItemName: baseSalaryLabel, Amount: pay.baseSalary, Type: "earning", IsTaxRelevant: true},
	}
	for _, item := range earnings.items {
		details = append(details, models.PayrollTransactionDetail{
			ItemName: item.name, Amount: item.amount, Type: "earning",
			EarningTypeID: item.typeID, IsTaxRelevant: item.taxRelated,
		})
	}
	if figures.overtime > 0 {
		details = append(details, models.PayrollTransactionDetail{ItemName: "Overtime", Amount: figures.overtime, Type: "earning", IsTaxRelevant: true})
	}
	if figures.extra > 0 {
		details = append(details, models.PayrollTransactionDetail{ItemName: "Extra Day", Amount: figures.extra, Type: "earning", IsTaxRelevant: true})
	}
	for _, item := range deductions.items {
		details = append(details, models.PayrollTransactionDetail{
			ItemName: item.name, Amount: item.amount, Type: "deduction",
			DeductionTypeID: item.typeID, IsTaxRelevant: item.taxRelated,
		})
	}
	if figures.unpaidLeave > 0 {
		details = append(details, models.PayrollTransactionDetail{ItemName: "Unpaid Leave", Amount: figures.unpaidLeave, Type: "deduction", IsTaxRelevant: false})
	}
	if figures.late > 0 {
		details = append(details, models.PayrollTransactionDetail{ItemName: "Late Amount", Amount: figures.late, Type: "deduction", IsTaxRelevant: true})
	}
	if figures.absent > 0 {
		details = append(details, models.PayrollTransactionDetail{ItemName: "Absent Amount", Amount: figures.absent, Type: "deduction", IsTaxRelevant: true})
	}
	if figures.statutory.sso > 0 {
		details = append(details, models.PayrollTransactionDetail{ItemName: "Social Security (SSO)", Amount: figures.statutory.sso, Type: "deduction", IsTaxRelevant: true})
	}
	if figures.statutory.tax > 0 {
		details = append(details, models.PayrollTransactionDetail{ItemName: "Income Tax", Amount: figures.statutory.tax, Type: "deduction", IsTaxRelevant: true})
	}
	return details
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
			noSalary = append(noSalary, fullName(employee))
		}
		if employee.BankID == nil || *employee.BankID == 0 {
			noBank = append(noBank, fullName(employee))
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

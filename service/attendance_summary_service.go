package service

import (
	"math"
	"time"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/timeutil"
)

// Summary range failures.
var (
	ErrSummaryFromInvalid  = apperr.Validation("Invalid 'from' date. Use YYYY-MM-DD")
	ErrSummaryToInvalid    = apperr.Validation("Invalid 'to' date. Use YYYY-MM-DD")
	ErrSummaryToBeforeFrom = apperr.Validation("'to' date must be on or after 'from' date")
)

// DeductionSummaryRow is one employee's late and absent totals for the window.
type DeductionSummaryRow struct {
	EmployeesID   uint
	EmployeesCode string
	FullName      string
	CompanyName   string
	TotalLateMin  float64
	TotalAbsent   int
	LateAmount    float64
	AbsentAmount  float64
	TotalAmount   float64
}

// EarningSummaryRow is one employee's overtime and extra-day totals for the window.
type EarningSummaryRow struct {
	EmployeesID          uint
	EmployeesCode        string
	FullName             string
	CompanyName          string
	TotalOvertimeMinutes float64
	TotalExtraDays       int
	OvertimeAmount       float64
	ExtraAmount          float64
	TotalAmount          float64
}

// SummaryRange is the window a summary covers, as the response reports it.
type SummaryRange struct {
	From time.Time
	To   time.Time
}

// From and To render the window the way the response has always rendered it.
func (r SummaryRange) FromLabel() string { return r.From.Format("2006-01-02") }
func (r SummaryRange) ToLabel() string {
	return time.Date(r.To.Year(), r.To.Month(), r.To.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

// AttendanceSummaryService owns the attendance arithmetic: which days count as Every calendar comparison goes through timeutil, because a time.Time read back
type AttendanceSummaryService struct {
	repo repository.AttendanceSummaryRepository
}

// NewAttendanceSummaryService wires the service to a repository implementation.
func NewAttendanceSummaryService(repo repository.AttendanceSummaryRepository) *AttendanceSummaryService {
	return &AttendanceSummaryService{repo: repo}
}

// ParseRange reads the from/to query values, defaulting to the current month so far.
func ParseSummaryRange(fromValue string, toValue string) (SummaryRange, error) {
	var window SummaryRange

	if fromValue != "" {
		parsed, err := timeutil.ParseDayStart(fromValue)
		if err != nil {
			return SummaryRange{}, ErrSummaryFromInvalid
		}
		window.From = parsed
	} else {
		now := time.Now()
		window.From = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	}

	if toValue != "" {
		parsed, err := timeutil.ParseDayStart(toValue)
		if err != nil {
			return SummaryRange{}, ErrSummaryToInvalid
		}
		window.To = parsed.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	} else {
		window.To = time.Now()
	}

	if window.To.Before(window.From) {
		return SummaryRange{}, ErrSummaryToBeforeFrom
	}
	return window, nil
}

// summaryData is everything both summaries need, loaded once. Logs and leave stay raw per employee so the payroll run's own functions can read them.
type summaryData struct {
	employees []models.Employees
	configs   map[uint]models.AttendanceConfig
	holidays  map[uint]map[string]bool
	dayOffs   map[uint]map[int]bool
	leaves    map[uint][]models.LeaveRequest
	logs      map[uint][]models.TimeLog
}

// runContextFor is the payroll run's view of one employee's company over the summary window, so both pages price attendance with the run's functions (BR-112).
func (data summaryData) runContextFor(employee models.Employees, window SummaryRange) runContext {
	configuration := data.configs[employee.CompaniesID]
	return runContext{
		cycle:            models.PayrollCycle{StartDate: window.From, EndDate: window.To},
		attendanceConfig: configuration,
		schedule:         newScheduleWindow(configuration),
		holidays:         data.holidays[employee.CompaniesID],
		dayOffWeekdays:   data.dayOffs[employee.CompaniesID],
	}
}

// load reads the whole window.
func (s *AttendanceSummaryService) load(scope repository.AccessScope, window SummaryRange, withLeave bool) (summaryData, error) {
	data := summaryData{
		configs:  map[uint]models.AttendanceConfig{},
		holidays: map[uint]map[string]bool{},
		dayOffs:  map[uint]map[int]bool{},
		leaves:   map[uint][]models.LeaveRequest{},
		logs:     map[uint][]models.TimeLog{},
	}

	employees, err := s.repo.ListEmployeesScoped(scope)
	if err != nil {
		return summaryData{}, err
	}
	data.employees = employees
	if len(employees) == 0 {
		return data, nil
	}

	employeeIDs := make([]uint, 0, len(employees))
	companySet := map[uint]bool{}
	for _, employee := range employees {
		employeeIDs = append(employeeIDs, employee.ID)
		companySet[employee.CompaniesID] = true
	}
	companyIDs := make([]uint, 0, len(companySet))
	for id := range companySet {
		companyIDs = append(companyIDs, id)
	}

	configs, err := s.repo.ListConfigsForCompanies(companyIDs)
	if err != nil {
		return summaryData{}, err
	}
	for _, configuration := range configs {
		data.configs[configuration.CompaniesID] = configuration
	}

	holidays, err := s.repo.ListHolidaysBetween(companyIDs, window.From, window.To)
	if err != nil {
		return summaryData{}, err
	}
	for _, holiday := range holidays {
		if data.holidays[holiday.CompaniesID] == nil {
			data.holidays[holiday.CompaniesID] = map[string]bool{}
		}
		data.holidays[holiday.CompaniesID][timeutil.DateKey(holiday.Date)] = true
	}

	dayOffs, err := s.repo.ListDayOffsForCompanies(companyIDs)
	if err != nil {
		return summaryData{}, err
	}
	for _, dayOff := range dayOffs {
		if data.dayOffs[dayOff.CompaniesID] == nil {
			data.dayOffs[dayOff.CompaniesID] = map[int]bool{}
		}
		data.dayOffs[dayOff.CompaniesID][dayOff.Weekday] = true
	}

	if withLeave {
		leaves, err := s.repo.ListApprovedLeavesOverlapping(employeeIDs, window.From, window.To)
		if err != nil {
			return summaryData{}, err
		}
		for _, leave := range leaves {
			data.leaves[leave.EmployeesID] = append(data.leaves[leave.EmployeesID], leave)
		}
	}

	logs, err := s.repo.ListTimeLogsBetween(employeeIDs, window.From, window.To)
	if err != nil {
		return summaryData{}, err
	}
	for _, log := range logs {
		data.logs[log.EmployeesID] = append(data.logs[log.EmployeesID], log)
	}

	return data, nil
}

// round2 keeps the two-decimal rounding the response has always used.
func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

// Deductions totals late minutes and absent days per employee, and prices them with the payroll run's own calculation (BR-112).
func (s *AttendanceSummaryService) Deductions(scope repository.AccessScope, window SummaryRange) ([]DeductionSummaryRow, error) {
	data, err := s.load(scope, window, true)
	if err != nil {
		return nil, err
	}

	rows := make([]DeductionSummaryRow, 0, len(data.employees))
	for _, employee := range data.employees {
		configuration := data.configs[employee.CompaniesID]
		run := data.runContextFor(employee, window)
		absence := run.absenceFor(attendanceFor(data.logs[employee.ID], run), leaveFor(data.leaves[employee.ID]))

		lateMinutes := 0.0
		if configuration.IsLateDeductionEnabled {
			for _, minutes := range absence.lateMinutesPerDay {
				lateMinutes += minutes
			}
		}
		absentDays := 0
		if configuration.IsAbsentDeductionEnabled {
			absentDays = absence.days
		}
		lateAmount := run.lateDeduction(absence)
		absentAmount := run.absentDeduction(absence)

		rows = append(rows, DeductionSummaryRow{
			EmployeesID:   employee.ID,
			EmployeesCode: employee.EmployeesCode,
			FullName:      employee.FirstName + " " + employee.LastName,
			CompanyName:   employee.Companies.Name,
			TotalLateMin:  round2(lateMinutes),
			TotalAbsent:   absentDays,
			LateAmount:    lateAmount,
			AbsentAmount:  absentAmount,
			TotalAmount:   round2(lateAmount + absentAmount),
		})
	}
	return rows, nil
}

// Earnings totals overtime minutes and extra days worked per employee, and prices them with the payroll run's own calculation (BR-112).
func (s *AttendanceSummaryService) Earnings(scope repository.AccessScope, window SummaryRange) ([]EarningSummaryRow, error) {
	data, err := s.load(scope, window, false)
	if err != nil {
		return nil, err
	}

	rows := make([]EarningSummaryRow, 0, len(data.employees))
	for _, employee := range data.employees {
		configuration := data.configs[employee.CompaniesID]
		run := data.runContextFor(employee, window)
		attendance := attendanceFor(data.logs[employee.ID], run)

		overtimeMinutes := 0.0
		if configuration.IsOvertimeEarningsEnabled {
			overtimeMinutes = attendance.overtimeMinutes
		}
		extraDays := 0
		if configuration.IsExtraEarningsEnabled {
			extraDays = attendance.extraDays
		}
		overtimeAmount := run.overtimeAmount(attendance)
		extraAmount := run.extraAmount(attendance)

		rows = append(rows, EarningSummaryRow{
			EmployeesID:          employee.ID,
			EmployeesCode:        employee.EmployeesCode,
			FullName:             employee.FirstName + " " + employee.LastName,
			CompanyName:          employee.Companies.Name,
			TotalOvertimeMinutes: round2(overtimeMinutes),
			TotalExtraDays:       extraDays,
			OvertimeAmount:       overtimeAmount,
			ExtraAmount:          extraAmount,
			TotalAmount:          round2(overtimeAmount + extraAmount),
		})
	}
	return rows, nil
}

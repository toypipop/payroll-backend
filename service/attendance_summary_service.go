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
	defaultWorkEndHour   = 18
	defaultWorkEndMinute = 0
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
		parsed, err := time.Parse("2006-01-02", fromValue)
		if err != nil {
			return SummaryRange{}, ErrSummaryFromInvalid
		}
		window.From = parsed
	} else {
		now := time.Now()
		window.From = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	}

	if toValue != "" {
		parsed, err := time.Parse("2006-01-02", toValue)
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

// dayRecord collapses an employee's logs for one calendar day.
type dayRecord struct {
	hasCheckPair    bool
	earliestCheckIn *time.Time
	latestCheckOut  *time.Time
	hasLateStatus   bool
}

// summaryData is everything both summaries need, loaded once.
type summaryData struct {
	employees []models.Employees
	configs   map[uint]models.AttendanceConfig
	holidays  map[string]bool
	dayOffs   map[uint]map[int]bool
	leaveDays map[string]bool
	logsByDay map[string]*dayRecord
}

func companyDayKey(companiesID uint, day time.Time) string {
	return fmt.Sprintf("%d|%s", companiesID, timeutil.DateKey(day))
}

func employeeDayKey(employeeID uint, day time.Time) string {
	return fmt.Sprintf("%d|%s", employeeID, timeutil.DateKey(day))
}

// load reads the whole window.
func (s *AttendanceSummaryService) load(scope repository.AccessScope, window SummaryRange, withLeave bool) (summaryData, error) {
	data := summaryData{
		configs:   map[uint]models.AttendanceConfig{},
		holidays:  map[string]bool{},
		dayOffs:   map[uint]map[int]bool{},
		leaveDays: map[string]bool{},
		logsByDay: map[string]*dayRecord{},
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
		data.holidays[companyDayKey(holiday.CompaniesID, holiday.Date)] = true
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
			for day := timeutil.DayStart(leave.StartDate); !day.After(timeutil.DayStart(leave.EndDate)); day = day.AddDate(0, 0, 1) {
				data.leaveDays[employeeDayKey(leave.EmployeesID, day)] = true
			}
		}
	}

	logs, err := s.repo.ListTimeLogsBetween(employeeIDs, window.From, window.To)
	if err != nil {
		return summaryData{}, err
	}
	known := map[uint]bool{}
	for _, employee := range employees {
		known[employee.ID] = true
	}
	for _, log := range logs {
		if !known[log.EmployeesID] {
			continue
		}

		// A log with no check-in is placed by when it was created.
		logDate := log.CreatedAt
		if log.CheckIn != nil {
			logDate = *log.CheckIn
		}

		key := employeeDayKey(log.EmployeesID, logDate)
		record := data.logsByDay[key]
		if record == nil {
			record = &dayRecord{}
			data.logsByDay[key] = record
		}

		if log.CheckIn != nil && log.CheckOut != nil {
			record.hasCheckPair = true
			if record.earliestCheckIn == nil || log.CheckIn.Before(*record.earliestCheckIn) {
				checkIn := *log.CheckIn
				record.earliestCheckIn = &checkIn
			}
			if record.latestCheckOut == nil || log.CheckOut.After(*record.latestCheckOut) {
				checkOut := *log.CheckOut
				record.latestCheckOut = &checkOut
			}
		}
		if log.Status == models.StatusLate {
			record.hasLateStatus = true
		}
	}

	return data, nil
}

// round2 keeps the two-decimal rounding the response has always used.
func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

// Deductions totals late minutes and absent days per employee, and prices them.
func (s *AttendanceSummaryService) Deductions(scope repository.AccessScope, window SummaryRange) ([]DeductionSummaryRow, error) {
	data, err := s.load(scope, window, true)
	if err != nil {
		return nil, err
	}

	rows := make([]DeductionSummaryRow, 0, len(data.employees))
	for _, employee := range data.employees {
		configuration := data.configs[employee.CompaniesID]

		startHour, startMinute := defaultWorkStartHour, defaultWorkStartMinute
		if configuration.WorkStartTime != nil {
			scheduled := timeutil.ToBangkok(*configuration.WorkStartTime)
			startHour, startMinute = scheduled.Hour(), scheduled.Minute()
		}

		lateMinutes := 0.0
		absentDays := 0

		for day := timeutil.DayStart(window.From); !day.After(timeutil.DayStart(window.To)); day = day.AddDate(0, 0, 1) {
			isDayOff := data.dayOffs[employee.CompaniesID] != nil && data.dayOffs[employee.CompaniesID][int(day.Weekday())]
			if data.holidays[companyDayKey(employee.CompaniesID, day)] || isDayOff {
				continue
			}
			if data.leaveDays[employeeDayKey(employee.ID, day)] {
				continue
			}

			record := data.logsByDay[employeeDayKey(employee.ID, day)]
			if record == nil || !record.hasCheckPair {
				absentDays++
				continue
			}

			scheduledStart := time.Date(day.Year(), day.Month(), day.Day(), startHour, startMinute, 0, 0, timeutil.BangkokLocation)
			switch {
			case record.earliestCheckIn != nil && record.earliestCheckIn.After(scheduledStart):
				lateMinutes += record.earliestCheckIn.Sub(scheduledStart).Minutes()
			case record.hasLateStatus:
				// Marked late but the arithmetic says on time: count one minute.
				lateMinutes += 1.0
			}
		}

		if !configuration.IsLateDeductionEnabled {
			lateMinutes = 0
		}
		if !configuration.IsAbsentDeductionEnabled {
			absentDays = 0
		}

		lateAmount := 0.0
		if configuration.IsLateDeductionEnabled {
			lateAmount = round2(lateMinutes * configuration.LateRatePerMinute)
			// A day of lateness can never cost more than a day of absence.
			if lateAmount > configuration.AbsentRatePerDay {
				lateAmount = configuration.AbsentRatePerDay
			}
		}

		absentAmount := 0.0
		if configuration.IsAbsentDeductionEnabled {
			absentAmount = round2(float64(absentDays) * configuration.AbsentRatePerDay)
		}

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

// Earnings totals overtime minutes and extra days worked per employee, and prices them.
func (s *AttendanceSummaryService) Earnings(scope repository.AccessScope, window SummaryRange) ([]EarningSummaryRow, error) {
	data, err := s.load(scope, window, false)
	if err != nil {
		return nil, err
	}

	rows := make([]EarningSummaryRow, 0, len(data.employees))
	for _, employee := range data.employees {
		configuration := data.configs[employee.CompaniesID]

		startHour, startMinute := defaultWorkStartHour, defaultWorkStartMinute
		endHour, endMinute := defaultWorkEndHour, defaultWorkEndMinute
		if configuration.WorkStartTime != nil {
			scheduled := timeutil.ToBangkok(*configuration.WorkStartTime)
			startHour, startMinute = scheduled.Hour(), scheduled.Minute()
		}
		if configuration.WorkEndTime != nil {
			scheduled := timeutil.ToBangkok(*configuration.WorkEndTime)
			endHour, endMinute = scheduled.Hour(), scheduled.Minute()
		}

		overtimeMinutes := 0.0
		extraDates := map[string]bool{}

		for day := timeutil.DayStart(window.From); !day.After(timeutil.DayStart(window.To)); day = day.AddDate(0, 0, 1) {
			record := data.logsByDay[employeeDayKey(employee.ID, day)]
			if record == nil || !record.hasCheckPair || record.latestCheckOut == nil {
				continue
			}

			isDayOff := data.dayOffs[employee.CompaniesID] != nil && data.dayOffs[employee.CompaniesID][int(day.Weekday())]
			if data.holidays[companyDayKey(employee.CompaniesID, day)] || isDayOff {
				// Working a holiday or a day off earns the extra-day rate, not overtime.
				extraDates[day.Format("2006-01-02")] = true
				continue
			}

			scheduledEnd := time.Date(day.Year(), day.Month(), day.Day(), endHour, endMinute, 0, 0, timeutil.BangkokLocation)
			// An end at or before the start means the shift runs past midnight.
			if endHour < startHour || (endHour == startHour && endMinute <= startMinute) {
				scheduledEnd = scheduledEnd.AddDate(0, 0, 1)
			}

			if record.latestCheckOut.After(scheduledEnd) {
				overtimeMinutes += record.latestCheckOut.Sub(scheduledEnd).Minutes()
			}
		}

		if !configuration.IsOvertimeEarningsEnabled {
			overtimeMinutes = 0
		}
		extraDays := len(extraDates)
		if !configuration.IsExtraEarningsEnabled {
			extraDays = 0
		}

		overtimeAmount := 0.0
		if configuration.IsOvertimeEarningsEnabled {
			overtimeAmount = round2(overtimeMinutes * configuration.OvertimeRatePerMinute)
		}

		extraAmount := 0.0
		if configuration.IsExtraEarningsEnabled {
			extraAmount = round2(float64(extraDays) * configuration.ExtraRatePerDay)
		}

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

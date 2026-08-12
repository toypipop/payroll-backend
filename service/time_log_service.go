package service

import (
	"time"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/timeutil"
)

// Layouts the time-log endpoints accept.
const (
	timeLogDateLayout  = "2006-01-02"
	timeLogClockLayout = "15:04"
)

// Fallback schedule used when a company has no attendance configuration.
const (
	defaultWorkStartHour   = 9
	defaultWorkStartMinute = 0
)

// bulkRangeLimit caps how long a bulk generation range may be.
const bulkRangeLimit = 366 * 24 * time.Hour

// Time-log rule failures.
var (
	ErrTimeLogEmployeeRequired     = apperr.Validation("Employee is required")
	ErrTimeLogWorkLocationRequired = apperr.Validation("Work location is required")
	ErrTimeLogCheckOutBefore       = apperr.Validation("Check-out time cannot be earlier than check-in time")
	ErrTimeLogEmployeeCompany      = apperr.Forbidden("Employee does not belong to selected company or access denied")
	ErrTimeLogStartDateInvalid     = apperr.Validation("Invalid start_date format. Expected YYYY-MM-DD")
	ErrTimeLogEndDateInvalid       = apperr.Validation("Invalid end_date format. Expected YYYY-MM-DD")
	ErrTimeLogEndBeforeStart       = apperr.Validation("end_date cannot be before start_date")
	ErrTimeLogRangeTooLong         = apperr.Validation("Date range cannot exceed 366 days")
	ErrTimeLogCheckInTimeInvalid   = apperr.Validation("Invalid check_in_time format. Expected HH:MM")
	ErrTimeLogCheckOutTimeInvalid  = apperr.Validation("Invalid check_out_time format. Expected HH:MM")
	ErrTimeLogWeekdayRequired      = apperr.Validation("At least one weekday must be selected")
)

// Re-exported so handlers import a single package.
var (
	ErrTimeLogNotFound     = repository.ErrTimeLogNotFound
	ErrTimeLogNotDeletable = repository.ErrTimeLogNotDeletable
)

// BulkTimeLogInput describes a run of generated logs for one employee.
type BulkTimeLogInput struct {
	EmployeesID    uint
	CompaniesID    uint
	WorkLocationID uint
	StartDate      string
	EndDate        string
	Weekdays       []int
	CheckInTime    string
	CheckOutTime   string
}

// BulkTimeLogResult reports what a bulk run created and what it skipped.
type BulkTimeLogResult struct {
	CreatedCount int
	SkippedDates []string
}

// TimeLogService owns the time-log rules: who a log may be filed against, and what
// attendance status a pair of timestamps amounts to.
type TimeLogService struct {
	repo      repository.TimeLogRepository
	employees repository.EmployeesRepository
}

// NewTimeLogService wires the service to repository implementations.
func NewTimeLogService(repo repository.TimeLogRepository, employees repository.EmployeesRepository) *TimeLogService {
	return &TimeLogService{repo: repo, employees: employees}
}

// List returns the logs the caller's scope covers, with relations loaded.
func (s *TimeLogService) List(scope repository.AccessScope) ([]models.TimeLog, error) {
	return s.repo.ListScoped(scope)
}

// resolveEmployee confirms the employee sits in the requested company and that the
// caller may act on that company, returning the employee to file the log against.
func (s *TimeLogService) resolveEmployee(scope repository.AccessScope, employeeID uint, companyID uint) (models.Employees, error) {
	if companyID == 0 || employeeID == 0 {
		return models.Employees{}, ErrTimeLogEmployeeCompany
	}
	if _, ok := scope.ResolveCompanyID(companyID); !ok {
		return models.Employees{}, ErrTimeLogEmployeeCompany
	}
	employee, err := s.employees.FindBriefScoped(scope, employeeID)
	if err != nil || employee.CompaniesID != companyID {
		return models.Employees{}, ErrTimeLogEmployeeCompany
	}
	return employee, nil
}

// DeriveStatus decides what a check-in and check-out pair amounts to. A missing
// timestamp on either side counts as absent; a company holiday or a configured day
// off outranks lateness; otherwise the check-in is compared against the company's
// scheduled start.
//
// Both timestamps are normalised to the business timezone before any date or
// weekday is read from them: a *time.Time read back from Postgres carries whatever
// session timezone the driver attached, so reading it directly can land on the wrong
// calendar day.
//
// Count failures are treated as "no holiday, no day off", which is how this
// behaved before the split.
func (s *TimeLogService) DeriveStatus(companiesID uint, checkIn *time.Time, checkOut *time.Time) models.AttendanceStatus {
	if checkIn == nil || checkOut == nil {
		return models.StatusAbsent
	}

	bangkokCheckIn := timeutil.ToBangkok(*checkIn)

	if holidays, err := s.repo.CountHolidaysOn(companiesID, bangkokCheckIn.Format(timeLogDateLayout)); err == nil && holidays > 0 {
		return models.StatusHoliday
	}
	if dayOffs, err := s.repo.CountDayOffsOn(companiesID, int(bangkokCheckIn.Weekday())); err == nil && dayOffs > 0 {
		return models.StatusHoliday
	}

	startHour, startMinute := defaultWorkStartHour, defaultWorkStartMinute
	if configuration, err := s.repo.FindAttendanceConfig(companiesID); err == nil && configuration.WorkStartTime != nil {
		scheduled := timeutil.ToBangkok(*configuration.WorkStartTime)
		startHour, startMinute = scheduled.Hour(), scheduled.Minute()
	}

	scheduledStart := time.Date(
		bangkokCheckIn.Year(), bangkokCheckIn.Month(), bangkokCheckIn.Day(),
		startHour, startMinute, 0, 0, timeutil.BangkokLocation,
	)
	if checkIn.After(scheduledStart) {
		return models.StatusLate
	}
	return models.StatusOnTime
}

// validateInput enforces the field rules shared by create and update.
func validateTimeLogInput(input models.TimeLog) error {
	if input.EmployeesID == 0 {
		return ErrTimeLogEmployeeRequired
	}
	if input.WorkLocationID == nil || *input.WorkLocationID == 0 {
		return ErrTimeLogWorkLocationRequired
	}
	return nil
}

// Create files a log against an employee, deriving its status.
func (s *TimeLogService) Create(scope repository.AccessScope, input models.TimeLog) (models.TimeLog, error) {
	if err := validateTimeLogInput(input); err != nil {
		return models.TimeLog{}, err
	}
	if input.CheckIn != nil && input.CheckOut != nil && input.CheckOut.Before(*input.CheckIn) {
		return models.TimeLog{}, ErrTimeLogCheckOutBefore
	}

	employee, err := s.resolveEmployee(scope, input.EmployeesID, input.CompaniesID)
	if err != nil {
		return models.TimeLog{}, err
	}

	// The company comes from the employee, never from the request.
	input.CompaniesID = employee.CompaniesID
	input.Status = s.DeriveStatus(input.CompaniesID, input.CheckIn, input.CheckOut)

	if err := s.repo.Create(&input); err != nil {
		return models.TimeLog{}, err
	}
	return input, nil
}

// Update rewrites a log the caller's scope covers, re-deriving its status.
func (s *TimeLogService) Update(scope repository.AccessScope, id uint, input models.TimeLog) (models.TimeLog, error) {
	log, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.TimeLog{}, err
	}
	if err := validateTimeLogInput(input); err != nil {
		return models.TimeLog{}, err
	}

	employee, err := s.resolveEmployee(scope, input.EmployeesID, input.CompaniesID)
	if err != nil {
		return models.TimeLog{}, err
	}

	log.EmployeesID = employee.ID
	log.CompaniesID = employee.CompaniesID
	log.CheckIn = input.CheckIn
	log.CheckOut = input.CheckOut
	log.WorkLocationID = input.WorkLocationID
	log.LocationIn = input.LocationIn
	log.Remark = input.Remark

	if log.CheckIn != nil && log.CheckOut != nil && log.CheckOut.Before(*log.CheckIn) {
		return models.TimeLog{}, ErrTimeLogCheckOutBefore
	}

	log.Status = s.DeriveStatus(log.CompaniesID, log.CheckIn, log.CheckOut)
	if err := s.repo.Save(&log); err != nil {
		return models.TimeLog{}, err
	}
	return log, nil
}

// Delete removes a log the caller's scope covers.
func (s *TimeLogService) Delete(scope repository.AccessScope, id uint) error {
	log, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return ErrTimeLogNotDeletable
	}
	return s.repo.Delete(&log)
}

// parseClock reads an HH:MM string into its hour and minute.
func parseClock(value string) (int, int, error) {
	parsed, err := time.Parse(timeLogClockLayout, value)
	if err != nil {
		return 0, 0, err
	}
	return parsed.Hour(), parsed.Minute(), nil
}

// BulkCreate creates one log per matching weekday in a date range, using the same
// time of day for each. A date that already has a log for the employee is skipped
// rather than duplicated.
//
// Logs are written one at a time and a failure returns immediately, leaving the
// logs created so far in place. That is the previous behaviour, preserved
// deliberately rather than wrapped in a transaction.
func (s *TimeLogService) BulkCreate(scope repository.AccessScope, input BulkTimeLogInput) (BulkTimeLogResult, error) {
	startDate, err := time.Parse(timeLogDateLayout, input.StartDate)
	if err != nil {
		return BulkTimeLogResult{}, ErrTimeLogStartDateInvalid
	}
	endDate, err := time.Parse(timeLogDateLayout, input.EndDate)
	if err != nil {
		return BulkTimeLogResult{}, ErrTimeLogEndDateInvalid
	}
	if endDate.Before(startDate) {
		return BulkTimeLogResult{}, ErrTimeLogEndBeforeStart
	}
	if endDate.Sub(startDate) > bulkRangeLimit {
		return BulkTimeLogResult{}, ErrTimeLogRangeTooLong
	}

	checkInHour, checkInMinute, err := parseClock(input.CheckInTime)
	if err != nil {
		return BulkTimeLogResult{}, ErrTimeLogCheckInTimeInvalid
	}
	checkOutHour, checkOutMinute, err := parseClock(input.CheckOutTime)
	if err != nil {
		return BulkTimeLogResult{}, ErrTimeLogCheckOutTimeInvalid
	}

	weekdays := make(map[int]bool, len(input.Weekdays))
	for _, weekday := range input.Weekdays {
		weekdays[weekday] = true
	}
	if len(weekdays) == 0 {
		return BulkTimeLogResult{}, ErrTimeLogWeekdayRequired
	}

	employee, err := s.resolveEmployee(scope, input.EmployeesID, input.CompaniesID)
	if err != nil {
		return BulkTimeLogResult{}, err
	}

	workLocationID := input.WorkLocationID
	result := BulkTimeLogResult{}

	for day := startDate; !day.After(endDate); day = day.AddDate(0, 0, 1) {
		if !weekdays[int(day.Weekday())] {
			continue
		}

		existing, err := s.repo.CountForEmployeeBetween(employee.ID, day, day.AddDate(0, 0, 1))
		if err != nil {
			return result, err
		}
		if existing > 0 {
			result.SkippedDates = append(result.SkippedDates, day.Format(timeLogDateLayout))
			continue
		}

		checkIn := time.Date(day.Year(), day.Month(), day.Day(), checkInHour, checkInMinute, 0, 0, timeutil.BangkokLocation)
		checkOut := time.Date(day.Year(), day.Month(), day.Day(), checkOutHour, checkOutMinute, 0, 0, timeutil.BangkokLocation)
		// An end at or before the start means the shift runs past midnight.
		if !checkOut.After(checkIn) {
			checkOut = checkOut.AddDate(0, 0, 1)
		}

		log := models.TimeLog{
			CompaniesID:    employee.CompaniesID,
			EmployeesID:    employee.ID,
			CheckIn:        &checkIn,
			CheckOut:       &checkOut,
			WorkLocationID: &workLocationID,
		}
		log.Status = s.DeriveStatus(log.CompaniesID, log.CheckIn, log.CheckOut)

		if err := s.repo.Create(&log); err != nil {
			return result, err
		}
		result.CreatedCount++
	}

	return result, nil
}

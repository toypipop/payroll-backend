package service

import (
	"errors"
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

// TimeLogInput accepts only editable fields, never persistence metadata or associations.
type TimeLogInput struct {
	CompaniesID    uint       `json:"companies_id"`
	EmployeesID    uint       `json:"employees_id"`
	CheckIn        *time.Time `json:"check_in"`
	CheckOut       *time.Time `json:"check_out"`
	WorkLocationID *uint      `json:"work_location_id"`
	LocationIn     string     `json:"location_in"`
	Remark         string     `json:"remark"`
}

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
	HolidayDates []string
}

// TimeLogService owns the time-log rules: who a log may be filed against, and what attendance status a pair of timestamps amounts to.
type TimeLogService struct {
	repo      repository.TimeLogRepository
	employees repository.EmployeesRepository
	locations repository.WorkLocationRepository
}

// NewTimeLogService wires the service to repository implementations.
func NewTimeLogService(repo repository.TimeLogRepository, employees repository.EmployeesRepository, locations repository.WorkLocationRepository) *TimeLogService {
	return &TimeLogService{repo: repo, employees: employees, locations: locations}
}

var ErrTimeLogWorkLocationCompany = apperr.Validation("Work location must belong to the time log company")

func (s *TimeLogService) validateWorkLocation(scope repository.AccessScope, companyID, locationID uint) error {
	if locationID == 0 {
		return ErrTimeLogWorkLocationRequired
	}
	if s.locations == nil {
		return ErrTimeLogWorkLocationCompany
	}
	location, err := s.locations.FindScoped(scope, locationID)
	if err != nil {
		return err
	}
	if location.CompaniesID != companyID {
		return ErrTimeLogWorkLocationCompany
	}
	return nil
}

// List returns the logs the caller's scope covers, with relations loaded.
func (s *TimeLogService) List(scope repository.AccessScope, q repository.PageQuery) (repository.Page[models.TimeLog], error) {
	return s.repo.ListScoped(scope, q)
}

// resolveEmployee confirms the employee sits in the requested company and that the caller may act on that company, returning the employee to file the log against.
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
	if err := requireActiveEmployee(employee); err != nil {
		return models.Employees{}, err
	}
	return employee, nil
}

// DeriveStatus decides what a check-in and check-out pair amounts to.
func (s *TimeLogService) DeriveStatus(companiesID uint, checkIn *time.Time, checkOut *time.Time) (models.AttendanceStatus, error) {
	if checkIn == nil || checkOut == nil {
		return models.StatusAbsent, nil
	}

	bangkokCheckIn := timeutil.ToBangkok(*checkIn)

	holidays, err := s.repo.CountHolidaysOn(companiesID, bangkokCheckIn.Format(timeLogDateLayout))
	if err != nil {
		return "", err
	}
	if holidays > 0 {
		return models.StatusHoliday, nil
	}

	dayOffs, err := s.repo.CountDayOffsOn(companiesID, int(bangkokCheckIn.Weekday()))
	if err != nil {
		return "", err
	}
	if dayOffs > 0 {
		return models.StatusHoliday, nil
	}

	// A company with no configuration yet is a normal state, not a failure: it falls back to the default start.
	startHour, startMinute := defaultWorkStartHour, defaultWorkStartMinute
	configuration, err := s.repo.FindAttendanceConfig(companiesID)
	switch {
	case err == nil && configuration.WorkStartTime != nil:
		scheduled := timeutil.ToBangkok(*configuration.WorkStartTime)
		startHour, startMinute = scheduled.Hour(), scheduled.Minute()
	case err != nil && !errors.Is(err, repository.ErrAttendanceConfigNotFound):
		return "", err
	}

	scheduledStart := time.Date(
		bangkokCheckIn.Year(), bangkokCheckIn.Month(), bangkokCheckIn.Day(),
		startHour, startMinute, 0, 0, timeutil.BangkokLocation,
	)
	if checkIn.After(scheduledStart) {
		return models.StatusLate, nil
	}
	return models.StatusOnTime, nil
}

// validateInput enforces the field rules shared by create and update.
func validateTimeLogInput(input TimeLogInput) error {
	if input.EmployeesID == 0 {
		return ErrTimeLogEmployeeRequired
	}
	if input.WorkLocationID == nil || *input.WorkLocationID == 0 {
		return ErrTimeLogWorkLocationRequired
	}
	return nil
}

// Create files a log against an employee, deriving its status.
func (s *TimeLogService) Create(scope repository.AccessScope, input TimeLogInput) (models.TimeLog, error) {
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

	// Persist only editable fields and the employee identity that passed authorization.
	if err := s.validateWorkLocation(scope, employee.CompaniesID, *input.WorkLocationID); err != nil {
		return models.TimeLog{}, err
	}
	log := models.TimeLog{
		CompaniesID: employee.CompaniesID, EmployeesID: employee.ID,
		CheckIn: input.CheckIn, CheckOut: input.CheckOut, WorkLocationID: input.WorkLocationID,
		LocationIn: input.LocationIn, Remark: input.Remark,
	}
	log.Status, err = s.DeriveStatus(log.CompaniesID, log.CheckIn, log.CheckOut)
	if err != nil {
		return models.TimeLog{}, err
	}

	audit, err := timeLogAudit(scope, models.AuditEventTimeLogCreated, "Time log created", log, nil)
	if err != nil {
		return models.TimeLog{}, err
	}
	if err := s.repo.Create(&log, &audit); err != nil {
		return models.TimeLog{}, err
	}
	return log, nil
}

// Update rewrites a log the caller's scope covers, re-deriving its status.
func (s *TimeLogService) Update(scope repository.AccessScope, id uint, input TimeLogInput) (models.TimeLog, error) {
	log, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.TimeLog{}, err
	}
	if err := requireSameCompany(log.CompaniesID, input.CompaniesID); err != nil {
		return models.TimeLog{}, err
	}
	if err := validateTimeLogInput(input); err != nil {
		return models.TimeLog{}, err
	}

	employee, err := s.resolveEmployee(scope, input.EmployeesID, input.CompaniesID)
	if err != nil {
		return models.TimeLog{}, err
	}

	before := timeLogAuditData(log)
	if err := s.validateWorkLocation(scope, employee.CompaniesID, *input.WorkLocationID); err != nil {
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

	log.Status, err = s.DeriveStatus(log.CompaniesID, log.CheckIn, log.CheckOut)
	if err != nil {
		return models.TimeLog{}, err
	}
	audit, err := timeLogAudit(scope, models.AuditEventTimeLogUpdated, "Time log updated", log, before)
	if err != nil {
		return models.TimeLog{}, err
	}
	if err := s.repo.Save(&log, &audit); err != nil {
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
	audit, err := timeLogAudit(scope, models.AuditEventTimeLogDeleted, "Time log deleted", log, timeLogAuditData(log))
	if err != nil {
		return err
	}
	return s.repo.Delete(&log, &audit)
}

// parseClock reads an HH:MM string into its hour and minute.
func parseClock(value string) (int, int, error) {
	parsed, err := time.Parse(timeLogClockLayout, value)
	if err != nil {
		return 0, 0, err
	}
	return parsed.Hour(), parsed.Minute(), nil
}

// BulkCreate creates one log per matching weekday in a date range, using the same time of day for each.
func (s *TimeLogService) BulkCreate(scope repository.AccessScope, input BulkTimeLogInput) (BulkTimeLogResult, error) {
	startDate, err := timeutil.ParseDayStart(input.StartDate)
	if err != nil {
		return BulkTimeLogResult{}, ErrTimeLogStartDateInvalid
	}
	endDate, err := timeutil.ParseDayStart(input.EndDate)
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
	if err := s.validateWorkLocation(scope, employee.CompaniesID, workLocationID); err != nil {
		return BulkTimeLogResult{}, err
	}
	result := BulkTimeLogResult{}
	pending := make([]models.TimeLog, 0)

	for day := startDate; !day.After(endDate); day = day.AddDate(0, 0, 1) {
		if !weekdays[int(day.Weekday())] {
			continue
		}
		date := day.Format(timeLogDateLayout)

		holidays, err := s.repo.CountHolidaysOn(employee.CompaniesID, date)
		if err != nil {
			return BulkTimeLogResult{}, err
		}
		if holidays > 0 {
			result.HolidayDates = append(result.HolidayDates, date)
			continue
		}

		existing, err := s.repo.CountForEmployeeBetween(employee.ID, day, day.AddDate(0, 0, 1))
		if err != nil {
			return BulkTimeLogResult{}, err
		}
		if existing > 0 {
			result.SkippedDates = append(result.SkippedDates, date)
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
		log.Status, err = s.DeriveStatus(log.CompaniesID, log.CheckIn, log.CheckOut)
		if err != nil {
			return BulkTimeLogResult{}, err
		}
		pending = append(pending, log)
	}

	audit, err := BuildAuditLog(scope, models.AuditLogInput{
		CompaniesID: &employee.CompaniesID, EventType: models.AuditEventTimeLogCreated,
		EntityType: "time_log", Summary: "Time logs created in bulk",
		AfterData: map[string]any{"employees_id": employee.ID, "created_count": len(pending), "start_date": input.StartDate, "end_date": input.EndDate},
	})
	if err != nil {
		return BulkTimeLogResult{}, err
	}
	if err := s.repo.CreateMany(pending, &audit); err != nil {
		return BulkTimeLogResult{}, err
	}
	result.CreatedCount = len(pending)
	return result, nil
}

func timeLogAudit(scope repository.AccessScope, event models.AuditEventType, summary string, log models.TimeLog, before map[string]any) (models.AuditLog, error) {
	return BuildAuditLog(scope, models.AuditLogInput{CompaniesID: &log.CompaniesID, EventType: event, EntityType: "time_log", EntityID: &log.ID, Summary: summary, BeforeData: before, AfterData: timeLogAuditData(log)})
}

func timeLogAuditData(log models.TimeLog) map[string]any {
	data := map[string]any{"employees_id": log.EmployeesID, "status": log.Status, "work_location_id": log.WorkLocationID}
	if log.CheckIn != nil {
		data["check_in"] = log.CheckIn.UTC().Format(time.RFC3339)
	}
	if log.CheckOut != nil {
		data["check_out"] = log.CheckOut.UTC().Format(time.RFC3339)
	}
	return data
}

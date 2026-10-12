package service

import (
	"time"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/timeutil"
)

// clockLayout is the wall-clock shape the schedule fields use, with no date part.
const clockLayout = "15:04"

// Attendance-config rule failures.
var (
	ErrAttendanceCompanyDenied = apperr.Forbidden("Access denied for this company")
	ErrWorkTimeInvalid         = apperr.Validation("work_start_time and work_end_time must be HH:MM")
)

// AttendanceConfigInput is what a caller may set on a company's attendance config.
type AttendanceConfigInput struct {
	CompaniesID               uint
	LateRatePerMinute         float64
	AbsentRatePerDay          float64
	IsLateDeductionEnabled    bool
	IsAbsentDeductionEnabled  bool
	WorkStartTime             string
	WorkEndTime               string
	OvertimeRatePerMinute     float64
	ExtraRatePerDay           float64
	IsOvertimeEarningsEnabled bool
	IsExtraEarningsEnabled    bool
}

// CompanySchedule is the minimal work schedule the Time Logs page needs.
type CompanySchedule struct {
	CompaniesID   uint
	WorkStartTime string
	WorkEndTime   string
	IsConfigured  bool
}

// AttendanceConfigService owns the attendance-configuration rules.
type AttendanceConfigService struct {
	repo repository.AttendanceConfigRepository
}

// NewAttendanceConfigService wires the service to a repository implementation.
func NewAttendanceConfigService(repo repository.AttendanceConfigRepository) *AttendanceConfigService {
	return &AttendanceConfigService{repo: repo}
}

// List returns the caller's authorized companies with their configuration attached.
func (s *AttendanceConfigService) List(scope repository.AccessScope, q repository.PageQuery) (repository.Page[models.Companies], error) {
	return s.repo.ListCompaniesWithConfigPage(scope, q)
}

// Schedules returns each company's start and end time as HH:MM in the business timezone, omitting the payroll rates so a timelog-only user never sees them.
func (s *AttendanceConfigService) Schedules(scope repository.AccessScope) ([]CompanySchedule, error) {
	companies, err := s.repo.ListCompaniesWithConfig(scope)
	if err != nil {
		return nil, err
	}

	schedules := make([]CompanySchedule, 0, len(companies))
	for _, company := range companies {
		schedule := CompanySchedule{CompaniesID: company.ID}
		cfg := company.AttendanceConfig
		if cfg != nil && cfg.WorkStartTime != nil && cfg.WorkEndTime != nil {
			schedule.WorkStartTime = timeutil.ToBangkok(*cfg.WorkStartTime).Format(clockLayout)
			schedule.WorkEndTime = timeutil.ToBangkok(*cfg.WorkEndTime).Format(clockLayout)
			schedule.IsConfigured = true
		}
		schedules = append(schedules, schedule)
	}
	return schedules, nil
}

// parseWorkTime turns an HH:MM string into an instant anchored to today's date in the business timezone.
func parseWorkTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(clockLayout, value)
	if err != nil {
		return nil, ErrWorkTimeInvalid
	}
	today := timeutil.ToBangkok(time.Now())
	anchored := time.Date(today.Year(), today.Month(), today.Day(), parsed.Hour(), parsed.Minute(), 0, 0, timeutil.BangkokLocation)
	return &anchored, nil
}

// Save writes a company's attendance configuration.
func (s *AttendanceConfigService) Save(scope repository.AccessScope, input AttendanceConfigInput) (models.AttendanceConfig, error) {
	if !scope.CanAccessCompany(input.CompaniesID) {
		return models.AttendanceConfig{}, ErrAttendanceCompanyDenied
	}
	for _, field := range []struct {
		name  string
		value float64
	}{
		{"Late rate per minute", input.LateRatePerMinute},
		{"Absent rate per day", input.AbsentRatePerDay},
		{"Overtime rate per minute", input.OvertimeRatePerMinute},
		{"Extra rate per day", input.ExtraRatePerDay},
	} {
		if err := validateConfigNumber(field.name, field.value, maximumConfigAmount); err != nil {
			return models.AttendanceConfig{}, err
		}
	}

	workStart, err := parseWorkTime(input.WorkStartTime)
	if err != nil {
		return models.AttendanceConfig{}, err
	}
	workEnd, err := parseWorkTime(input.WorkEndTime)
	if err != nil {
		return models.AttendanceConfig{}, err
	}

	configuration := models.AttendanceConfig{
		CompaniesID:               input.CompaniesID,
		LateRatePerMinute:         input.LateRatePerMinute,
		AbsentRatePerDay:          input.AbsentRatePerDay,
		IsLateDeductionEnabled:    input.IsLateDeductionEnabled,
		IsAbsentDeductionEnabled:  input.IsAbsentDeductionEnabled,
		WorkStartTime:             workStart,
		WorkEndTime:               workEnd,
		OvertimeRatePerMinute:     input.OvertimeRatePerMinute,
		ExtraRatePerDay:           input.ExtraRatePerDay,
		IsOvertimeEarningsEnabled: input.IsOvertimeEarningsEnabled,
		IsExtraEarningsEnabled:    input.IsExtraEarningsEnabled,
	}
	audit, err := masterAudit(scope, models.AuditEventAttendanceConfigSaved, "Attendance configuration saved", "attendance_config", configuration.CompaniesID, &configuration.ID, nil, map[string]any{"work_start_time": input.WorkStartTime, "work_end_time": input.WorkEndTime, "late_deduction_enabled": input.IsLateDeductionEnabled, "absent_deduction_enabled": input.IsAbsentDeductionEnabled, "overtime_earnings_enabled": input.IsOvertimeEarningsEnabled, "extra_earnings_enabled": input.IsExtraEarningsEnabled})
	if err != nil {
		return models.AttendanceConfig{}, err
	}
	if err := s.repo.UpsertForCompany(&configuration, &audit); err != nil {
		return models.AttendanceConfig{}, err
	}
	return configuration, nil
}

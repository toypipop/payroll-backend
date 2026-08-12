package repository

import (
	"time"

	"payroll-backend/config"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// AttendanceSummaryRepository is the read boundary shared by the deduction and
// earning summaries. Both walk the same window of employees, schedules, calendars
// and time logs, so both draw from one place.
type AttendanceSummaryRepository interface {
	ListEmployeesScoped(scope AccessScope) ([]models.Employees, error)
	ListConfigsForCompanies(companyIDs []uint) ([]models.AttendanceConfig, error)
	ListHolidaysBetween(companyIDs []uint, from time.Time, to time.Time) ([]models.Holiday, error)
	ListDayOffsForCompanies(companyIDs []uint) ([]models.DayOff, error)
	ListApprovedLeavesOverlapping(employeeIDs []uint, from time.Time, to time.Time) ([]models.LeaveRequest, error)
	ListTimeLogsBetween(employeeIDs []uint, from time.Time, to time.Time) ([]models.TimeLog, error)
}

type gormAttendanceSummaryRepository struct {
	db *gorm.DB
}

// NewAttendanceSummaryRepository returns the GORM-backed implementation bound to the process-wide connection.
func NewAttendanceSummaryRepository() AttendanceSummaryRepository {
	return &gormAttendanceSummaryRepository{db: config.DB}
}

// ListEmployeesScoped filters by company and by employee, matching the
// employee-level declaration of both summary pages.
func (r *gormAttendanceSummaryRepository) ListEmployeesScoped(scope AccessScope) ([]models.Employees, error) {
	var employees []models.Employees
	err := r.db.Scopes(scope.ApplyEmployee("employees.companies_id", "employees.id")).
		Preload("Companies").
		Find(&employees).Error
	return employees, err
}

func (r *gormAttendanceSummaryRepository) ListConfigsForCompanies(companyIDs []uint) ([]models.AttendanceConfig, error) {
	var configs []models.AttendanceConfig
	if len(companyIDs) == 0 {
		return configs, nil
	}
	err := r.db.Where("companies_id IN ?", companyIDs).Find(&configs).Error
	return configs, err
}

func (r *gormAttendanceSummaryRepository) ListHolidaysBetween(companyIDs []uint, from time.Time, to time.Time) ([]models.Holiday, error) {
	var holidays []models.Holiday
	if len(companyIDs) == 0 {
		return holidays, nil
	}
	err := r.db.Where("companies_id IN ? AND date >= ? AND date <= ?", companyIDs, from, to).Find(&holidays).Error
	return holidays, err
}

func (r *gormAttendanceSummaryRepository) ListDayOffsForCompanies(companyIDs []uint) ([]models.DayOff, error) {
	var dayOffs []models.DayOff
	if len(companyIDs) == 0 {
		return dayOffs, nil
	}
	err := r.db.Where("companies_id IN ?", companyIDs).Find(&dayOffs).Error
	return dayOffs, err
}

func (r *gormAttendanceSummaryRepository) ListApprovedLeavesOverlapping(employeeIDs []uint, from time.Time, to time.Time) ([]models.LeaveRequest, error) {
	var leaves []models.LeaveRequest
	if len(employeeIDs) == 0 {
		return leaves, nil
	}
	err := r.db.Where(
		"employees_id IN ? AND status = ? AND start_date <= ? AND end_date >= ?",
		employeeIDs, models.LeaveStatusApproved, to, from,
	).Find(&leaves).Error
	return leaves, err
}

// ListTimeLogsBetween takes logs whose check-in falls in the window, plus logs with
// no check-in at all, which are placed by their creation time instead.
func (r *gormAttendanceSummaryRepository) ListTimeLogsBetween(employeeIDs []uint, from time.Time, to time.Time) ([]models.TimeLog, error) {
	var logs []models.TimeLog
	if len(employeeIDs) == 0 {
		return logs, nil
	}
	err := r.db.Where(
		"employees_id IN ? AND ((check_in >= ? AND check_in <= ?) OR (check_in IS NULL AND created_at >= ? AND created_at <= ?))",
		employeeIDs, from, to, from, to,
	).Find(&logs).Error
	return logs, err
}

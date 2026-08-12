package repository

import (
	"errors"
	"time"

	"payroll-backend/apperr"
	"payroll-backend/config"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// Time-log lookup failures. Update and delete answered slightly different messages
// before the split, and both are kept.
var (
	ErrTimeLogNotFound     = apperr.NotFound("Record not found or access denied")
	ErrTimeLogNotDeletable = apperr.NotFound("Record not found or unauthorized")
)

// TimeLogRepository is the persistence boundary for the time-logs domain.
type TimeLogRepository interface {
	ListScoped(scope AccessScope) ([]models.TimeLog, error)
	FindScoped(scope AccessScope, id uint) (models.TimeLog, error)
	Create(log *models.TimeLog) error
	CreateMany(logs []models.TimeLog) error
	Save(log *models.TimeLog) error
	Delete(log *models.TimeLog) error
	CountForEmployeeBetween(employeesID uint, from time.Time, to time.Time) (int64, error)
	CountHolidaysOn(companyID uint, date string) (int64, error)
	CountDayOffsOn(companyID uint, weekday int) (int64, error)
	FindAttendanceConfig(companyID uint) (models.AttendanceConfig, error)
}

type gormTimeLogRepository struct {
	db *gorm.DB
}

// NewTimeLogRepository returns the GORM-backed implementation bound to the process-wide connection.
func NewTimeLogRepository() TimeLogRepository {
	return &gormTimeLogRepository{db: config.DB}
}

// scoped filters by company and by employee: the table has a real employee column
// and TimelogsListPage is declared employee-level, so Own and Department bite.
func (r *gormTimeLogRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyEmployee("time_log.companies_id", "time_log.employees_id"))
}

func (r *gormTimeLogRepository) ListScoped(scope AccessScope) ([]models.TimeLog, error) {
	var logs []models.TimeLog
	err := r.scoped(scope).
		Preload("Employees").
		Preload("Employees.Companies").
		Find(&logs).Error
	return logs, err
}

func (r *gormTimeLogRepository) FindScoped(scope AccessScope, id uint) (models.TimeLog, error) {
	var log models.TimeLog
	if err := r.scoped(scope).First(&log, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.TimeLog{}, ErrTimeLogNotFound
		}
		return models.TimeLog{}, err
	}
	return log, nil
}

func (r *gormTimeLogRepository) Create(log *models.TimeLog) error {
	return r.db.Create(log).Error
}

// CreateMany writes a generated run as one transaction, so a failure part-way
// through leaves no logs behind instead of a partial date range.
func (r *gormTimeLogRepository) CreateMany(logs []models.TimeLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		return tx.Create(&logs).Error
	})
}

func (r *gormTimeLogRepository) Save(log *models.TimeLog) error {
	return r.db.Save(log).Error
}

func (r *gormTimeLogRepository) Delete(log *models.TimeLog) error {
	return r.db.Delete(log).Error
}

// CountForEmployeeBetween counts an employee's logs whose check-in falls in
// [from, to). The bulk generator uses it to skip a date that already has a log,
// and passes a window anchored to the business timezone so it covers exactly one
// Bangkok calendar day.
func (r *gormTimeLogRepository) CountForEmployeeBetween(employeesID uint, from time.Time, to time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.TimeLog{}).
		Where("employees_id = ? AND check_in >= ? AND check_in < ?", employeesID, from, to).
		Count(&count).Error
	return count, err
}

func (r *gormTimeLogRepository) CountHolidaysOn(companyID uint, date string) (int64, error) {
	var count int64
	err := r.db.Model(&models.Holiday{}).
		Where("companies_id = ? AND date = ?", companyID, date).
		Count(&count).Error
	return count, err
}

func (r *gormTimeLogRepository) CountDayOffsOn(companyID uint, weekday int) (int64, error) {
	var count int64
	err := r.db.Model(&models.DayOff{}).
		Where("companies_id = ? AND weekday = ?", companyID, weekday).
		Count(&count).Error
	return count, err
}

// FindAttendanceConfig returns the company's schedule configuration, if it has one.
func (r *gormTimeLogRepository) FindAttendanceConfig(companyID uint) (models.AttendanceConfig, error) {
	var configuration models.AttendanceConfig
	if err := r.db.Where("companies_id = ?", companyID).First(&configuration).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.AttendanceConfig{}, ErrAttendanceConfigNotFound
		}
		return models.AttendanceConfig{}, err
	}
	return configuration, nil
}

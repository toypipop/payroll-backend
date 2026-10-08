package repository

import (
	"errors"
	"time"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Time-log lookup failures.
var (
	ErrTimeLogNotFound     = apperr.NotFound("Record not found or access denied")
	ErrTimeLogNotDeletable = apperr.NotFound("Record not found or unauthorized")
)

// TimeLogRepository is the persistence boundary for the time-logs domain.
type TimeLogRepository interface {
	ListScoped(scope AccessScope, q PageQuery) (Page[models.TimeLog], error)
	FindScoped(scope AccessScope, id uint) (models.TimeLog, error)
	Create(log *models.TimeLog, audit *models.AuditLog) error
	CreateMany(logs []models.TimeLog, audit *models.AuditLog) error
	Save(log *models.TimeLog, audit *models.AuditLog) error
	Delete(log *models.TimeLog, audit *models.AuditLog) error
	CountForEmployeeBetween(employeesID uint, from time.Time, to time.Time) (int64, error)
	CountHolidaysOn(companyID uint, date string) (int64, error)
	CountDayOffsOn(companyID uint, weekday int) (int64, error)
	FindAttendanceConfig(companyID uint) (models.AttendanceConfig, error)
}

type gormTimeLogRepository struct {
	db *gorm.DB
}

// NewTimeLogRepository returns the GORM-backed implementation on the given connection.
func NewTimeLogRepository(db *gorm.DB) TimeLogRepository {
	return &gormTimeLogRepository{db: db}
}

// scoped filters by company and by employee: the table has a real employee column and TimelogsListPage is declared employee-level, so Own and Department bite.
func (r *gormTimeLogRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyEmployee("time_log.companies_id", "time_log.employees_id"))
}

func (r *gormTimeLogRepository) ListScoped(scope AccessScope, q PageQuery) (Page[models.TimeLog], error) {
	name := "concat_ws(' ', e.first_name, e.last_name)"
	order, err := pageOrder(q, map[string]string{
		"employee_name": name, "check_in_date": "time_log.check_in",
		"check_in_time":  "(time_log.check_in AT TIME ZONE 'Asia/Bangkok')::time",
		"check_out_time": "(time_log.check_out AT TIME ZONE 'Asia/Bangkok')::time", "employees.companies.name": "c.name",
	}, "time_log.id", "time_log.id")
	if err != nil {
		return Page[models.TimeLog]{}, err
	}
	query := r.scoped(scope).Model(&models.TimeLog{}).
		Joins("LEFT JOIN employees e ON e.id = time_log.employees_id AND e.deleted_at = 0").
		Joins("LEFT JOIN companies c ON c.id = e.companies_id AND c.deleted_at = 0")
	if q.Search != "" {
		pattern := searchPattern(q.Search)
		query = query.Where("("+name+" ILIKE ? OR e.employees_code ILIKE ? OR c.name ILIKE ? OR to_char(time_log.check_in AT TIME ZONE 'Asia/Bangkok', 'YYYY-MM-DD HH24:MI') ILIKE ? OR to_char(time_log.check_out AT TIME ZONE 'Asia/Bangkok', 'HH24:MI') ILIKE ?)", pattern, pattern, pattern, pattern, pattern)
	}
	return readPage[models.TimeLog](query.Preload("Employees", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, companies_id, first_name, last_name, employees_code")
	}).Preload("Employees.Companies"), q, order)
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

func (r *gormTimeLogRepository) Create(log *models.TimeLog, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockActiveEmployees(tx, log.EmployeesID); err != nil {
			return err
		}
		if err := tx.Omit(clause.Associations).Create(log).Error; err != nil {
			return err
		}
		if audit != nil && audit.EntityID != nil {
			*audit.EntityID = log.ID
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

// CreateMany writes a generated run as one transaction, so a failure part-way through leaves no logs behind instead of a partial date range.
func (r *gormTimeLogRepository) CreateMany(logs []models.TimeLog, audit *models.AuditLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		ids := make([]uint, 0, len(logs))
		for _, log := range logs {
			ids = append(ids, log.EmployeesID)
		}
		if err := lockActiveEmployees(tx, ids...); err != nil {
			return err
		}
		if err := tx.Omit(clause.Associations).Create(&logs).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormTimeLogRepository) Save(log *models.TimeLog, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockActiveEmployees(tx, log.EmployeesID); err != nil {
			return err
		}
		if err := tx.Omit(clause.Associations).Save(log).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormTimeLogRepository) Delete(log *models.TimeLog, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(log).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

// CountForEmployeeBetween counts an employee's logs whose check-in falls in [from, to).
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

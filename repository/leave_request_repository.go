package repository

import (
	"errors"
	"time"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Leave-request lookup failures.
var (
	ErrLeaveRequestNotFound      = apperr.NotFound("Record not found or access denied")
	ErrLeaveRequestNotDeletable  = apperr.NotFound("Record not found or unauthorized")
	ErrLeaveRequestNotApprovable = apperr.NotFound("Leave request not found or access denied")
	ErrLeaveQuotaExceeded        = apperr.Conflict("This approval would exceed the leave type's annual limit")
)

// LeaveRequestRepository is the persistence boundary for the leave-requests domain.
type LeaveRequestRepository interface {
	ListScoped(scope AccessScope, q PageQuery) (Page[models.LeaveRequest], error)
	ListPendingScoped(scope AccessScope, q PageQuery) (Page[models.LeaveRequest], error)
	FindScoped(scope AccessScope, id uint) (models.LeaveRequest, error)
	FindDetail(id uint) (models.LeaveRequest, error)
	Create(leave *models.LeaveRequest, audit *models.AuditLog) error
	UpdateFields(leave *models.LeaveRequest, fields map[string]any, audit *models.AuditLog) error
	Delete(leave *models.LeaveRequest, audit *models.AuditLog) error
	CountOverlapping(employeesID uint, start time.Time, end time.Time, excludeID uint) (int64, error)
	SumApprovedDaysInYear(employeesID uint, leaveTypeID uint, year int) (float64, error)
	ApproveWithinQuota(id uint, approverID uint, audit *models.AuditLog) error
}

type gormLeaveRequestRepository struct {
	db *gorm.DB
}

// NewLeaveRequestRepository returns the GORM-backed implementation on the given connection.
func NewLeaveRequestRepository(db *gorm.DB) LeaveRequestRepository {
	return &gormLeaveRequestRepository{db: db}
}

// scoped filters by company and by employee: the table has a real employee column and LeaveRequestsPage is declared employee-level, so Own and Department bite.
func (r *gormLeaveRequestRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyEmployee("leave_request.companies_id", "leave_request.employees_id"))
}

func (r *gormLeaveRequestRepository) ListScoped(scope AccessScope, q PageQuery) (Page[models.LeaveRequest], error) {
	return r.listPage(scope, q, false)
}

func (r *gormLeaveRequestRepository) ListPendingScoped(scope AccessScope, q PageQuery) (Page[models.LeaveRequest], error) {
	return r.listPage(scope, q, true)
}

func (r *gormLeaveRequestRepository) listPage(scope AccessScope, q PageQuery, pending bool) (Page[models.LeaveRequest], error) {
	name := "concat_ws(' ', e.first_name, e.last_name)"
	approver := "concat_ws(' ', a.first_name, a.last_name)"
	leaveType := "COALESCE(lt.name, leave_request.leave_type)"
	order, err := pageOrder(q, map[string]string{
		"employee_name": name, "leave_type_name": leaveType, "leave_type": "leave_request.leave_type",
		"start_date": "leave_request.start_date", "status": "leave_request.status", "approver_name": approver,
		"employees.companies.name": "c.name", "total_days": "leave_request.total_days", "reason": "leave_request.reason",
	}, "leave_request.id", "leave_request.id")
	if err != nil {
		return Page[models.LeaveRequest]{}, err
	}
	query := r.scoped(scope).Model(&models.LeaveRequest{}).
		Joins("LEFT JOIN employees e ON e.id = leave_request.employees_id AND e.deleted_at = 0").
		Joins("LEFT JOIN companies c ON c.id = e.companies_id AND c.deleted_at = 0").
		Joins("LEFT JOIN employees a ON a.id = leave_request.approved_by AND a.deleted_at = 0").
		Joins("LEFT JOIN leave_type_setting lt ON lt.id = leave_request.leave_type_id AND lt.deleted_at = 0")
	if pending {
		query = query.Where("leave_request.status = ?", models.LeaveStatusPending)
	}
	if q.Search != "" {
		pattern := searchPattern(q.Search)
		query = query.Where("("+name+" ILIKE ? OR e.employees_code ILIKE ? OR c.name ILIKE ? OR "+leaveType+" ILIKE ? OR "+approver+" ILIKE ? OR leave_request.status ILIKE ? OR leave_request.reason ILIKE ? OR to_char(leave_request.start_date AT TIME ZONE 'Asia/Bangkok', 'YYYY-MM-DD') ILIKE ? OR to_char(leave_request.end_date AT TIME ZONE 'Asia/Bangkok', 'YYYY-MM-DD') ILIKE ?)", pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern)
	}
	employeeFields := func(db *gorm.DB) *gorm.DB {
		return db.Select("id, companies_id, first_name, last_name, employees_code")
	}
	return readPage[models.LeaveRequest](query.Preload("Employees", employeeFields).Preload("Employees.Companies").Preload("Approver", employeeFields).Preload("LeaveTypeMaster"), q, order)
}

func (r *gormLeaveRequestRepository) FindScoped(scope AccessScope, id uint) (models.LeaveRequest, error) {
	var leave models.LeaveRequest
	if err := r.scoped(scope).First(&leave, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.LeaveRequest{}, ErrLeaveRequestNotFound
		}
		return models.LeaveRequest{}, err
	}
	return leave, nil
}

// FindDetail reloads a row with its relations for the response body.
func (r *gormLeaveRequestRepository) FindDetail(id uint) (models.LeaveRequest, error) {
	var leave models.LeaveRequest
	err := r.db.
		Preload("Employees").
		Preload("Employees.Companies").
		Preload("Approver").
		Preload("LeaveTypeMaster").
		First(&leave, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.LeaveRequest{}, ErrLeaveRequestNotFound
		}
		return models.LeaveRequest{}, err
	}
	return leave, nil
}

func (r *gormLeaveRequestRepository) Create(leave *models.LeaveRequest, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockActiveEmployees(tx, leave.EmployeesID); err != nil {
			return err
		}
		if err := tx.Create(leave).Error; err != nil {
			return err
		}
		if audit != nil && audit.EntityID != nil {
			*audit.EntityID = leave.ID
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormLeaveRequestRepository) UpdateFields(leave *models.LeaveRequest, fields map[string]any, audit *models.AuditLog) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Rejection remains available to clean up pending requests for retired employees.
		if fields["status"] != string(models.LeaveStatusRejected) && fields["status"] != models.LeaveStatusRejected {
			if err := lockEmployeeUpdate(tx, leave.EmployeesID, fields, "employees_id"); err != nil {
				return err
			}
		}
		if err := tx.Model(leave).Updates(fields).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormLeaveRequestRepository) Delete(leave *models.LeaveRequest, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(leave).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

// CountOverlapping counts the employee's pending or approved requests that overlap [start, end].
func (r *gormLeaveRequestRepository) CountOverlapping(employeesID uint, start time.Time, end time.Time, excludeID uint) (int64, error) {
	query := r.db.Model(&models.LeaveRequest{}).
		Where("employees_id = ? AND status IN (?)", employeesID,
			[]models.LeaveStatus{models.LeaveStatusPending, models.LeaveStatusApproved}).
		Where("NOT (start_date > ? OR end_date < ?)", end, start)
	if excludeID != 0 {
		query = query.Where("id <> ?", excludeID)
	}

	var count int64
	err := query.Count(&count).Error
	return count, err
}

// SumApprovedDaysInYear totals the portion of approved leave that falls within one calendar year.
// A leave crossing New Year therefore contributes only the days in the requested year.
func (r *gormLeaveRequestRepository) SumApprovedDaysInYear(employeesID uint, leaveTypeID uint, year int) (float64, error) {
	yearStart := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	yearEnd := time.Date(year, time.December, 31, 23, 59, 59, 999999999, time.UTC)
	var approved []models.LeaveRequest
	err := r.db.
		Where("employees_id = ? AND leave_type_id = ? AND status = ?", employeesID, leaveTypeID, models.LeaveStatusApproved).
		Where("start_date <= ? AND end_date >= ?", yearEnd, yearStart).
		Find(&approved).Error
	if err != nil {
		return 0, err
	}

	var used float64
	for _, leave := range approved {
		used += daysByCalendarYear(leave.StartDate, leave.EndDate)[year]
	}
	return used, nil
}

func daysByCalendarYear(start, end time.Time) map[int]float64 {
	days := make(map[int]float64)
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		days[day.Year()]++
	}
	return days
}

// ApproveWithinQuota serializes approvals for one employee and checks the current leave-type cap in the same transaction.
func (r *gormLeaveRequestRepository) ApproveWithinQuota(id uint, approverID uint, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var leave models.LeaveRequest
		if err := tx.First(&leave, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrLeaveRequestNotApprovable
			}
			return err
		}
		// Also serialize quota checks for this employee; UPDATE avoids upgrading a
		// shared lock later. All writers lock the employee before the request.
		if err := lockEmployeeStatus(tx, leave.EmployeesID, "UPDATE"); err != nil {
			return err
		}
		lockedEmployeeID := leave.EmployeesID
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&leave, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrLeaveRequestNotApprovable
			}
			return err
		}
		if leave.EmployeesID != lockedEmployeeID {
			return apperr.Conflict("This leave request was changed by someone else. Reload and try again.")
		}
		if leave.Status != models.LeaveStatusPending {
			return apperr.Validation("This request has already been processed")
		}

		if leave.LeaveTypeID != nil {
			var leaveType models.LeaveTypeSetting
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&leaveType, *leave.LeaveTypeID).Error; err != nil {
				return err
			}
			if leaveType.MaxDaysPerYear > 0 {
				firstYearStart := time.Date(leave.StartDate.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
				lastYearEnd := time.Date(leave.EndDate.Year(), time.December, 31, 23, 59, 59, 0, time.UTC)
				var approved []models.LeaveRequest
				if err := tx.Where("employees_id = ? AND leave_type_id = ? AND status = ?", leave.EmployeesID, *leave.LeaveTypeID, models.LeaveStatusApproved).
					Where("start_date <= ? AND end_date >= ?", lastYearEnd, firstYearStart).
					Find(&approved).Error; err != nil {
					return err
				}
				used := make(map[int]float64)
				for _, approvedLeave := range approved {
					for year, days := range daysByCalendarYear(approvedLeave.StartDate, approvedLeave.EndDate) {
						used[year] += days
					}
				}
				for year, days := range daysByCalendarYear(leave.StartDate, leave.EndDate) {
					if used[year]+days > leaveType.MaxDaysPerYear {
						return ErrLeaveQuotaExceeded
					}
				}
			}
		}

		if err := tx.Model(&leave).Updates(map[string]any{
			"status":      models.LeaveStatusApproved,
			"approved_by": approverID,
			"updated_at":  time.Now(),
		}).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

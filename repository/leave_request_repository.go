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
	ListScoped(scope AccessScope) ([]models.LeaveRequest, error)
	ListPendingScoped(scope AccessScope) ([]models.LeaveRequest, error)
	FindScoped(scope AccessScope, id uint) (models.LeaveRequest, error)
	FindDetail(id uint) (models.LeaveRequest, error)
	Create(leave *models.LeaveRequest) error
	UpdateFields(leave *models.LeaveRequest, fields map[string]any) error
	Delete(leave *models.LeaveRequest) error
	CountOverlapping(employeesID uint, start time.Time, end time.Time, excludeID uint) (int64, error)
	SumApprovedDaysInYear(employeesID uint, leaveTypeID uint, year int) (float64, error)
	ApproveWithinQuota(id uint, approverID uint) error
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

func (r *gormLeaveRequestRepository) ListScoped(scope AccessScope) ([]models.LeaveRequest, error) {
	var leaves []models.LeaveRequest
	err := r.scoped(scope).
		Preload("Employees").
		Preload("Employees.Companies").
		Preload("Approver").
		Find(&leaves).Error
	return leaves, err
}

// ListPendingScoped backs the approval page, which only ever shows Pending rows.
func (r *gormLeaveRequestRepository) ListPendingScoped(scope AccessScope) ([]models.LeaveRequest, error) {
	var leaves []models.LeaveRequest
	err := r.scoped(scope).
		Preload("Employees").
		Preload("Employees.Companies").
		Preload("LeaveTypeMaster").
		Where("status = ?", models.LeaveStatusPending).
		Find(&leaves).Error
	return leaves, err
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

func (r *gormLeaveRequestRepository) Create(leave *models.LeaveRequest) error {
	return r.db.Create(leave).Error
}

func (r *gormLeaveRequestRepository) UpdateFields(leave *models.LeaveRequest, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Model(leave).Updates(fields).Error
}

func (r *gormLeaveRequestRepository) Delete(leave *models.LeaveRequest) error {
	return r.db.Delete(leave).Error
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
func (r *gormLeaveRequestRepository) ApproveWithinQuota(id uint, approverID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var leave models.LeaveRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&leave, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrLeaveRequestNotApprovable
			}
			return err
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
				var employee models.Employees
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").First(&employee, leave.EmployeesID).Error; err != nil {
					return err
				}
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

		return tx.Model(&leave).Updates(map[string]any{
			"status":      models.LeaveStatusApproved,
			"approved_by": approverID,
			"updated_at":  time.Now(),
		}).Error
	})
}

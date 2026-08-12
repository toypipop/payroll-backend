package repository

import (
	"errors"
	"time"

	"payroll-backend/apperr"
	"payroll-backend/config"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// Leave-request lookup failures. The three endpoints answered three different
// messages before the split, and all three are kept.
var (
	ErrLeaveRequestNotFound      = apperr.NotFound("Record not found or access denied")
	ErrLeaveRequestNotDeletable  = apperr.NotFound("Record not found or unauthorized")
	ErrLeaveRequestNotApprovable = apperr.NotFound("Leave request not found or access denied")
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
}

type gormLeaveRequestRepository struct {
	db *gorm.DB
}

// NewLeaveRequestRepository returns the GORM-backed implementation bound to the process-wide connection.
func NewLeaveRequestRepository() LeaveRequestRepository {
	return &gormLeaveRequestRepository{db: config.DB}
}

// scoped filters by company and by employee: the table has a real employee column
// and LeaveRequestsPage is declared employee-level, so Own and Department bite.
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

// CountOverlapping counts the employee's pending or approved requests that overlap
// [start, end]. Pass excludeID to ignore the row being edited.
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

// SumApprovedDaysInYear totals the approved days an employee has already taken of one
// leave type in a calendar year, for the quota warning on the approval page.
func (r *gormLeaveRequestRepository) SumApprovedDaysInYear(employeesID uint, leaveTypeID uint, year int) (float64, error) {
	var used float64
	err := r.db.Model(&models.LeaveRequest{}).
		Where("employees_id = ? AND leave_type_id = ? AND status = ?", employeesID, leaveTypeID, models.LeaveStatusApproved).
		Where("EXTRACT(YEAR FROM start_date) = ?", year).
		Select("COALESCE(SUM(total_days), 0)").
		Scan(&used).Error
	return used, err
}

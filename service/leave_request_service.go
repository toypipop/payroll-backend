package service

import (
	"time"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// Leave-request rule failures. This domain answers 400 where other domains answer
// 403 for an employee outside the caller's reach; that is preserved.
var (
	ErrLeaveEndBeforeStart   = apperr.Validation("End date must be on or after start date")
	ErrLeaveTypeRequired     = apperr.Validation("Leave type is required")
	ErrLeaveEmployeeInvalid  = apperr.Validation("Invalid employee or access denied")
	ErrLeaveTypeInvalid      = apperr.Validation("Invalid leave type for your company")
	ErrLeaveTypeCompany      = apperr.Validation("Leave type must belong to employee company")
	ErrLeaveOverlap          = apperr.Conflict("This employee already has a leave request during this period")
	ErrLeaveNotPending       = apperr.Forbidden("Only pending requests can be modified")
	ErrLeaveAlreadyProcessed = apperr.Validation("This request has already been processed")
	ErrLeaveStatusInvalid    = apperr.Validation("status must be Approved or Rejected")
)

// Re-exported so handlers import a single package.
var (
	ErrLeaveRequestNotFound      = repository.ErrLeaveRequestNotFound
	ErrLeaveRequestNotDeletable  = repository.ErrLeaveRequestNotDeletable
	ErrLeaveRequestNotApprovable = repository.ErrLeaveRequestNotApprovable
)

// LeaveInput is what a caller may set on a leave request.
type LeaveInput struct {
	EmployeesID uint
	CompaniesID uint
	LeaveTypeID uint
	StartDate   time.Time
	EndDate     time.Time
	Reason      string
}

// PendingLeave is a pending request plus the quota figures the approval page warns on.
type PendingLeave struct {
	Request       models.LeaveRequest
	QuotaMaxDays  float64
	QuotaUsedDays float64
	QuotaExceeded bool
}

// LeaveRequestService owns the leave rules: no overlapping requests, a leave type
// belonging to the employee's company, and only a pending request being editable.
type LeaveRequestService struct {
	repo       repository.LeaveRequestRepository
	employees  repository.EmployeesRepository
	leaveTypes repository.LeaveTypeSettingRepository
}

// NewLeaveRequestService wires the service to repository implementations.
func NewLeaveRequestService(
	repo repository.LeaveRequestRepository,
	employees repository.EmployeesRepository,
	leaveTypes repository.LeaveTypeSettingRepository,
) *LeaveRequestService {
	return &LeaveRequestService{repo: repo, employees: employees, leaveTypes: leaveTypes}
}

// dateOnly strips the time of day, anchored to UTC.
//
// Leave dates are calendar values with no time-of-day component, so they are
// deliberately NOT run through timeutil: they are compared against each other and
// against EXTRACT(YEAR FROM ...), and anchoring them to +07 would shift them.
func dateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

// totalDays counts the inclusive span between two date-only values.
func totalDays(start time.Time, end time.Time) float64 {
	return float64(int(end.Sub(start).Hours()/24)) + 1
}

// List returns the requests the caller's scope covers.
func (s *LeaveRequestService) List(scope repository.AccessScope) ([]models.LeaveRequest, error) {
	return s.repo.ListScoped(scope)
}

// resolveTarget checks the employee and the leave type against the requested company.
func (s *LeaveRequestService) resolveTarget(scope repository.AccessScope, input LeaveInput) (models.Employees, models.LeaveTypeSetting, error) {
	if input.EmployeesID == 0 || input.CompaniesID == 0 {
		return models.Employees{}, models.LeaveTypeSetting{}, ErrLeaveEmployeeInvalid
	}
	if _, ok := scope.ResolveCompanyID(input.CompaniesID); !ok {
		return models.Employees{}, models.LeaveTypeSetting{}, ErrLeaveEmployeeInvalid
	}
	employee, err := s.employees.FindBriefScoped(scope, input.EmployeesID)
	if err != nil || employee.CompaniesID != input.CompaniesID {
		return models.Employees{}, models.LeaveTypeSetting{}, ErrLeaveEmployeeInvalid
	}

	leaveType, err := s.leaveTypes.FindScoped(scope, input.LeaveTypeID)
	if err != nil {
		return models.Employees{}, models.LeaveTypeSetting{}, ErrLeaveTypeInvalid
	}
	if leaveType.CompaniesID != employee.CompaniesID {
		return models.Employees{}, models.LeaveTypeSetting{}, ErrLeaveTypeCompany
	}
	return employee, leaveType, nil
}

// Create files a pending leave request.
func (s *LeaveRequestService) Create(scope repository.AccessScope, input LeaveInput) (models.LeaveRequest, error) {
	start, end := dateOnly(input.StartDate), dateOnly(input.EndDate)
	if end.Before(start) {
		return models.LeaveRequest{}, ErrLeaveEndBeforeStart
	}
	if input.LeaveTypeID == 0 {
		return models.LeaveRequest{}, ErrLeaveTypeRequired
	}

	employee, leaveType, err := s.resolveTarget(scope, input)
	if err != nil {
		return models.LeaveRequest{}, err
	}

	overlapping, err := s.repo.CountOverlapping(input.EmployeesID, start, end, 0)
	if err != nil {
		return models.LeaveRequest{}, err
	}
	if overlapping > 0 {
		return models.LeaveRequest{}, ErrLeaveOverlap
	}

	leaveTypeID := input.LeaveTypeID
	leave := models.LeaveRequest{
		CompaniesID: employee.CompaniesID,
		EmployeesID: input.EmployeesID,
		LeaveTypeID: &leaveTypeID,
		LeaveType:   models.LeaveType(leaveType.Name),
		StartDate:   start,
		EndDate:     end,
		TotalDays:   totalDays(start, end),
		Reason:      input.Reason,
		Status:      models.LeaveStatusPending,
	}
	if err := s.repo.Create(&leave); err != nil {
		return models.LeaveRequest{}, err
	}
	return leave, nil
}

// Update rewrites a request. Only a pending request may be edited.
func (s *LeaveRequestService) Update(scope repository.AccessScope, id uint, input LeaveInput) (models.LeaveRequest, error) {
	leave, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.LeaveRequest{}, err
	}
	if leave.Status != models.LeaveStatusPending {
		return models.LeaveRequest{}, ErrLeaveNotPending
	}

	start, end := dateOnly(input.StartDate), dateOnly(input.EndDate)
	if end.Before(start) {
		return models.LeaveRequest{}, ErrLeaveEndBeforeStart
	}

	overlapping, err := s.repo.CountOverlapping(input.EmployeesID, start, end, leave.ID)
	if err != nil {
		return models.LeaveRequest{}, err
	}
	if overlapping > 0 {
		return models.LeaveRequest{}, ErrLeaveOverlap
	}

	employee, leaveType, err := s.resolveTarget(scope, input)
	if err != nil {
		return models.LeaveRequest{}, err
	}

	fields := map[string]any{
		"employees_id":  input.EmployeesID,
		"leave_type_id": input.LeaveTypeID,
		"leave_type":    models.LeaveType(leaveType.Name),
		"start_date":    start,
		"end_date":      end,
		"total_days":    totalDays(start, end),
		"reason":        input.Reason,
		"companies_id":  employee.CompaniesID,
		"updated_at":    time.Now(),
	}
	if err := s.repo.UpdateFields(&leave, fields); err != nil {
		return models.LeaveRequest{}, err
	}

	// A failed reload still returns the updated row rather than an error, which is
	// how this behaved before.
	if detailed, err := s.repo.FindDetail(leave.ID); err == nil {
		return detailed, nil
	}
	return leave, nil
}

// Delete removes a request the caller's scope covers.
func (s *LeaveRequestService) Delete(scope repository.AccessScope, id uint) error {
	leave, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return ErrLeaveRequestNotDeletable
	}
	return s.repo.Delete(&leave)
}

// Pending returns the requests awaiting approval, each decorated with how much of
// the leave type's yearly quota the employee has already used. The quota only warns;
// it never blocks.
func (s *LeaveRequestService) Pending(scope repository.AccessScope) ([]PendingLeave, error) {
	leaves, err := s.repo.ListPendingScoped(scope)
	if err != nil {
		return nil, err
	}

	pending := make([]PendingLeave, 0, len(leaves))
	for _, leave := range leaves {
		entry := PendingLeave{Request: leave}
		if leave.LeaveTypeID != nil && leave.LeaveTypeMaster != nil && leave.LeaveTypeMaster.MaxDaysPerYear > 0 {
			used, err := s.repo.SumApprovedDaysInYear(leave.EmployeesID, *leave.LeaveTypeID, leave.StartDate.Year())
			if err != nil {
				return nil, err
			}
			entry.QuotaMaxDays = leave.LeaveTypeMaster.MaxDaysPerYear
			entry.QuotaUsedDays = used
			entry.QuotaExceeded = used+leave.TotalDays > entry.QuotaMaxDays
		}
		pending = append(pending, entry)
	}
	return pending, nil
}

// Decide approves or rejects a pending request.
//
// The approver is the authenticated caller, not a value from the request body. The
// body used to supply it, and the frontend read it out of localStorage, so anyone
// able to approve could record the approval against another employee's name.
func (s *LeaveRequestService) Decide(scope repository.AccessScope, id uint, status string) error {
	if status != string(models.LeaveStatusApproved) && status != string(models.LeaveStatusRejected) {
		return ErrLeaveStatusInvalid
	}

	leave, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return ErrLeaveRequestNotApprovable
	}
	if leave.Status != models.LeaveStatusPending {
		return ErrLeaveAlreadyProcessed
	}

	approverID := scope.User.ID
	return s.repo.UpdateFields(&leave, map[string]any{
		"status":      status,
		"approved_by": approverID,
		"updated_at":  time.Now(),
	})
}

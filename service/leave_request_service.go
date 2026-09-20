package service

import (
	"time"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// Leave-request rule failures.
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
	ErrLeaveQuotaExceeded        = repository.ErrLeaveQuotaExceeded
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
	QuotaYears    []LeaveQuotaYear
}

// LeaveQuotaYear describes the yearly portion of a pending request's annual quota.
type LeaveQuotaYear struct {
	Year          int
	MaxDays       float64
	UsedDays      float64
	RequestedDays float64
	Exceeded      bool
}

// LeaveRequestService owns the leave rules: no overlapping requests, a leave type belonging to the employee's company, and only a pending request being editable.
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
func dateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

// totalDays counts the inclusive span between two date-only values.
func totalDays(start time.Time, end time.Time) float64 {
	return float64(int(end.Sub(start).Hours()/24)) + 1
}

// List returns the requests the caller's scope covers.
func (s *LeaveRequestService) List(scope repository.AccessScope, q repository.PageQuery) (repository.Page[models.LeaveRequest], error) {
	return s.repo.ListScoped(scope, q)
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
	if err := requireActiveEmployee(employee); err != nil {
		return models.Employees{}, models.LeaveTypeSetting{}, err
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
	audit, err := leaveAudit(scope, models.AuditEventLeaveCreated, "Leave request created", leave, nil)
	if err != nil {
		return models.LeaveRequest{}, err
	}
	if err := s.repo.Create(&leave, &audit); err != nil {
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

	before := leaveAuditData(leave)
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
	updated := leave
	updated.EmployeesID, updated.CompaniesID, updated.LeaveTypeID = input.EmployeesID, employee.CompaniesID, &input.LeaveTypeID
	updated.LeaveType, updated.StartDate, updated.EndDate, updated.TotalDays = models.LeaveType(leaveType.Name), start, end, totalDays(start, end)
	audit, err := leaveAudit(scope, models.AuditEventLeaveUpdated, "Leave request updated", updated, before)
	if err != nil {
		return models.LeaveRequest{}, err
	}
	if err := s.repo.UpdateFields(&leave, fields, &audit); err != nil {
		return models.LeaveRequest{}, err
	}

	// A failed reload still returns the updated row rather than an error, which is how this behaved before.
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
	audit, err := leaveAudit(scope, models.AuditEventLeaveDeleted, "Leave request deleted", leave, leaveAuditData(leave))
	if err != nil {
		return err
	}
	return s.repo.Delete(&leave, &audit)
}

// Pending returns the requests awaiting approval, each decorated with the quota for every calendar year they touch.
func (s *LeaveRequestService) Pending(scope repository.AccessScope, q repository.PageQuery) (repository.Page[PendingLeave], error) {
	page, err := s.repo.ListPendingScoped(scope, q)
	if err != nil {
		return repository.Page[PendingLeave]{}, err
	}

	pending := make([]PendingLeave, 0, len(page.Items))
	for _, leave := range page.Items {
		entry := PendingLeave{Request: leave}
		if leave.LeaveTypeID != nil && leave.LeaveTypeMaster != nil && leave.LeaveTypeMaster.MaxDaysPerYear > 0 {
			requestedByYear := leaveDaysByCalendarYear(leave.StartDate, leave.EndDate)
			for year := leave.StartDate.Year(); year <= leave.EndDate.Year(); year++ {
				requestedDays := requestedByYear[year]
				used, err := s.repo.SumApprovedDaysInYear(leave.EmployeesID, *leave.LeaveTypeID, year)
				if err != nil {
					return repository.Page[PendingLeave]{}, err
				}
				quotaYear := LeaveQuotaYear{
					Year:          year,
					MaxDays:       leave.LeaveTypeMaster.MaxDaysPerYear,
					UsedDays:      used,
					RequestedDays: requestedDays,
					Exceeded:      used+requestedDays > leave.LeaveTypeMaster.MaxDaysPerYear,
				}
				entry.QuotaYears = append(entry.QuotaYears, quotaYear)
				if quotaYear.Exceeded {
					entry.QuotaExceeded = true
				}
				if year == leave.StartDate.Year() {
					// Retain the existing fields for API consumers while quota_years
					// provides the complete cross-year explanation.
					entry.QuotaMaxDays = quotaYear.MaxDays
					entry.QuotaUsedDays = quotaYear.UsedDays
				}
			}
		}
		pending = append(pending, entry)
	}
	return repository.Page[PendingLeave]{Items: pending, Pagination: page.Pagination}, nil
}

func leaveDaysByCalendarYear(start time.Time, end time.Time) map[int]float64 {
	days := make(map[int]float64)
	for day := dateOnly(start); !day.After(end); day = day.AddDate(0, 0, 1) {
		days[day.Year()]++
	}
	return days
}

// Decide approves or rejects a pending request.
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

	if status == string(models.LeaveStatusApproved) {
		if err := activeEmployeeByID(s.employees, scope, leave.EmployeesID); err != nil {
			return err
		}
		approvedBy := scope.User.ID
		updated := leave
		updated.Status, updated.ApprovedBy = models.LeaveStatusApproved, &approvedBy
		audit, err := leaveAudit(scope, models.AuditEventLeaveApproved, "Leave request approved", updated, leaveAuditData(leave))
		if err != nil {
			return err
		}
		return s.repo.ApproveWithinQuota(leave.ID, scope.User.ID, &audit)
	}
	approvedBy := scope.User.ID
	updated := leave
	updated.Status, updated.ApprovedBy = models.LeaveStatusRejected, &approvedBy
	audit, err := leaveAudit(scope, models.AuditEventLeaveRejected, "Leave request rejected", updated, leaveAuditData(leave))
	if err != nil {
		return err
	}
	return s.repo.UpdateFields(&leave, map[string]any{
		"status":      status,
		"approved_by": scope.User.ID,
		"updated_at":  time.Now(),
	}, &audit)
}

func leaveAudit(scope repository.AccessScope, event models.AuditEventType, summary string, leave models.LeaveRequest, before map[string]any) (models.AuditLog, error) {
	return BuildAuditLog(scope, models.AuditLogInput{CompaniesID: &leave.CompaniesID, EventType: event, EntityType: "leave_request", EntityID: &leave.ID, Summary: summary, BeforeData: before, AfterData: leaveAuditData(leave)})
}

func leaveAuditData(leave models.LeaveRequest) map[string]any {
	data := map[string]any{"employees_id": leave.EmployeesID, "leave_type_id": leave.LeaveTypeID, "leave_type": leave.LeaveType, "start_date": leave.StartDate.Format("2006-01-02"), "end_date": leave.EndDate.Format("2006-01-02"), "total_days": leave.TotalDays, "status": leave.Status}
	if leave.ApprovedBy != nil {
		data["approved_by"] = *leave.ApprovedBy
	}
	return data
}

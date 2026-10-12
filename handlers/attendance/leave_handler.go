package attendance

import (
	"net/http"
	"payroll-backend/apperr"
	"time"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"
	"payroll-backend/timeutil"

	"github.com/gin-gonic/gin"
)

type leaveWriteRequest struct {
	EmployeesID uint   `json:"employees_id" binding:"required"`
	CompaniesID uint   `json:"companies_id" binding:"required"`
	LeaveTypeID *uint  `json:"leave_type_id" binding:"required"`
	StartDate   string `json:"start_date" binding:"required"`
	EndDate     string `json:"end_date" binding:"required"`
	Reason      string `json:"reason"`
}

// Calendar dates are the current wire contract. Accept legacy RFC3339 clients
// during rollout, interpreting their instants on the Bangkok calendar.
func parseLeaveDate(value string) (time.Time, error) {
	if date, err := timeutil.ParseDayStart(value); err == nil {
		return date, nil
	}
	instant, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return timeutil.DayStart(instant), nil
}

func (input leaveWriteRequest) serviceInput(c *gin.Context) (service.LeaveInput, bool) {
	start, err := parseLeaveDate(input.StartDate)
	if err != nil {
		httpx.RespondError(c, apperr.Validation("Invalid start_date; use YYYY-MM-DD"))
		return service.LeaveInput{}, false
	}
	end, err := parseLeaveDate(input.EndDate)
	if err != nil {
		httpx.RespondError(c, apperr.Validation("Invalid end_date; use YYYY-MM-DD"))
		return service.LeaveInput{}, false
	}
	leaveTypeID := uint(0)
	if input.LeaveTypeID != nil {
		leaveTypeID = *input.LeaveTypeID
	}
	return service.LeaveInput{EmployeesID: input.EmployeesID, CompaniesID: input.CompaniesID, LeaveTypeID: leaveTypeID, StartDate: start, EndDate: end, Reason: input.Reason}, true
}

func (h *Handler) GetLeaveRequests(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	query, ok := httpx.ParsePage(c)
	if !ok {
		return
	}
	leaves, err := h.LeaveRequest.List(scope, query)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, leaves)
}

func (h *Handler) CreateLeaveRequest(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input leaveWriteRequest
	if !httpx.BindJSON(c, &input) {
		return
	}

	request, ok := input.serviceInput(c)
	if !ok {
		return
	}
	leave, err := h.LeaveRequest.Create(scope, request)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, leave)
}

func (h *Handler) UpdateLeaveRequest(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "leave request")
	if !ok {
		return
	}

	var input leaveWriteRequest
	if !httpx.BindJSON(c, &input) {
		return
	}

	request, ok := input.serviceInput(c)
	if !ok {
		return
	}
	leave, err := h.LeaveRequest.Update(scope, id, request)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, leave)
}

func (h *Handler) DeleteLeaveRequest(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "leave request")
	if !ok {
		return
	}

	if err := h.LeaveRequest.Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

// pendingLeaveResponse adds yearly quota usage so the approval page can warn without blocking.
type pendingLeaveResponse struct {
	models.LeaveRequest
	QuotaMaxDays  float64                  `json:"quota_max_days"`
	QuotaUsedDays float64                  `json:"quota_used_days"`
	QuotaExceeded bool                     `json:"quota_exceeded"`
	QuotaYears    []service.LeaveQuotaYear `json:"quota_years"`
}

func (h *Handler) GetPendingLeaveRequests(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	query, ok := httpx.ParsePage(c)
	if !ok {
		return
	}
	pending, err := h.LeaveRequest.Pending(scope, query)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	responses := make([]pendingLeaveResponse, 0, len(pending.Items))
	for _, entry := range pending.Items {
		responses = append(responses, pendingLeaveResponse{
			LeaveRequest:  entry.Request,
			QuotaMaxDays:  entry.QuotaMaxDays,
			QuotaUsedDays: entry.QuotaUsedDays,
			QuotaExceeded: entry.QuotaExceeded,
			QuotaYears:    entry.QuotaYears,
		})
	}
	c.JSON(http.StatusOK, repository.Page[pendingLeaveResponse]{Items: responses, Pagination: pending.Pagination})
}

// ApproveLeaveRequest records an approval or rejection.
func (h *Handler) ApproveLeaveRequest(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "leave request")
	if !ok {
		return
	}

	var input struct {
		Status     string `json:"status" binding:"required,oneof=Approved Rejected"`
		ApprovedBy uint   `json:"approved_by"`
	}
	if !httpx.BindJSON(c, &input) {
		return
	}

	if err := h.LeaveRequest.Decide(scope, id, input.Status); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Status updated to " + input.Status})
}

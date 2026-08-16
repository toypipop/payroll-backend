package attendance

import (
	"net/http"
	"time"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// leaveRequestService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func leaveRequestService() *service.LeaveRequestService {
	return service.NewLeaveRequestService(
		repository.NewLeaveRequestRepository(),
		repository.NewEmployeesRepository(),
		repository.NewLeaveTypeSettingRepository(),
	)
}

func GetLeaveRequests(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	leaves, err := leaveRequestService().List(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, leaves)
}

func CreateLeaveRequest(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.LeaveRequest
	if !httpx.BindJSON(c, &input) {
		return
	}

	leaveTypeID := uint(0)
	if input.LeaveTypeID != nil {
		leaveTypeID = *input.LeaveTypeID
	}

	leave, err := leaveRequestService().Create(scope, service.LeaveInput{
		EmployeesID: input.EmployeesID,
		CompaniesID: input.CompaniesID,
		LeaveTypeID: leaveTypeID,
		StartDate:   input.StartDate,
		EndDate:     input.EndDate,
		Reason:      input.Reason,
	})
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, leave)
}

func UpdateLeaveRequest(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "leave request")
	if !ok {
		return
	}

	var input struct {
		EmployeesID uint      `json:"employees_id" binding:"required"`
		CompaniesID uint      `json:"companies_id" binding:"required"`
		LeaveTypeID uint      `json:"leave_type_id" binding:"required"`
		StartDate   time.Time `json:"start_date" binding:"required"`
		EndDate     time.Time `json:"end_date" binding:"required"`
		Reason      string    `json:"reason"`
	}
	if !httpx.BindJSON(c, &input) {
		return
	}

	leave, err := leaveRequestService().Update(scope, id, service.LeaveInput{
		EmployeesID: input.EmployeesID,
		CompaniesID: input.CompaniesID,
		LeaveTypeID: input.LeaveTypeID,
		StartDate:   input.StartDate,
		EndDate:     input.EndDate,
		Reason:      input.Reason,
	})
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, leave)
}

func DeleteLeaveRequest(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "leave request")
	if !ok {
		return
	}

	if err := leaveRequestService().Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

// pendingLeaveResponse adds yearly quota usage so the approval page can warn without blocking.
type pendingLeaveResponse struct {
	models.LeaveRequest
	QuotaMaxDays  float64 `json:"quota_max_days"`
	QuotaUsedDays float64 `json:"quota_used_days"`
	QuotaExceeded bool    `json:"quota_exceeded"`
}

func GetPendingLeaveRequests(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	pending, err := leaveRequestService().Pending(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	responses := make([]pendingLeaveResponse, 0, len(pending))
	for _, entry := range pending {
		responses = append(responses, pendingLeaveResponse{
			LeaveRequest:  entry.Request,
			QuotaMaxDays:  entry.QuotaMaxDays,
			QuotaUsedDays: entry.QuotaUsedDays,
			QuotaExceeded: entry.QuotaExceeded,
		})
	}
	c.JSON(http.StatusOK, responses)
}

// ApproveLeaveRequest records an approval or rejection.
func ApproveLeaveRequest(c *gin.Context) {
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

	if err := leaveRequestService().Decide(scope, id, input.Status); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Status updated to " + input.Status})
}

package attendance

import (
	"net/http"
	"time"

	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

func GetLeaveRequests(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var leaves []models.LeaveRequest
	// Apply data scope to ensure users see only what they are allowed to (English Comment)
	err := config.DB.Scopes(
		repository.ApplyEmployeeScopes(c, currentUser, dataScope, "leave_request.companies_id", "leave_request.employees_id"),
	).Preload("Employees").Preload("Employees.Companies").Preload("Approver").Find(&leaves).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, leaves)
}

func CreateLeaveRequest(c *gin.Context) {
	var input models.LeaveRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user session invalid"})
		return
	}

	// Normalize CreateLeaveRequest dates to midnight UTC before validation
	input.StartDate = time.Date(input.StartDate.Year(), input.StartDate.Month(), input.StartDate.Day(), 0, 0, 0, 0, time.UTC)
	input.EndDate = time.Date(input.EndDate.Year(), input.EndDate.Month(), input.EndDate.Day(), 0, 0, 0, 0, time.UTC)

	if input.EndDate.Before(input.StartDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "End date must be on or after start date"})
		return
	}

	if input.LeaveTypeID == nil || *input.LeaveTypeID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Leave type is required"})
		return
	}

	targetEmployee, ok := repository.ResolveEmployeeForCompany(c, currentUser, dataScope, input.EmployeesID, input.CompaniesID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid employee or access denied"})
		return
	}

	var leaveType models.LeaveTypeSetting
	if err := config.DB.Scopes(
		repository.ApplyCompanyScopes(c, currentUser, "leave_type_setting.companies_id"),
	).Where("id = ?", *input.LeaveTypeID).First(&leaveType).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid leave type for your company"})
		return
	}

	if leaveType.CompaniesID != targetEmployee.CompaniesID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Leave type must belong to employee company"})
		return
	}

	input.LeaveType = models.LeaveType(leaveType.Name)

	// Change from userID to input.EmployeesID to check the selected employee
	var count int64
	config.DB.Model(&models.LeaveRequest{}).
		Where("employees_id = ? AND status IN (?)", input.EmployeesID, []models.LeaveStatus{models.LeaveStatusPending, models.LeaveStatusApproved}).
		Where("NOT (start_date > ? OR end_date < ?)", input.EndDate, input.StartDate).
		Count(&count)

	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "This employee already has a leave request during this period"})
		return
	}

	input.CompaniesID = targetEmployee.CompaniesID
	input.Status = models.LeaveStatusPending

	input.TotalDays = float64(int(input.EndDate.Sub(input.StartDate).Hours()/24)) + 1

	if err := config.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, input)
}

func UpdateLeaveRequest(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	var leave models.LeaveRequest
	// Check access rights before updating (English Comment)
	if err := config.DB.Scopes(
		repository.ApplyEmployeeScopes(c, currentUser, dataScope, "leave_request.companies_id", "leave_request.employees_id"),
	).First(&leave, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or access denied"})
		return
	}

	// Business Logic: Only pending requests can be edited (English Comment)
	if leave.Status != models.LeaveStatusPending {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only pending requests can be modified"})
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

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Normalize dates to midnight UTC for date-only storage
	input.StartDate = time.Date(input.StartDate.Year(), input.StartDate.Month(), input.StartDate.Day(), 0, 0, 0, 0, time.UTC)
	input.EndDate = time.Date(input.EndDate.Year(), input.EndDate.Month(), input.EndDate.Day(), 0, 0, 0, 0, time.UTC)

	if input.EndDate.Before(input.StartDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "End date must be on or after start date"})
		return
	}

	var count int64
	config.DB.Model(&models.LeaveRequest{}).
		Where("id <> ?", leave.ID).
		Where("employees_id = ? AND status IN (?)", input.EmployeesID, []models.LeaveStatus{models.LeaveStatusPending, models.LeaveStatusApproved}).
		Where("NOT (start_date > ? OR end_date < ?)", input.EndDate, input.StartDate).
		Count(&count)

	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "This employee already has a leave request during this period"})
		return
	}

	targetEmployee, ok := repository.ResolveEmployeeForCompany(c, currentUser, dataScope, input.EmployeesID, input.CompaniesID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid employee or access denied"})
		return
	}

	var leaveType models.LeaveTypeSetting
	if err := config.DB.Scopes(
		repository.ApplyCompanyScopes(c, currentUser, "leave_type_setting.companies_id"),
	).Where("id = ?", input.LeaveTypeID).First(&leaveType).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid leave type for your company"})
		return
	}

	if leaveType.CompaniesID != targetEmployee.CompaniesID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Leave type must belong to employee company"})
		return
	}

	totalDays := float64(int(input.EndDate.Sub(input.StartDate).Hours()/24)) + 1

	updates := map[string]interface{}{
		"employees_id":  input.EmployeesID,
		"leave_type_id": input.LeaveTypeID,
		"leave_type":    models.LeaveType(leaveType.Name),
		"start_date":    input.StartDate,
		"end_date":      input.EndDate,
		"total_days":    totalDays,
		"reason":        input.Reason,
		"companies_id":  targetEmployee.CompaniesID,
		"updated_at":    time.Now(),
	}

	if err := config.DB.Model(&leave).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := config.DB.Preload("Employees").Preload("Employees.Companies").Preload("Approver").Preload("LeaveTypeMaster").First(&leave, leave.ID).Error; err != nil {
		c.JSON(http.StatusOK, leave)
		return
	}

	c.JSON(http.StatusOK, leave)
}

func DeleteLeaveRequest(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	// Ensure user has permission to delete this specific record (English Comment)
	result := config.DB.Scopes(
		repository.ApplyEmployeeScopes(c, currentUser, dataScope, "leave_request.companies_id", "leave_request.employees_id"),
	).Where("id = ?", id).Delete(&models.LeaveRequest{})

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

// pendingLeaveResponse decorates a pending request with yearly quota usage so
// the approval page can warn (not block) when the employee would exceed
// MaxDaysPerYear of the leave type.
type pendingLeaveResponse struct {
	models.LeaveRequest
	QuotaMaxDays  float64 `json:"quota_max_days"`
	QuotaUsedDays float64 `json:"quota_used_days"`
	QuotaExceeded bool    `json:"quota_exceeded"`
}

func GetPendingLeaveRequests(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	var leaves []models.LeaveRequest
	// filter only 'Pending' status for approval page (English Comment)
	err := config.DB.Scopes(
		repository.ApplyEmployeeScopes(c, currentUser, dataScope, "leave_request.companies_id", "leave_request.employees_id"),
	).Preload("Employees").Preload("Employees.Companies").Preload("LeaveTypeMaster").Where("status = ?", models.LeaveStatusPending).Find(&leaves).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responses := make([]pendingLeaveResponse, 0, len(leaves))
	for _, leave := range leaves {
		resp := pendingLeaveResponse{LeaveRequest: leave}
		if leave.LeaveTypeID != nil && leave.LeaveTypeMaster != nil && leave.LeaveTypeMaster.MaxDaysPerYear > 0 {
			resp.QuotaMaxDays = leave.LeaveTypeMaster.MaxDaysPerYear

			var used float64
			config.DB.Model(&models.LeaveRequest{}).
				Where("employees_id = ? AND leave_type_id = ? AND status = ?", leave.EmployeesID, *leave.LeaveTypeID, models.LeaveStatusApproved).
				Where("EXTRACT(YEAR FROM start_date) = ?", leave.StartDate.Year()).
				Select("COALESCE(SUM(total_days), 0)").
				Scan(&used)

			resp.QuotaUsedDays = used
			resp.QuotaExceeded = used+leave.TotalDays > resp.QuotaMaxDays
		}
		responses = append(responses, resp)
	}
	c.JSON(http.StatusOK, responses)
}

func ApproveLeaveRequest(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var approver models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&approver, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var input struct {
		Status     string `json:"status" binding:"required,oneof=Approved Rejected"`
		ApprovedBy uint   `json:"approved_by"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var leave models.LeaveRequest
	if err := config.DB.Scopes(
		repository.ApplyEmployeeScopes(c, approver, dataScope, "leave_request.companies_id", "leave_request.employees_id"),
	).First(&leave, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Leave request not found or access denied"})
		return
	}

	if leave.Status != models.LeaveStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This request has already been processed"})
		return
	}

	updates := map[string]interface{}{
		"status":      input.Status,
		"approved_by": input.ApprovedBy,
		"updated_at":  time.Now(),
	}

	if err := config.DB.Model(&leave).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Status updated to " + input.Status})
}

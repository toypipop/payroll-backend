package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// leaveTypeService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func leaveTypeService() *service.LeaveTypeSettingService {
	return service.NewLeaveTypeSettingService(repository.NewLeaveTypeSettingRepository())
}

func GetLeaveTypeSettings(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	leaveTypes, err := leaveTypeService().List(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, leaveTypes)
}

// GetLeaveTypeOptions returns a minimal id/name list for dropdowns. Ungated
// by page permission (mirrors GetCompanies) so any role submitting a leave
// request can see whether leave types exist for a company.
func GetLeaveTypeOptions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	leaveTypes, err := leaveTypeService().Options(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	type option struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		CompaniesID uint   `json:"companies_id"`
	}
	options := make([]option, 0, len(leaveTypes))
	for _, lt := range leaveTypes {
		options = append(options, option{ID: lt.ID, Name: lt.Name, CompaniesID: lt.CompaniesID})
	}
	c.JSON(http.StatusOK, options)
}

func CreateLeaveTypeSetting(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.LeaveTypeSetting
	if !httpx.BindJSON(c, &input) {
		return
	}

	leaveType, err := leaveTypeService().Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, leaveType)
}

func UpdateLeaveTypeSetting(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "leave type")
	if !ok {
		return
	}

	// companies_id is not accepted here: the company is immutable after creation.
	var input models.LeaveTypeSetting
	if !httpx.BindJSON(c, &input) {
		return
	}

	leaveType, err := leaveTypeService().Update(scope, id, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, leaveType)
}

func DeleteLeaveTypeSetting(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "leave type")
	if !ok {
		return
	}

	if err := leaveTypeService().Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Leave type deleted"})
}

package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetLeaveTypeSettings(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	query, ok := httpx.ParsePage(c)
	if !ok {
		return
	}
	leaveTypes, err := h.LeaveType.List(scope, query)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, leaveTypes)
}

// GetLeaveTypeOptions returns a minimal id/name list for dropdowns.
func (h *Handler) GetLeaveTypeOptions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	leaveTypes, err := h.LeaveType.Options(scope)
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

func (h *Handler) CreateLeaveTypeSetting(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.LeaveTypeSetting
	if !httpx.BindJSON(c, &input) {
		return
	}

	leaveType, err := h.LeaveType.Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, leaveType)
}

func (h *Handler) UpdateLeaveTypeSetting(c *gin.Context) {
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

	leaveType, err := h.LeaveType.Update(scope, id, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, leaveType)
}

func (h *Handler) DeleteLeaveTypeSetting(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "leave type")
	if !ok {
		return
	}

	if err := h.LeaveType.Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Leave type deleted"})
}

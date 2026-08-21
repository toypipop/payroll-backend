package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

type employeeCompanyAccessInput struct {
	EmployeeID uint `json:"employee_id"`
	CompanyID  uint `json:"company_id"`
	RoleID     uint `json:"role_id"`
	IsActive   bool `json:"is_active"`
}

// toServiceInput converts the wire shape into the service's input type.
func (i employeeCompanyAccessInput) toServiceInput() service.EmployeeCompanyAccessInput {
	return service.EmployeeCompanyAccessInput{
		EmployeeID: i.EmployeeID,
		CompanyID:  i.CompanyID,
		RoleID:     i.RoleID,
		IsActive:   i.IsActive,
	}
}

func (h *Handler) GetEmployeeCompanyAccess(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	mappings, err := h.EmployeeCompanyAccess.List(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, mappings)
}

func (h *Handler) CreateEmployeeCompanyAccess(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input employeeCompanyAccessInput
	if !httpx.BindJSON(c, &input) {
		return
	}

	mapping, err := h.EmployeeCompanyAccess.Create(scope, input.toServiceInput())
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, mapping)
}

func (h *Handler) UpdateEmployeeCompanyAccess(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "access mapping")
	if !ok {
		return
	}

	var input employeeCompanyAccessInput
	if !httpx.BindJSON(c, &input) {
		return
	}

	mapping, err := h.EmployeeCompanyAccess.Update(scope, id, input.toServiceInput())
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, mapping)
}

func (h *Handler) DeleteEmployeeCompanyAccess(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "access mapping")
	if !ok {
		return
	}

	if err := h.EmployeeCompanyAccess.Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

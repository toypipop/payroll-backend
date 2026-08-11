package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// employeeCompanyAccessService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func employeeCompanyAccessService() *service.EmployeeCompanyAccessService {
	return service.NewEmployeeCompanyAccessService(
		repository.NewEmployeeCompanyAccessRepository(),
		repository.NewRoleRepository(),
	)
}

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

func GetEmployeeCompanyAccess(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	mappings, err := employeeCompanyAccessService().List(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, mappings)
}

func CreateEmployeeCompanyAccess(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input employeeCompanyAccessInput
	if !httpx.BindJSON(c, &input) {
		return
	}

	mapping, err := employeeCompanyAccessService().Create(scope, input.toServiceInput())
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, mapping)
}

func UpdateEmployeeCompanyAccess(c *gin.Context) {
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

	mapping, err := employeeCompanyAccessService().Update(scope, id, input.toServiceInput())
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, mapping)
}

func DeleteEmployeeCompanyAccess(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "access mapping")
	if !ok {
		return
	}

	if err := employeeCompanyAccessService().Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

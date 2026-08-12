package employees

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// employeeEarningService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func employeeEarningService() *service.EmployeeEarningService {
	return service.NewEmployeeEarningService(
		repository.NewEmployeeEarningRepository(),
		repository.NewEmployeesRepository(),
		repository.NewEarningTypeRepository(),
	)
}

func GetEmployeeEarnings(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	items, err := employeeEarningService().List(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func CreateEmployeeEarning(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.EmployeeEarning
	if !httpx.BindJSON(c, &input) {
		return
	}

	earning, err := employeeEarningService().Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, earning)
}

func UpdateEmployeeEarning(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "employee earning")
	if !ok {
		return
	}

	var input models.EmployeeEarning
	if !httpx.BindJSON(c, &input) {
		return
	}

	earning, err := employeeEarningService().Update(scope, id, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, earning)
}

func DeleteEmployeeEarning(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "employee earning")
	if !ok {
		return
	}

	if err := employeeEarningService().Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

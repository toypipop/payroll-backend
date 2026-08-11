package employees

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// employeesService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func employeesService() *service.EmployeesService {
	return service.NewEmployeesService(repository.NewEmployeesRepository())
}

func GetEmployees(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	employees, err := employeesService().List(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, employees)
}

func CreateEmployees(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.Employees
	if !httpx.BindJSON(c, &input) {
		return
	}

	created, err := employeesService().Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, created)
}

func UpdateEmployees(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "employee")
	if !ok {
		return
	}

	var patch map[string]any
	if !httpx.BindJSON(c, &patch) {
		return
	}

	updated, err := employeesService().Update(scope, id, patch)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, updated)
}

func DeleteEmployees(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "employee")
	if !ok {
		return
	}

	if err := employeesService().Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

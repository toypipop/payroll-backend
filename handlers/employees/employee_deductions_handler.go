package employees

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// employeeDeductionService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func employeeDeductionService() *service.EmployeeDeductionService {
	return service.NewEmployeeDeductionService(
		repository.NewEmployeeDeductionRepository(),
		repository.NewEmployeesRepository(),
		repository.NewDeductionTypeRepository(),
	)
}

func GetEmployeeDeductions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	items, err := employeeDeductionService().List(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func CreateEmployeeDeduction(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.EmployeeDeduction
	if !httpx.BindJSON(c, &input) {
		return
	}

	deduction, err := employeeDeductionService().Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, deduction)
}

func UpdateEmployeeDeduction(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "employee deduction")
	if !ok {
		return
	}

	var input models.EmployeeDeduction
	if !httpx.BindJSON(c, &input) {
		return
	}

	deduction, err := employeeDeductionService().Update(scope, id, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, deduction)
}

func DeleteEmployeeDeduction(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "employee deduction")
	if !ok {
		return
	}

	if err := employeeDeductionService().Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

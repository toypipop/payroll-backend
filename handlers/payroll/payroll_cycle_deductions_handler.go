package payroll

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// cycleDeductionService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func cycleDeductionService() *service.CycleDeductionService {
	return service.NewCycleDeductionService(
		repository.NewCycleDeductionRepository(),
		repository.NewPayrollRunRepository(),
		repository.NewEmployeesRepository(),
		repository.NewDeductionTypeRepository(),
	)
}

func GetCycleDeductions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	items, err := cycleDeductionService().List(scope, queryCycleID(c))
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func CreateCycleDeduction(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.PayrollCycleDeduction
	if !httpx.BindJSON(c, &input) {
		return
	}

	item, err := cycleDeductionService().Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func UpdateCycleDeduction(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "cycle deduction")
	if !ok {
		return
	}

	var input models.PayrollCycleDeduction
	if !httpx.BindJSON(c, &input) {
		return
	}

	item, err := cycleDeductionService().Update(scope, id, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func DeleteCycleDeduction(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "cycle deduction")
	if !ok {
		return
	}

	if err := cycleDeductionService().Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

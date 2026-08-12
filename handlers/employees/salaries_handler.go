package employees

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// UpdateSalariesStructure either disables an employee's current salary or
// supersedes it with a new record. The archive-or-erase decision for the outgoing
// records lives in the service; see SalariesStructureService.
func UpdateSalariesStructure(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input struct {
		EmployeesID   uint    `json:"employees_id"`
		CompaniesID   uint    `json:"companies_id"`
		BaseSalaries  float64 `json:"base_salaries"`
		EffectiveDate string  `json:"effective_date"`
		DeletedAt     *bool   `json:"deleted_at"`
	}
	if !httpx.BindJSON(c, &input) {
		return
	}

	result, err := salariesStructureService().Save(scope, service.SalaryInput{
		EmployeesID:   input.EmployeesID,
		CompaniesID:   input.CompaniesID,
		BaseSalaries:  input.BaseSalaries,
		EffectiveDate: input.EffectiveDate,
		Disable:       input.DeletedAt != nil && *input.DeletedAt,
	})
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	if result.Disabled {
		c.JSON(http.StatusOK, gin.H{
			"message": "Salaries structure disabled successfully",
			"data":    nil,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Salaries structure saved successfully"})
}

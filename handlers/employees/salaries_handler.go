package employees

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// UpdateSalariesStructure either disables an employee's current salary or supersedes it with a new record.
func (h *Handler) UpdateSalariesStructure(c *gin.Context) {
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
		// Version is the record the form was seeded from; the save is refused if that record has moved since.
		Version uint `json:"version"`
		// Reason is why the change is being made, and every salary write carries one.
		Reason string `json:"reason"`
	}
	if !httpx.BindJSON(c, &input) {
		return
	}

	result, err := h.SalariesStructure.Save(scope, service.SalaryInput{
		EmployeesID:   input.EmployeesID,
		CompaniesID:   input.CompaniesID,
		BaseSalaries:  input.BaseSalaries,
		EffectiveDate: input.EffectiveDate,
		Disable:       input.DeletedAt != nil && *input.DeletedAt,
		Version:       input.Version,
		Reason:        input.Reason,
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

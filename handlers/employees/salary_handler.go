package employees

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// salariesStructureService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func salariesStructureService() *service.SalariesStructureService {
	return service.NewSalariesStructureService(
		repository.NewSalariesStructureRepository(),
		repository.NewEmployeesRepository(),
	)
}

// GetSalaries returns all employees in scope with their salary structure (nil if not yet assigned).
func GetSalaries(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	employees, err := salariesStructureService().ListEmployees(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, employees)
}

// GetSalaryHistory returns every salary record for one employee (current +
func GetSalaryHistory(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	employeesID, ok := httpx.ParseID(c, "employees_id", "employee")
	if !ok {
		return
	}

	entries, err := salariesStructureService().History(scope, employeesID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	type historyRow struct {
		models.SalariesStructure
		CanDelete bool `json:"can_delete"`
	}
	rows := make([]historyRow, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, historyRow{SalariesStructure: entry.Record, CanDelete: entry.CanDelete})
	}
	c.JSON(http.StatusOK, rows)
}

// DeleteSalaryRecord erases a salary record, but only one that no closed cycle ever used.
func DeleteSalaryRecord(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "salary record")
	if !ok {
		return
	}

	if err := salariesStructureService().DeleteRecord(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Salary record deleted successfully"})
}

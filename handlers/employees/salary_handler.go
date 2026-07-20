package employees

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

// GetSalaries returns all employees in scope with their salary structure (nil if not yet assigned).
// Used by the SalariesStructurePage to list employees and allow salary assignment.
func GetSalaries(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var employees []models.Employees
	if err := config.DB.
		Scopes(repository.ApplyContextScopes(c, currentUser, dataScope, "employees.companies_id", "employees.id")).
		Preload("Companies").
		Preload("Department").
		Preload("Position").
		Preload("SalariesStructure", "is_active = ?", true).
		Find(&employees).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, employees)
}

// GetSalaryHistory returns every salary record for one employee (current +
// preserved/archived), excluding records that were truly deleted (never used
// in a real calculation — those don't count as history at all).
func GetSalaryHistory(c *gin.Context) {
	employeesID := c.Param("employees_id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var employee models.Employees
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "employees.companies_id", "employees.id"),
	).Select("id, companies_id").First(&employee, employeesID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employee not found or access denied"})
		return
	}

	var records []models.SalariesStructure
	if err := config.DB.Where("employees_id = ?", employeesID).Order("effective_date DESC, id DESC").Find(&records).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type historyRow struct {
		models.SalariesStructure
		CanDelete bool `json:"can_delete"`
	}
	rows := make([]historyRow, 0, len(records))
	for _, rec := range records {
		usedCount, _ := usedInClosedCycleCount(config.DB, rec.ID)
		rows = append(rows, historyRow{SalariesStructure: rec, CanDelete: usedCount == 0})
	}

	c.JSON(http.StatusOK, rows)
}

// DeleteSalaryRecord permanently removes a salary record, but only if it was
// never actually used in a real payroll calculation — once used, a record is
// kept forever for audit and can only be superseded/disabled, never deleted.
func DeleteSalaryRecord(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var record models.SalariesStructure
	if err := config.DB.First(&record, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Salary record not found"})
		return
	}

	if !repository.CanAccessCompany(c, currentUser.ID, dataScope, record.CompaniesID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	usedCount, err := usedInClosedCycleCount(config.DB, record.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if usedCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete — this record was already used in a closed payroll cycle's calculation. Disable it instead if it should no longer apply."})
		return
	}

	if err := config.DB.Delete(&record).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete salary record"})
		return
	}

	// If the deleted record was the current one, restore whichever record is
	// now the most recent survivor — deleting a mistake should roll the
	// employee back to the state before that mistake, not leave them with no
	// active salary at all.
	if record.IsActive {
		var previous models.SalariesStructure
		if err := config.DB.Where("employees_id = ?", record.EmployeesID).
			Order("effective_date DESC, id DESC").
			First(&previous).Error; err == nil {
			config.DB.Model(&models.SalariesStructure{}).Where("id = ?", previous.ID).Update("is_active", true)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Salary record deleted successfully"})
}

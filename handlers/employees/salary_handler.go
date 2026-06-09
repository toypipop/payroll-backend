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
		Preload("SalariesStructure").
		Find(&employees).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, employees)
}

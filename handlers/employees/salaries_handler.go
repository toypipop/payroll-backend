package employees

import (
	"net/http"
	"time"

	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

func UpdateSalariesStructure(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var input struct {
		EmployeesID   uint    `json:"employees_id" binding:"required"`
		BaseSalaries  float64 `json:"base_salaries"`
		EffectiveDate string  `json:"effective_date" binding:"required"`
		IsEnabled     bool    `json:"is_enabled"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	parsedDate, err := time.Parse("2006-01-02", input.EffectiveDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format. Expected YYYY-MM-DD"})
		return
	}

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var emp models.Employees
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "employees.companies_id", "employees.id"),
	).Where("id = ?", input.EmployeesID).First(&emp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employees not found or access denied"})
		return
	}

	salaries := models.SalariesStructure{
		EmployeesID:   input.EmployeesID,
		BaseSalaries:  input.BaseSalaries,
		EffectiveDate: parsedDate,
		IsEnabled:     input.IsEnabled,
	}

	if err := config.DB.Create(&salaries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create salaries record"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Salaries structure updated successfully",
		"data":    salaries,
	})
}

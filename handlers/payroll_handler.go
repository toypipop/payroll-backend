package handlers

import (
	"net/http"
	"time"

	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

func UpdateSalariesStructure(c *gin.Context) {
	companiesID, _ := c.Get("companies_id")

	// Input struct uses string for date to prevent parsing error from React (English Comment)
	var input struct {
		EmployeesID   uint    `json:"Employees_id" binding:"required"`
		BaseSalaries  float64 `json:"base_salaries"`
		EffectiveDate string  `json:"effective_date" binding:"required"`
		IsEnabled     bool    `json:"is_enabled"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse date string to time.Time for database compatibility (English Comment)
	parsedDate, err := time.Parse("2006-01-02", input.EffectiveDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format. Expected YYYY-MM-DD"})
		return
	}

	// 1. Validation: Ensure the Employees belongs to the user's company (English Comment)
	var emp models.Employees
	if err := config.DB.Scopes(repository.ScopeCompanies(companiesID.(uint))).
		Where("id = ?", input.EmployeesID).First(&emp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employees not found or unauthorized"})
		return
	}

	// 2. Prepare Salaries record using your specific model name (English Comment)
	salaries := models.SalariesStructure{
		EmployeesID:   input.EmployeesID,
		BaseSalaries:  input.BaseSalaries,
		EffectiveDate: parsedDate,
		IsEnabled:     input.IsEnabled,
	}

	// 3. Upsert: Update if exists using the singular/plural columns you defined (English Comment)
	err = config.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "employees_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"base_salaries":  salaries.BaseSalaries,
			"effective_date": salaries.EffectiveDate,
			"is_enabled":     salaries.IsEnabled,
			"updated_at":     time.Now(),
		}),
	}).Create(&salaries).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update salaries information"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Salaries structure updated successfully",
		"data":    salaries,
	})
}

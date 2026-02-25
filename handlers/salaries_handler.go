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
	// 1. Get Auth Context (English Comment)
	userID, dataScope := repository.GetContextAuth(c)

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

	parsedDate, err := time.Parse("2006-01-02", input.EffectiveDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format. Expected YYYY-MM-DD"})
		return
	}

	// 2. Retrieve currentUser for scoping (English Comment)
	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	// 3. Validation: Use ApplyScopes to ensure the target Employee is within allowed access (English Comment)
	var emp models.Employees
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "employees.companies_id", "employees.id"),
	).Where("id = ?", input.EmployeesID).First(&emp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employees not found or access denied"})
		return
	}

	// 4. Prepare Salaries record (English Comment)
	salaries := models.SalariesStructure{
		EmployeesID:   input.EmployeesID,
		BaseSalaries:  input.BaseSalaries,
		EffectiveDate: parsedDate,
		IsEnabled:     input.IsEnabled,
	}

	// 5. Upsert operation (English Comment)
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

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

// UpdateSalariesStructure - Update or Create salaries with ownership validation (English Comment)
func UpdateSalariesStructure(c *gin.Context) {
	companiesID, _ := c.Get("companies_id")

	var input struct {
		EmployeeID    uint      `json:"employee_id" binding:"required"`
		BaseSalaries    float64   `json:"base_salaries"`
		EffectiveDate time.Time `json:"effective_date" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. Validation: Ensure the employee belongs to the user's company (English Comment)
	var emp models.Employee
	if err := config.DB.Scopes(repository.ScopeCompanies(companiesID.(uint))).
		Where("id = ?", input.EmployeeID).First(&emp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employee not found or unauthorized"})
		return
	}

	// 2. Prepare Salaries record (English Comment)
	salaries := models.SalariesStructure{
		EmployeeID:    input.EmployeeID,
		BaseSalaries:    input.BaseSalaries,
		EffectiveDate: input.EffectiveDate,
	}

	// 3. Upsert: Update if exists, otherwise create (English Comment)
	err := config.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "employee_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"base_salaries", "effective_date", "updated_at"}),
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

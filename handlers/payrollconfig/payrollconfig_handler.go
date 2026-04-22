package payrollconfig

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetPayrollConfigs(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var companies []models.Companies

	// Fetch latest active config per company
	err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "companies.id", "unused"),
	).Preload("PayrollConfig", func(db *gorm.DB) *gorm.DB {
		return db.Where("deleted_at IS NULL").Order("created_at DESC").Limit(1)
	}).Find(&companies).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch payroll configuration"})
		return
	}

	c.JSON(http.StatusOK, companies)
}

// UpdatePayrollConfig saves new config with soft delete versioning
func UpdatePayrollConfig(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var input models.PayrollConfig
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	// Validate user company access
	var targetCompany models.Companies
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "companies.id", "unused"),
	).Where("id = ?", input.CompaniesID).First(&targetCompany).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for this Companie"})
		return
	}

	// Soft delete old config versions
	if err := config.DB.Where("companies_id = ? AND deleted_at IS NULL", input.CompaniesID).Delete(&models.PayrollConfig{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to archive previous configuration"})
		return
	}

	// Insert new config as active version
	input.ID = 0
	input.CreatedAt = time.Now()
	input.UpdatedAt = time.Now()

	if err := config.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Configuration saved", "data": input})
}

// GetConfigByPayrollCycle fetches config by cycle
func GetConfigByPayrollCycle(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	cycleID := c.Param("cycle_id")

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	// Fetch cycle for company ID
	var cycle models.PayrollCycle
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "payroll_cycle.companies_id", "unused"),
	).Preload("Companies").First(&cycle, cycleID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Payroll cycle not found or access denied"})
		return
	}

	// Fetch latest non-deleted config
	var payrollConfig models.PayrollConfig
	if err := config.DB.
		Where("companies_id = ? AND deleted_at IS NULL", cycle.CompaniesID).
		Order("created_at DESC").
		First(&payrollConfig).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No active payroll config found for this cycle's company"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config":       payrollConfig,
		"cycle_id":     cycle.ID,
		"companies_id": cycle.CompaniesID,
	})
}

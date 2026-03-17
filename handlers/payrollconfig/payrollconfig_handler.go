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

	// LATEST ACTIVE VERSION: Fetch companies with their LATEST non-deleted config
	// Use a custom preload hook to ensure only the active (latest) version is loaded
	err := config.DB.Debug().Scopes(
		repository.ApplyScopes(currentUser, dataScope, "companies.id", "unused"),
	).Preload("PayrollConfig", func(db *gorm.DB) *gorm.DB {
		// Ensure preload returns ONLY the latest non-deleted config for each company
		return db.Where("deleted_at IS NULL").Order("created_at DESC").Limit(1)
	}).Find(&companies).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, companies)
}

// UpdatePayrollConfig saves a new version of the payroll config.
// APPROACH B: Versioning with Soft Delete
// When saving new config for a company, soft-delete the old one and create a new row.
// This ensures:
// 1. Audit trail is preserved (old versions exist but marked deleted)
// 2. Latest version is always active (newest non-deleted row)
// 3. No data accumulation/duplication issues
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

	// Validation: Ensure user has access to the target Companie
	var targetCompany models.Companies
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "companies.id", "unused"),
	).Where("id = ?", input.CompaniesID).First(&targetCompany).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for this Companie"})
		return
	}

	// SOFT DELETE: Mark any existing active config for this company as deleted
	// This creates a new version while preserving the old one for audit trail
	if err := config.DB.Where("companies_id = ? AND deleted_at IS NULL", input.CompaniesID).Delete(&models.PayrollConfig{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to archive previous configuration"})
		return
	}

	// CREATE: Insert the new config as the active version
	// Note: Set ID to 0 to ensure a new row is created (not updated)
	input.ID = 0
	input.CreatedAt = time.Now()
	input.UpdatedAt = time.Now()

	if err := config.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Configuration saved", "data": input})
}

// GetConfigByPayrollCycle fetches the payroll configuration for a specific cycle.
// CRITICAL: Filters by the Cycle's Company ID, NOT the current user's company.
// This ensures the Run Payroll page always uses the correct company's SSO parameters.
func GetConfigByPayrollCycle(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	cycleID := c.Param("cycle_id")

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	// Fetch the cycle to get its company ID
	var cycle models.PayrollCycle
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "payroll_cycle.companies_id", "unused"),
	).Preload("Companies").First(&cycle, cycleID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Payroll cycle not found or access denied"})
		return
	}

	// LATEST ACTIVE VERSION: Fetch the newest NON-DELETED config for this company
	// Query ensures:
	// 1. ONLY the cycle's company config (not user's company)
	// 2. ONLY non-deleted rows (deleted_at IS NULL)
	// 3. LATEST version (ORDER BY created_at DESC LIMIT 1)
	var payrollConfig models.PayrollConfig
	if err := config.DB.
		Where("companies_id = ? AND deleted_at IS NULL", cycle.CompaniesID).
		Order("created_at DESC").
		First(&payrollConfig).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No active payroll config found for this cycle's company"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config": payrollConfig,
		"cycle_id": cycle.ID,
		"companies_id": cycle.CompaniesID,
	})
}

package payroll

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

func GetPayrollConfigs(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var companies []models.Companies

	// Start from Companies Table to show all companies in the scope (English Comment)
	err := config.DB.Scopes(
		repository.ApplyCompanyScopes(c, currentUser, "companies.id"),
	).Order("companies.id ASC").Preload("PayrollConfig").Find(&companies).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch payroll configuration"})
		return
	}

	c.JSON(http.StatusOK, companies)
}

func UpdatePayrollConfig(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)

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

	// Validation: Ensure user has access to the target Companie (English Comment)
	// We check if the input.CompaniesID is within the allowed scope
	var targetCompany models.Companies
	if err := config.DB.Scopes(
		repository.ApplyCompanyScopes(c, currentUser, "companies.id"),
	).Where("id = ?", input.CompaniesID).First(&targetCompany).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for this Companie"})
		return
	}

	// Upsert: One Companie has only one config (English Comment)
	err := config.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "companies_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"sso_rate":        input.SsoRate,
			"sso_max_amount":  input.SsoMaxAmount,
			"sso_min_salary":  input.SsoMinSalary,
			"tax_calculation": input.TaxCalculation,
			"updated_at":      time.Now(),
		}),
	}).Create(&input).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Configuration saved", "data": input})
}

package handlers

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository" // Import your repository package
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func parseUint(s string) uint {
	val, _ := strconv.ParseUint(s, 10, 32)
	return uint(val)
}

// GetRolePermissions fetches all modules and current access for a specific role
func GetRolePermissions(c *gin.Context) {
	roleID := c.Param("id")

	// Get companies_id from middleware context (English Comment)
	valID, _ := c.Get("companies_id")
	compID := valID.(uint)

	// 1. Get all modules first (AppModules are typically global/master data) (English Comment)
	var modules []models.AppModule
	config.DB.Order("`order` asc").Find(&modules)

	// 2. Get existing access for this role filtered by companies (English Comment)
	// Even if we search by roleID, ScopeCompanies ensures the role belongs to the companies.
	var access []models.RoleModuleAccess
	if err := config.DB.Scopes(repository.ScopeCompanies(compID)).
		Where("role_id = ?", roleID).
		Find(&access).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch access"})
		return
	}

	// 3. Map access for easier frontend consumption (English Comment)
	c.JSON(http.StatusOK, gin.H{
		"modules": modules,
		"access":  access,
	})
}

// UpdateRolePermissions updates or creates permissions for a role
func UpdateRolePermissions(c *gin.Context) {
	roleID := c.Param("id")

	// Get companies_id from middleware context (English Comment)
	valID, _ := c.Get("companies_id")
	compID := valID.(uint)

	var input []models.RoleModuleAccess
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Use transaction to ensure data consistency (English Comment)
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		// 1. Clear old permissions for this role within the companies scope (English Comment)
		if err := tx.Scopes(repository.ScopeCompanies(compID)).
			Where("role_id = ?", roleID).
			Delete(&models.RoleModuleAccess{}).Error; err != nil {
			return err
		}

		// 2. Insert new permissions (English Comment)
		if len(input) > 0 {
			rID := parseUint(roleID)
			for i := range input {
				// Ensure all records match current Role and Companies (English Comment)
				input[i].RoleID = rID
				input[i].CompaniesID = compID
			}
			if err := tx.Create(&input).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update permissions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Permissions updated successfully"})
}

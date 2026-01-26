package handlers

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
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

	// 1. Get all modules first (English Comment)
	var modules []models.AppModule
	config.DB.Order("`order` asc").Find(&modules)

	// 2. Get existing access for this role (English Comment)
	var access []models.RoleModuleAccess
	config.DB.Where("role_id = ?", roleID).Find(&access)

	// 3. Map access for easier frontend consumption (English Comment)
	c.JSON(http.StatusOK, gin.H{
		"modules": modules,
		"access":  access,
	})
}

// UpdateRolePermissions updates or creates permissions for a role
func UpdateRolePermissions(c *gin.Context) {
	roleID := c.Param("id")
	var input []models.RoleModuleAccess

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Use transaction to ensure data consistency (English Comment)
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		// 1. Clear old permissions for this role (English Comment)
		if err := tx.Where("role_id = ?", roleID).Delete(&models.RoleModuleAccess{}).Error; err != nil {
			return err
		}

		// 2. Insert new permissions (English Comment)
		if len(input) > 0 {
			for i := range input {
				// Force RoleID to match URL param (English Comment)
				input[i].RoleID = uint(parseUint(roleID))
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

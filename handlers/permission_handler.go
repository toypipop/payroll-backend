package handlers

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func parseUint(s string) uint {
	val, _ := strconv.ParseUint(s, 10, 32)
	return uint(val)
}

func GetRolesPermissions(c *gin.Context) {
	rolesID := c.Param("id")

	// Use GetContextAuth to retrieve userID and dataScope
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var modules []models.AppModule
	config.DB.Order("`order` asc").Find(&modules)

	var access []models.RolesModuleAccess
	// Apply composite scopes for tenant and data access
	if err := config.DB.Scopes(repository.ApplyScopes(currentUser, dataScope, "roles_module_accesses.companies_id", "unused")).
		Where("roles_id = ?", rolesID).
		Find(&access).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch access"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"modules": modules,
		"access":  access,
	})
}

type UpdatePermissionRequest struct {
	ModuleID  uint             `json:"module_id" binding:"required"`
	CanView   bool             `json:"can_view"`
	CanCreate bool             `json:"can_create"`
	CanEdit   bool             `json:"can_edit"`
	CanDelete bool             `json:"can_delete"`
	DataScope models.DataScope `json:"data_scope" binding:"required,oneof=Own Department Companies All"`
}

func UpdateRolesPermissions(c *gin.Context) {
	rolesID := c.Param("id")

	// Use GetContextAuth to retrieve core info
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var input []UpdatePermissionRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := config.DB.Transaction(func(tx *gorm.DB) error {
		// Apply Scopes to ensure the user only deletes permissions within their authorized companies/access
		if err := tx.Scopes(repository.ApplyScopes(currentUser, dataScope, "roles_module_accesses.companies_id", "unused")).
			Where("roles_id = ?", rolesID).
			Delete(&models.RolesModuleAccess{}).Error; err != nil {
			return err
		}

		if len(input) > 0 {
			rID := parseUint(rolesID)
			var newPermissions []models.RolesModuleAccess

			for _, item := range input {
				newPermissions = append(newPermissions, models.RolesModuleAccess{
					RolesID:     rID,
					CompaniesID: currentUser.CompaniesID, // Use Companies ID from validated currentUser
					ModuleID:    item.ModuleID,
					CanView:     item.CanView,
					CanCreate:   item.CanCreate,
					CanEdit:     item.CanEdit,
					CanDelete:   item.CanDelete,
					DataScope:   item.DataScope,
				})
			}

			if err := tx.Create(&newPermissions).Error; err != nil {
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

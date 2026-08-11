package settings

import (
	"errors"
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func parseUint(s string) uint {
	val, _ := strconv.ParseUint(s, 10, 32)
	return uint(val)
}

func GetRolesPermissions(c *gin.Context) {
	rolesID := c.Param("id")

	// Use GetContextAuth to retrieve userID and dataScope
	userID, _ := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var modules []models.AppModule
	// Let GORM quote the reserved column name for PostgreSQL instead of hand-writing SQL.
	config.DB.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}}).Find(&modules)

	var access []models.RolesModuleAccess
	// Apply composite scopes for tenant and data access
	if err := config.DB.Scopes(repository.ApplyCompanyScopes(c, currentUser, "roles_module_access.companies_id")).
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

// errUnknownModule marks a payload referencing a module row that does not
// exist, or one with no capability declaration. Both are client/config errors
// rather than server faults, so the caller maps them to 400.
var errUnknownModule = errors.New("permission payload references an unknown module")

// capabilitiesForRequest resolves every module in the payload to its declared
// capability in one query, so the write loop stays free of per-item lookups.
func capabilitiesForRequest(tx *gorm.DB, input []UpdatePermissionRequest) (map[uint]models.ModuleCapability, error) {
	moduleIDs := make([]uint, 0, len(input))
	for _, item := range input {
		moduleIDs = append(moduleIDs, item.ModuleID)
	}

	var modules []models.AppModule
	if err := tx.Select("id, module_key").Where("id IN ?", moduleIDs).Find(&modules).Error; err != nil {
		return nil, err
	}

	capabilities := make(map[uint]models.ModuleCapability, len(modules))
	for _, module := range modules {
		capability, ok := models.CapabilityFor(module.ModuleKey)
		if !ok {
			return nil, errUnknownModule
		}
		capabilities[module.ID] = capability
	}

	for _, item := range input {
		if _, ok := capabilities[item.ModuleID]; !ok {
			return nil, errUnknownModule
		}
	}
	return capabilities, nil
}

func UpdateRolesPermissions(c *gin.Context) {
	rolesID := c.Param("id")

	// Use GetContextAuth to retrieve core info
	userID, _ := repository.GetContextAuth(c)

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
	rID := parseUint(rolesID)
	var role models.Roles
	if err := config.DB.Scopes(repository.ApplyCompanyScopes(c, currentUser, "roles.companies_id")).
		First(&role, rID).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for this role"})
		return
	}

	err := config.DB.Transaction(func(tx *gorm.DB) error {
		// Apply Scopes to ensure the user only deletes permissions within their authorized companies/access
		if err := tx.Scopes(repository.ApplyCompanyScopes(c, currentUser, "roles_module_access.companies_id")).
			Where("roles_id = ?", rolesID).
			Delete(&models.RolesModuleAccess{}).Error; err != nil {
			return err
		}

		if len(input) > 0 {
			capabilities, capErr := capabilitiesForRequest(tx, input)
			if capErr != nil {
				return capErr
			}

			var newPermissions []models.RolesModuleAccess

			for _, item := range input {
				capability := capabilities[item.ModuleID]

				// Store only what the backend will actually honour. A module
				// whose rows have no employee column cannot answer Own or
				// Department, and an action no route gates on can never fire —
				// persisting either would put a claim in the database that the
				// permission check silently ignores, which is the exact defect
				// this capability model exists to remove.
				scope := item.DataScope
				if !capability.AllowsScope(scope) {
					scope = models.ScopeCompanies
				}

				newPermissions = append(newPermissions, models.RolesModuleAccess{
					RolesID:     rID,
					CompaniesID: role.CompaniesID,
					ModuleID:    item.ModuleID,
					CanView:     item.CanView && capability.AllowsAction(models.ActionView),
					CanCreate:   item.CanCreate && capability.AllowsAction(models.ActionCreate),
					CanEdit:     item.CanEdit && capability.AllowsAction(models.ActionEdit),
					CanDelete:   item.CanDelete && capability.AllowsAction(models.ActionDelete),
					DataScope:   scope,
				})
			}

			if err := tx.Create(&newPermissions).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		if errors.Is(err, errUnknownModule) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Permission payload references an unknown module"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update permissions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Permissions updated successfully"})
}

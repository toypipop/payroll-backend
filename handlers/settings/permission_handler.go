package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// permissionService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func permissionService() *service.PermissionService {
	return service.NewPermissionService(repository.NewPermissionRepository(), repository.NewRoleRepository())
}

type UpdatePermissionRequest struct {
	ModuleID  uint             `json:"module_id"`
	CanView   bool             `json:"can_view"`
	CanCreate bool             `json:"can_create"`
	CanEdit   bool             `json:"can_edit"`
	CanDelete bool             `json:"can_delete"`
	DataScope models.DataScope `json:"data_scope"`
}

func GetRolesPermissions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	rolesID, ok := httpx.ParseID(c, "id", "role")
	if !ok {
		return
	}

	overview, err := permissionService().Overview(scope, rolesID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"modules": overview.Modules,
		"access":  overview.Access,
	})
}

func UpdateRolesPermissions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	rolesID, ok := httpx.ParseID(c, "id", "role")
	if !ok {
		return
	}

	var input []UpdatePermissionRequest
	if !httpx.BindJSON(c, &input) {
		return
	}

	entries := make([]service.PermissionEntry, 0, len(input))
	for _, item := range input {
		entries = append(entries, service.PermissionEntry{
			ModuleID:  item.ModuleID,
			CanView:   item.CanView,
			CanCreate: item.CanCreate,
			CanEdit:   item.CanEdit,
			CanDelete: item.CanDelete,
			DataScope: item.DataScope,
		})
	}

	if err := permissionService().Replace(scope, rolesID, entries); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Permissions updated successfully"})
}

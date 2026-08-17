package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

type UpdatePermissionRequest struct {
	ModuleID  uint             `json:"module_id"`
	CanView   bool             `json:"can_view"`
	CanCreate bool             `json:"can_create"`
	CanEdit   bool             `json:"can_edit"`
	CanDelete bool             `json:"can_delete"`
	DataScope models.DataScope `json:"data_scope"`
}

func (h *Handler) GetRolesPermissions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	rolesID, ok := httpx.ParseID(c, "id", "role")
	if !ok {
		return
	}

	overview, err := h.Permission.Overview(scope, rolesID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"modules": overview.Modules,
		"access":  overview.Access,
	})
}

func (h *Handler) UpdateRolesPermissions(c *gin.Context) {
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

	if err := h.Permission.Replace(scope, rolesID, entries); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Permissions updated successfully"})
}

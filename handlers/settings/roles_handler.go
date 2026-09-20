package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
)

// GetRoles returns the roles of the caller's authorized companies.
func (h *Handler) GetRoles(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	query, ok := httpx.ParsePage(c)
	if !ok {
		return
	}
	roles, err := h.Role.List(scope, query)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, roles)
}

// GetRoleOptions returns a minimal id/name list for dropdowns, scoped to the caller's accessible companies.
func (h *Handler) GetRoleOptions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	roles, err := h.Role.Options(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	type option struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		CompaniesID uint   `json:"companies_id"`
	}
	options := make([]option, 0, len(roles))
	for _, r := range roles {
		options = append(options, option{ID: r.ID, Name: r.RolesName, CompaniesID: r.CompaniesID})
	}
	c.JSON(http.StatusOK, options)
}

func (h *Handler) CreateRole(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.Roles
	if !httpx.BindJSON(c, &input) {
		return
	}

	role, err := h.Role.Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, role)
}

func (h *Handler) UpdateRole(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "role")
	if !ok {
		return
	}

	// companies_id is accepted and ignored: the company is immutable after creation.
	var input struct {
		RolesName string `json:"roles_name"`
	}
	if !httpx.BindJSON(c, &input) {
		return
	}

	role, err := h.Role.Update(scope, id, input.RolesName)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, role)
}

func (h *Handler) DeleteRole(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "role")
	if !ok {
		return
	}

	if err := h.Role.Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// roleService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func roleService() *service.RoleService {
	return service.NewRoleService(repository.NewRoleRepository())
}

// GetRoles returns the roles of the caller's authorized companies. This used to
// read every role in the database regardless of company, which contradicted both
// its own doc comment and RolesPage being declared a company-level module.
func GetRoles(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	roles, err := roleService().List(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, roles)
}

// GetRoleOptions returns a minimal id/name list for dropdowns, scoped to the
// caller's accessible companies. Ungated by page permission (mirrors
// GetCompanies) so any role filling out an employee-linked form or granting
// company access can see whether roles exist for a company.
func GetRoleOptions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	roles, err := roleService().Options(scope)
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

func CreateRole(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.Roles
	if !httpx.BindJSON(c, &input) {
		return
	}

	role, err := roleService().Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, role)
}

func UpdateRole(c *gin.Context) {
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

	role, err := roleService().Update(scope, id, input.RolesName)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, role)
}

func DeleteRole(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "role")
	if !ok {
		return
	}

	if err := roleService().Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

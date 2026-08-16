package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// departmentService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func departmentService() *service.DepartmentService {
	return service.NewDepartmentService(repository.NewDepartmentRepository())
}

func GetDepartments(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	departments, err := departmentService().List(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, departments)
}

// GetDepartmentOptions returns a minimal id/name list for dropdowns.
func GetDepartmentOptions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	departments, err := departmentService().Options(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	type option struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		CompaniesID uint   `json:"companies_id"`
	}
	options := make([]option, 0, len(departments))
	for _, d := range departments {
		options = append(options, option{ID: d.ID, Name: d.DeptName, CompaniesID: d.CompaniesID})
	}
	c.JSON(http.StatusOK, options)
}

func CreateDepartment(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.Department
	if !httpx.BindJSON(c, &input) {
		return
	}

	department, err := departmentService().Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, department)
}

func UpdateDepartment(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "department")
	if !ok {
		return
	}

	// companies_id is accepted and ignored: the company is immutable after creation.
	var input struct {
		DeptName   string `json:"dept_name"`
		CategoryID uint   `json:"category_id"`
	}
	if !httpx.BindJSON(c, &input) {
		return
	}

	department, err := departmentService().Update(scope, id, input.DeptName, input.CategoryID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, department)
}

func DeleteDepartment(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "department")
	if !ok {
		return
	}

	if err := departmentService().Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Department deleted successfully"})
}

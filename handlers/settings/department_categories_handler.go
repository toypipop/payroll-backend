package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// departmentCategoryService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func departmentCategoryService() *service.DepartmentCategoryService {
	return service.NewDepartmentCategoryService(repository.NewDepartmentCategoryRepository())
}

func GetDepartmentCategories(c *gin.Context) {
	categories, err := departmentCategoryService().List()
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, categories)
}

func CreateDepartmentCategory(c *gin.Context) {
	var input struct {
		Name string `json:"name"`
	}
	if !httpx.BindJSON(c, &input) {
		return
	}

	category, err := departmentCategoryService().Create(input.Name)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, category)
}

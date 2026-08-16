package payrollconfig

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// employmentTypeService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func employmentTypeService() *service.EmploymentTypeService {
	return service.NewEmploymentTypeService(repository.NewEmploymentTypeRepository())
}

func GetEmploymentTypes(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	types, err := employmentTypeService().List(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, types)
}

// GetEmploymentTypeOptions returns a minimal id/name list for dropdowns.
func GetEmploymentTypeOptions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	types, err := employmentTypeService().Options(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	type option struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		CompaniesID uint   `json:"companies_id"`
	}
	options := make([]option, 0, len(types))
	for _, t := range types {
		options = append(options, option{ID: t.ID, Name: t.Name, CompaniesID: t.CompaniesID})
	}
	c.JSON(http.StatusOK, options)
}

func CreateEmploymentType(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.EmploymentType
	if !httpx.BindJSON(c, &input) {
		return
	}

	employmentType, err := employmentTypeService().Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, employmentType)
}

func UpdateEmploymentType(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "employment type")
	if !ok {
		return
	}

	// companies_id is not accepted here: the company is immutable after creation.
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if !httpx.BindJSON(c, &input) {
		return
	}

	employmentType, err := employmentTypeService().Update(scope, id, input.Name, input.Description)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, employmentType)
}

func DeleteEmploymentType(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "employment type")
	if !ok {
		return
	}

	if err := employmentTypeService().Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Employment type deleted"})
}

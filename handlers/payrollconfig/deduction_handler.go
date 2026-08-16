package payrollconfig

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// deductionTypeService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func deductionTypeService() *service.DeductionTypeService {
	return service.NewDeductionTypeService(repository.NewDeductionTypeRepository())
}

func GetDeductionTypes(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	deductions, err := deductionTypeService().List(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, deductions)
}

// GetDeductionTypeOptions returns a minimal id/name list for dropdowns.
func GetDeductionTypeOptions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	deductions, err := deductionTypeService().Options(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	type option struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		CompaniesID uint   `json:"companies_id"`
	}
	options := make([]option, 0, len(deductions))
	for _, d := range deductions {
		options = append(options, option{ID: d.ID, Name: d.Name, CompaniesID: d.CompaniesID})
	}
	c.JSON(http.StatusOK, options)
}

func CreateDeductionType(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.DeductionType
	if !httpx.BindJSON(c, &input) {
		return
	}

	deductionType, err := deductionTypeService().Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, deductionType)
}

func UpdateDeductionType(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "deduction type")
	if !ok {
		return
	}

	// companies_id is accepted and ignored: the company is immutable after creation.
	var input models.DeductionType
	if !httpx.BindJSON(c, &input) {
		return
	}

	deductionType, err := deductionTypeService().Update(scope, id, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, deductionType)
}

func DeleteDeductionType(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "deduction type")
	if !ok {
		return
	}

	if err := deductionTypeService().Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

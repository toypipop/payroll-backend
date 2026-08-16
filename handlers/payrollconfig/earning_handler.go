package payrollconfig

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// earningTypeService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func earningTypeService() *service.EarningTypeService {
	return service.NewEarningTypeService(repository.NewEarningTypeRepository())
}

func GetEarningTypes(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	earnings, err := earningTypeService().List(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, earnings)
}

// GetEarningTypeOptions returns a minimal id/name list for dropdowns.
func GetEarningTypeOptions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	earnings, err := earningTypeService().Options(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	type option struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		CompaniesID uint   `json:"companies_id"`
	}
	options := make([]option, 0, len(earnings))
	for _, e := range earnings {
		options = append(options, option{ID: e.ID, Name: e.Name, CompaniesID: e.CompaniesID})
	}
	c.JSON(http.StatusOK, options)
}

func CreateEarningType(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.EarningType
	if !httpx.BindJSON(c, &input) {
		return
	}

	earningType, err := earningTypeService().Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, earningType)
}

func UpdateEarningType(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "earning type")
	if !ok {
		return
	}

	// companies_id is accepted and ignored: the company is immutable after creation.
	var input models.EarningType
	if !httpx.BindJSON(c, &input) {
		return
	}

	earningType, err := earningTypeService().Update(scope, id, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, earningType)
}

func DeleteEarningType(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "earning type")
	if !ok {
		return
	}

	if err := earningTypeService().Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// workLocationService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func workLocationService() *service.WorkLocationService {
	return service.NewWorkLocationService(repository.NewWorkLocationRepository())
}

func GetWorkLocations(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	locations, err := workLocationService().List(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, locations)
}

// GetWorkLocationOptions returns a minimal id/name list for dropdowns.
// Ungated by page permission (mirrors GetCompanies) so any role filling
// out a time log needs this dropdown, not just WorkLocationPage users.
func GetWorkLocationOptions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	locations, err := workLocationService().Options(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	type option struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		CompaniesID uint   `json:"companies_id"`
	}
	options := make([]option, 0, len(locations))
	for _, l := range locations {
		options = append(options, option{ID: l.ID, Name: l.Name, CompaniesID: l.CompaniesID})
	}
	c.JSON(http.StatusOK, options)
}

func CreateWorkLocation(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.WorkLocation
	if !httpx.BindJSON(c, &input) {
		return
	}

	location, err := workLocationService().Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, location)
}

func UpdateWorkLocation(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "work location")
	if !ok {
		return
	}

	var input struct {
		Name string `json:"name"`
	}
	if !httpx.BindJSON(c, &input) {
		return
	}

	location, err := workLocationService().Update(scope, id, input.Name)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, location)
}

func DeleteWorkLocation(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "work location")
	if !ok {
		return
	}

	if err := workLocationService().Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Work location deleted"})
}

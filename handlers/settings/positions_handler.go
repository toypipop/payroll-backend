package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// positionService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func positionService() *service.PositionService {
	return service.NewPositionService(repository.NewPositionRepository())
}

func GetPositions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	positions, err := positionService().List(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, positions)
}

// GetPositionOptions returns a minimal id/name list for dropdowns.
func GetPositionOptions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	positions, err := positionService().Options(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	type option struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		CompaniesID uint   `json:"companies_id"`
	}
	options := make([]option, 0, len(positions))
	for _, p := range positions {
		options = append(options, option{ID: p.ID, Name: p.PosName, CompaniesID: p.CompaniesID})
	}
	c.JSON(http.StatusOK, options)
}

func CreatePosition(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.Position
	if !httpx.BindJSON(c, &input) {
		return
	}

	position, err := positionService().Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, position)
}

func UpdatePosition(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "position")
	if !ok {
		return
	}

	// companies_id is accepted and ignored: the company is immutable after creation.
	var input struct {
		PosName string `json:"pos_name"`
	}
	if !httpx.BindJSON(c, &input) {
		return
	}

	position, err := positionService().Update(scope, id, input.PosName)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, position)
}

func DeletePosition(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "position")
	if !ok {
		return
	}

	if err := positionService().Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Position deleted successfully"})
}

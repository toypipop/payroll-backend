package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetWorkLocations(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	query, ok := httpx.ParsePage(c)
	if !ok {
		return
	}
	locations, err := h.WorkLocation.List(scope, query)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, locations)
}

// GetWorkLocationOptions returns a minimal id/name list for dropdowns.
func (h *Handler) GetWorkLocationOptions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	locations, err := h.WorkLocation.Options(scope)
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

func (h *Handler) CreateWorkLocation(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.WorkLocation
	if !httpx.BindJSON(c, &input) {
		return
	}

	location, err := h.WorkLocation.Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, location)
}

func (h *Handler) UpdateWorkLocation(c *gin.Context) {
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

	location, err := h.WorkLocation.Update(scope, id, input.Name)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, location)
}

func (h *Handler) DeleteWorkLocation(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "work location")
	if !ok {
		return
	}

	if err := h.WorkLocation.Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Work location deleted"})
}

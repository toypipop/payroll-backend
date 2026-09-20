package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetPositions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	query, ok := httpx.ParsePage(c)
	if !ok {
		return
	}
	positions, err := h.Position.List(scope, query)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, positions)
}

// GetPositionOptions returns a minimal id/name list for dropdowns.
func (h *Handler) GetPositionOptions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	positions, err := h.Position.Options(scope)
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

func (h *Handler) CreatePosition(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.Position
	if !httpx.BindJSON(c, &input) {
		return
	}

	position, err := h.Position.Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, position)
}

func (h *Handler) UpdatePosition(c *gin.Context) {
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

	position, err := h.Position.Update(scope, id, input.PosName)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, position)
}

func (h *Handler) DeletePosition(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "position")
	if !ok {
		return
	}

	if err := h.Position.Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Position deleted successfully"})
}

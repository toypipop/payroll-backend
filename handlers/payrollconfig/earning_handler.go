package payrollconfig

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetEarningTypes(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	query, ok := httpx.ParsePage(c)
	if !ok {
		return
	}
	earnings, err := h.EarningType.List(scope, query)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, earnings)
}

// GetEarningTypeOptions returns a minimal id/name list for dropdowns.
func (h *Handler) GetEarningTypeOptions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	earnings, err := h.EarningType.Options(scope)
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

func (h *Handler) CreateEarningType(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.EarningType
	if !httpx.BindJSON(c, &input) {
		return
	}

	earningType, err := h.EarningType.Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, earningType)
}

func (h *Handler) UpdateEarningType(c *gin.Context) {
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

	earningType, err := h.EarningType.Update(scope, id, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, earningType)
}

func (h *Handler) DeleteEarningType(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "earning type")
	if !ok {
		return
	}

	if err := h.EarningType.Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

package payrollconfig

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetDeductionTypes(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	query, ok := httpx.ParsePage(c)
	if !ok {
		return
	}
	deductions, err := h.DeductionType.List(scope, query)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, deductions)
}

// GetDeductionTypeOptions returns a minimal id/name list for dropdowns.
func (h *Handler) GetDeductionTypeOptions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	deductions, err := h.DeductionType.Options(scope)
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

func (h *Handler) CreateDeductionType(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.DeductionType
	if !httpx.BindJSON(c, &input) {
		return
	}

	deductionType, err := h.DeductionType.Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, deductionType)
}

func (h *Handler) UpdateDeductionType(c *gin.Context) {
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

	deductionType, err := h.DeductionType.Update(scope, id, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, deductionType)
}

func (h *Handler) DeleteDeductionType(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "deduction type")
	if !ok {
		return
	}

	if err := h.DeductionType.Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

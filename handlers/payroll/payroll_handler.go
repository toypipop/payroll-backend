package payroll

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
)

// GetPayrollConfigs starts from companies so a company with no configuration yet still appears in the list.
func (h *Handler) GetPayrollConfigs(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	query, ok := httpx.ParsePage(c)
	if !ok {
		return
	}
	companies, err := h.PayrollConfig.List(scope, query)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, companies)
}

// UpdatePayrollConfig saves a company's configuration, updating the existing row when there is one: companies_id is unique, so a company has exactly one.
func (h *Handler) UpdatePayrollConfig(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.PayrollConfig
	if !httpx.BindJSON(c, &input) {
		return
	}

	saved, err := h.PayrollConfig.Save(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Configuration saved", "data": saved})
}

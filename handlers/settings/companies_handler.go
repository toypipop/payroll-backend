package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// GetCompanies backs /companies/options and is limited to the caller's
func (h *Handler) GetCompanies(c *gin.Context) {
	userID, _ := httpx.ContextAuth(c)

	companies, err := h.Company.Options(userID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, companies)
}

func (h *Handler) GetAllCompanies(c *gin.Context) {
	companies, err := h.Company.ListAll()
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, companies)
}

// CreateCompanies bootstraps a usable tenant in one transaction: the company
func (h *Handler) CreateCompanies(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.Companies
	if !httpx.BindJSON(c, &input) {
		return
	}

	company, err := h.Company.Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, company)
}

func (h *Handler) UpdateCompanies(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "company")
	if !ok {
		return
	}

	var input struct {
		Name    string `json:"name"`
		TaxID   string `json:"tax_id"`
		Address string `json:"address"`
		Phone   string `json:"phone"`
	}
	if !httpx.BindJSON(c, &input) {
		return
	}

	company, err := h.Company.Update(scope, id, service.CompanyUpdate{
		Name:    input.Name,
		TaxID:   input.TaxID,
		Address: input.Address,
		Phone:   input.Phone,
	})
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, company)
}

func (h *Handler) SetCompanyStatus(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "company")
	if !ok {
		return
	}

	var input struct {
		IsActive *bool `json:"is_active" binding:"required"`
	}
	if !httpx.BindJSON(c, &input) {
		return
	}
	company, err := h.Company.SetActive(scope, id, *input.IsActive)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, company)
}

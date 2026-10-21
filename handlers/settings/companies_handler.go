package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// GetCompanies backs /companies/options and is limited to the caller's accessible companies.
// Dropdowns need only the id and name, so the profile (tax ID, address, phone) and configs stay out of
// this ungated response; pages that show them read /companies/all or their own report endpoints.
func (h *Handler) GetCompanies(c *gin.Context) {
	userID, _ := httpx.ContextAuth(c)

	companies, err := h.Company.Options(userID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	type option struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	}
	options := make([]option, 0, len(companies))
	for _, company := range companies {
		options = append(options, option{ID: company.ID, Name: company.Name})
	}
	c.JSON(http.StatusOK, options)
}

func (h *Handler) GetAllCompanies(c *gin.Context) {
	query, ok := httpx.ParsePage(c)
	if !ok {
		return
	}
	active, ok := httpx.ParseOptionalBool(c, "is_active")
	if !ok {
		return
	}
	companies, err := h.Company.ListPage(query, active)
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

	var input struct {
		Name    string `json:"name"`
		TaxID   string `json:"tax_id"`
		Address string `json:"address"`
		Phone   string `json:"phone"`
	}
	if !httpx.BindStrictJSON(c, &input) {
		return
	}

	company, err := h.Company.Create(scope, models.Companies{Name: input.Name, TaxID: input.TaxID, Address: input.Address, Phone: input.Phone})
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

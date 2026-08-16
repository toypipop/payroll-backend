package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// companyService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func companyService() *service.CompanyService {
	return service.NewCompanyService(repository.NewCompanyRepository())
}

// GetCompanies backs /companies/options and is limited to the caller's
func GetCompanies(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)

	companies, err := companyService().Options(userID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, companies)
}

func GetAllCompanies(c *gin.Context) {
	companies, err := companyService().ListAll()
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, companies)
}

// CreateCompanies bootstraps a usable tenant in one transaction: the company
func CreateCompanies(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)

	var input models.Companies
	if !httpx.BindJSON(c, &input) {
		return
	}

	company, err := companyService().Create(userID, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, company)
}

func UpdateCompanies(c *gin.Context) {
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

	company, err := companyService().Update(id, service.CompanyUpdate{
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

func DeleteCompanies(c *gin.Context) {
	id, ok := httpx.ParseID(c, "id", "company")
	if !ok {
		return
	}

	if err := companyService().Delete(id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

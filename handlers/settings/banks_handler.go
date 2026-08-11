package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// bankService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func bankService() *service.BankService {
	return service.NewBankService(repository.NewBankRepository())
}

func GetBanks(c *gin.Context) {
	banks, err := bankService().List()
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, banks)
}

// GetBankOptions returns a minimal list for dropdowns. Ungated by page
// permission (mirrors GetCompanies) so any role filling out a bank-account
// field needs this dropdown, not just MasterBankPage users. Banks are global
// (not company-scoped), so no data-access check applies.
func GetBankOptions(c *gin.Context) {
	banks, err := bankService().List()
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	type option struct {
		ID       uint   `json:"id"`
		BankCode string `json:"bank_code"`
		BankName string `json:"bank_name"`
	}
	options := make([]option, 0, len(banks))
	for _, b := range banks {
		options = append(options, option{ID: b.ID, BankCode: b.BankCode, BankName: b.BankName})
	}
	c.JSON(http.StatusOK, options)
}

func CreateBank(c *gin.Context) {
	var input models.Bank
	if !httpx.BindJSON(c, &input) {
		return
	}

	bank, err := bankService().Create(input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, bank)
}

func UpdateBank(c *gin.Context) {
	id, ok := httpx.ParseID(c, "id", "bank")
	if !ok {
		return
	}

	var input models.Bank
	if !httpx.BindJSON(c, &input) {
		return
	}

	bank, err := bankService().Update(id, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, bank)
}

func DeleteBank(c *gin.Context) {
	id, ok := httpx.ParseID(c, "id", "bank")
	if !ok {
		return
	}

	if err := bankService().Delete(id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Bank deleted"})
}

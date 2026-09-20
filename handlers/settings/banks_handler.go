package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetBanks(c *gin.Context) {
	query, ok := httpx.ParsePage(c)
	if !ok {
		return
	}
	banks, err := h.Bank.ListPage(query)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, banks)
}

// GetBankOptions returns a minimal list for dropdowns.
func (h *Handler) GetBankOptions(c *gin.Context) {
	banks, err := h.Bank.List()
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

func (h *Handler) CreateBank(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	var input models.Bank
	if !httpx.BindJSON(c, &input) {
		return
	}

	bank, err := h.Bank.Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, bank)
}

func (h *Handler) UpdateBank(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "bank")
	if !ok {
		return
	}

	var input models.Bank
	if !httpx.BindJSON(c, &input) {
		return
	}

	bank, err := h.Bank.Update(scope, id, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, bank)
}

func (h *Handler) DeleteBank(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "bank")
	if !ok {
		return
	}

	if err := h.Bank.Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Bank deleted"})
}

package settings

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
)

func GetBanks(c *gin.Context) {
	var banks []models.Bank
	if err := config.DB.Order("bank_code").Find(&banks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch banks"})
		return
	}
	c.JSON(http.StatusOK, banks)
}

// GetBankOptions returns a minimal list for dropdowns. Ungated by page
// permission (mirrors GetCompanies) so any role filling out a
// bank-account field needs this dropdown, not just MasterBankPage users.
// Banks are global (not company-scoped), so no data-access check applies.
func GetBankOptions(c *gin.Context) {
	var banks []models.Bank
	if err := config.DB.Order("bank_code").Find(&banks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch banks"})
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
	var bank models.Bank
	if err := c.ShouldBindJSON(&bank); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := config.DB.Create(&bank).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create bank"})
		return
	}
	c.JSON(http.StatusCreated, bank)
}

func UpdateBank(c *gin.Context) {
	id := c.Param("id")
	var bank models.Bank
	if err := config.DB.Where("id = ?", id).First(&bank).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bank not found"})
		return
	}
	var input struct {
		BankCode  string `json:"bank_code" binding:"required"`
		BankName  string `json:"bank_name" binding:"required"`
		SwiftCode string `json:"swift_code"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	bank.BankCode = input.BankCode
	bank.BankName = input.BankName
	bank.SwiftCode = input.SwiftCode
	config.DB.Save(&bank)
	c.JSON(http.StatusOK, bank)
}

func DeleteBank(c *gin.Context) {
	id := c.Param("id")
	var bank models.Bank
	if err := config.DB.Where("id = ?", id).First(&bank).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bank not found"})
		return
	}
	config.DB.Delete(&bank)
	c.JSON(http.StatusOK, gin.H{"message": "Bank deleted"})
}

package settings

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
)

// --- COMPANIES ---
// Companies has no owning company of its own — every user with CompaniesPage
// permission sees and manages the full list, gated only by CheckPermission.

func GetCompanies(c *gin.Context) {
	var companies []models.Companies
	if err := config.DB.Find(&companies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, companies)
}

func GetAllCompanies(c *gin.Context) {
	var companies []models.Companies
	if err := config.DB.Order("id desc").Find(&companies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch companies"})
		return
	}
	c.JSON(http.StatusOK, companies)
}

func CreateCompanies(c *gin.Context) {
	var companies models.Companies
	if err := c.ShouldBindJSON(&companies); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := config.DB.Create(&companies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create companies"})
		return
	}
	c.JSON(http.StatusCreated, companies)
}

func UpdateCompanies(c *gin.Context) {
	id := c.Param("id")

	var companies models.Companies
	if err := config.DB.First(&companies, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Companies not found"})
		return
	}

	var updateData struct {
		Name    string `json:"name"`
		TaxID   string `json:"tax_id"`
		Address string `json:"address"`
		Phone   string `json:"phone"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config.DB.Model(&companies).Updates(updateData)
	c.JSON(http.StatusOK, companies)
}

func DeleteCompanies(c *gin.Context) {
	id := c.Param("id")

	result := config.DB.Delete(&models.Companies{}, id)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Companies not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

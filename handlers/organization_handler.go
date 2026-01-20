package handlers

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
)

/* --- Master Data Handlers (English Comments) --- */

// GetDepartments - Fetch all available departments
func GetDepartments(c *gin.Context) {
	var depts []models.Department
	config.DB.Find(&depts)
	c.JSON(http.StatusOK, depts)
}

func CreateDepartment(c *gin.Context) {
	var dept models.Department
	if err := c.ShouldBindJSON(&dept); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.DB.Create(&dept)
	c.JSON(http.StatusCreated, dept)
}

// GetPositions - Fetch all available positions
func GetPositions(c *gin.Context) {
	var positions []models.Position
	config.DB.Find(&positions)
	c.JSON(http.StatusOK, positions)
}

// GET: Fetch all companies
func GetCompanies(c *gin.Context) {
	var companies []models.Company
	config.DB.Find(&companies)
	c.JSON(http.StatusOK, companies)
}

// POST: Create new company
func CreateCompany(c *gin.Context) {
	var company models.Company
	if err := c.ShouldBindJSON(&company); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := config.DB.Create(&company).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create company"})
		return
	}
	c.JSON(http.StatusCreated, company)
}

/* --- UpdateCompany Handler (English Comment) --- */
func UpdateCompany(c *gin.Context) {
	id := c.Param("id")
	var company models.Company

	// 1. Find the existing record first
	if err := config.DB.First(&company, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Company not found"})
		return
	}

	// 2. Bind new data to a temporary struct or map
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

	// 3. Update with specific fields to avoid unique conflict with itself
	if err := config.DB.Model(&company).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, company)
}

// DELETE: Remove company
func DeleteCompany(c *gin.Context) {
	id := c.Param("id")
	if err := config.DB.Delete(&models.Company{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

package handlers

import (
	"fmt"
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

/* --- Master Data Handlers (English Comments) --- */

// GetRoles fetches all available roles without any filters (English Comment)
func GetRoles(c *gin.Context) {
	var roles []models.Role

	// Select * FROM roles (English Comment)
	if err := config.DB.Find(&roles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch roles"})
		return
	}

	c.JSON(http.StatusOK, roles)
}

// GetDepartments - Fetch all available departments
func GetDepartments(c *gin.Context) {
	companyID, _ := c.Get("company_id") // Get from Middleware
	var depts []models.Department

	// Filter by company
	config.DB.Scopes(repository.ScopeCompany(companyID.(uint))).Find(&depts)
	c.JSON(http.StatusOK, depts)
}

func CreateDepartment(c *gin.Context) {
	var dept models.Department
	if err := c.ShouldBindJSON(&dept); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Assign company_id from token
	companyID, _ := c.Get("company_id")
	dept.CompanyID = companyID.(uint)

	config.DB.Create(&dept)
	c.JSON(http.StatusCreated, dept)
}

// GetPositions - Fetch all available positions
func GetPositions(c *gin.Context) {
	companyID, _ := c.Get("company_id")
	var positions []models.Position
	// Added Scope (English Comment)
	config.DB.Scopes(repository.ScopeCompany(companyID.(uint))).Find(&positions)
	c.JSON(http.StatusOK, positions)
}

func GetCompany(c *gin.Context) {
	companyID, _ := c.Get("company_id")

	var companys []models.Company

	// Fetch only the company associated with the user's company_id
	config.DB.Where("id = ?", companyID).Find(&companys)

	c.JSON(http.StatusOK, companys)
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

// PUT: Update company (Own Company Only)
func UpdateCompany(c *gin.Context) {
	userCompanyID, _ := c.Get("company_id")
	targetID := c.Param("id")

	var company models.Company

	// Verify if the target ID matches the user's authorized company ID
	if targetID != fmt.Sprintf("%v", userCompanyID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only update your own company"})
		return
	}

	if err := config.DB.First(&company, targetID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Company not found"})
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

	config.DB.Model(&company).Updates(updateData)
	c.JSON(http.StatusOK, company)
}

// DELETE: Remove company
func DeleteCompany(c *gin.Context) {
	id := c.Param("id")

	// Perform deletion based on the provided ID
	if err := config.DB.Delete(&models.Company{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

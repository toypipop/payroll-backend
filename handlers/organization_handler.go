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

/* --- handlers/organization_handler.go --- */

// GET: Fetch company (Filtered by Role)
func GetCompany(c *gin.Context) {
	isSysAdmin, _ := c.Get("is_super_admin")
	companyID, _ := c.Get("company_id")

	var companys []models.Company

	if isSysAdmin.(bool) {
		// System Admin: Fetch all companys (English Comment)
		config.DB.Find(&companys)
	} else {
		// Regular User: Fetch only their own company (English Comment)
		config.DB.Where("id = ?", companyID).Find(&companys)
	}
	c.JSON(http.StatusOK, companys)
}

// POST: Create new company (System Admin Only)
func CreateCompany(c *gin.Context) {
	isSysAdmin, _ := c.Get("is_super_admin")

	// Role check (English Comment)
	if !isSysAdmin.(bool) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: System Admin only"})
		return
	}

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

// PUT: Update company (System Admin or Own Company)
func UpdateCompany(c *gin.Context) {
	isSysAdmin, _ := c.Get("is_super_admin")
	userCompanyID, _ := c.Get("company_id")
	targetID := c.Param("id") // ID from URL

	var company models.Company

	// Logic: If not sys admin, the targetID must match user's companyID (English Comment)
	if !isSysAdmin.(bool) && targetID != fmt.Sprintf("%v", userCompanyID) {
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

// DELETE: Remove company (System Admin Only)
func DeleteCompany(c *gin.Context) {
	isSysAdmin, _ := c.Get("is_super_admin")

	if !isSysAdmin.(bool) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: System Admin only"})
		return
	}

	id := c.Param("id")
	if err := config.DB.Delete(&models.Company{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

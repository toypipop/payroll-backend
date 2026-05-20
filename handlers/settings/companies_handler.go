package settings

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

// --- COMPANIES ---

func GetCompanies(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var companies []models.Companies
	// For Companies table, we use "id" as the companies filter column
	dbResult := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "id", "id"),
	).Find(&companies)

	if dbResult.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": dbResult.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, companies)
}

func GetAllCompanies(c *gin.Context) {
	// This is typically for Super Admin, but we can apply ApplyScopes with ScopeAll logic
	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	var companies []models.Companies
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "id", "id"),
	).Order("id desc").Find(&companies).Error; err != nil {
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
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	var companies models.Companies
	// Ensure the user can only update the companies they belong to (unless ScopeAll)
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "id", "id"),
	).First(&companies, id).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this companies"})
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
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	// Apply strict scope for deletion
	result := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "id", "id"),
	).Delete(&models.Companies{}, id)

	if result.RowsAffected == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "Delete failed or unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

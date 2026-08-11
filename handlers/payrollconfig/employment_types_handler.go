package payrollconfig

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

func GetEmploymentTypes(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)
	var currentUser models.Employees
	if err := config.DB.Select("id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var types []models.EmploymentType
	if err := config.DB.Scopes(
		repository.ApplyCompanyScopes(c, currentUser, "employment_type.companies_id"),
	).Preload("Companies").Order("name").Find(&types).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch employment types"})
		return
	}
	c.JSON(http.StatusOK, types)
}

// GetEmploymentTypeOptions returns a minimal id/name list for dropdowns.
// Ungated by page permission (mirrors GetCompanies) so any role filling out
// an employee-linked form can see whether employment types exist for a company.
func GetEmploymentTypeOptions(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)
	var currentUser models.Employees
	if err := config.DB.Select("id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	type option struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		CompaniesID uint   `json:"companies_id"`
	}

	var types []models.EmploymentType
	if err := config.DB.Scopes(
		repository.ApplyCompanyScopes(c, currentUser, "employment_type.companies_id"),
	).Order("name").Find(&types).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch employment types"})
		return
	}

	options := make([]option, 0, len(types))
	for _, t := range types {
		options = append(options, option{ID: t.ID, Name: t.Name, CompaniesID: t.CompaniesID})
	}
	c.JSON(http.StatusOK, options)
}

func CreateEmploymentType(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, companies_id").First(&currentUser, userID)

	var et models.EmploymentType
	if err := c.ShouldBindJSON(&et); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var targetCompany models.Companies
	if err := config.DB.Scopes(
		repository.ApplyCompanyScopes(c, currentUser, "id"),
	).Where("id = ?", et.CompaniesID).First(&targetCompany).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for this company"})
		return
	}

	et.ID = 0
	if err := config.DB.Create(&et).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create employment type"})
		return
	}
	c.JSON(http.StatusCreated, et)
}

func UpdateEmploymentType(c *gin.Context) {
	id := c.Param("id")
	userID, _ := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, companies_id").First(&currentUser, userID)

	var et models.EmploymentType
	if err := config.DB.Scopes(
		repository.ApplyCompanyScopes(c, currentUser, "employment_type.companies_id"),
	).First(&et, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employment type not found"})
		return
	}

	var input struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	et.Name = input.Name
	et.Description = input.Description
	config.DB.Save(&et)
	c.JSON(http.StatusOK, et)
}

func DeleteEmploymentType(c *gin.Context) {
	id := c.Param("id")
	userID, _ := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, companies_id").First(&currentUser, userID)

	var et models.EmploymentType
	if err := config.DB.Scopes(
		repository.ApplyCompanyScopes(c, currentUser, "employment_type.companies_id"),
	).Where("id = ?", id).First(&et).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employment type not found"})
		return
	}

	config.DB.Delete(&et)
	c.JSON(http.StatusOK, gin.H{"message": "Employment type deleted"})
}

package payrollconfig

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

// GET: Fetch all deduction types based on data scope (English Comment)
func GetDeductionTypes(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees

	if err := config.DB.Select("id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var deductions []models.DeductionType
	err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "deduction_types.companies_id", "unused"),
	).Preload("Companies").Find(&deductions).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, deductions)
}

// POST: Create a new deduction type (English Comment)
func CreateDeductionType(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, companies_id").First(&currentUser, userID)

	var input models.DeductionType
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate if the user has permission to create for the target company (English Comment)
	var targetCompany models.Companies
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "id", "unused"),
	).Where("id = ?", input.CompaniesID).First(&targetCompany).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for this company"})
		return
	}

	input.ID = 0
	if err := config.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create record"})
		return
	}
	c.JSON(http.StatusCreated, input)
}

// PUT: Update deduction type using map to handle boolean false (English Comment)
func UpdateDeductionType(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, companies_id").First(&currentUser, userID)

	var deduction models.DeductionType
	// Apply scope to verify record ownership before update (English Comment)
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "deduction_types.companies_id", "unused"),
	).First(&deduction, id).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied or record not found"})
		return
	}

	var input models.DeductionType
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updateData := map[string]interface{}{
		"name":              input.Name,
		"is_tax_deductible": input.IsTaxDeductible,
		"is_sso_deductible": input.IsSsoDeductible,
		"companies_id":      input.CompaniesID,
	}

	if err := config.DB.Model(&deduction).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed"})
		return
	}
	c.JSON(http.StatusOK, deduction)
}

// DELETE: Soft delete with Scope protection (English Comment)
func DeleteDeductionType(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, companies_id").First(&currentUser, userID)

	// Use Scopes and Where to ensure only authorized records are deleted (English Comment)
	var deduction models.DeductionType
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "deduction_types.companies_id", "unused"),
	).Where("id = ?", id).First(&deduction).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Delete failed or unauthorized"})
		return
	}

	result := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "deduction_types.companies_id", "unused"),
	).Where("id = ?", id).Delete(&models.DeductionType{})

	if result.RowsAffected == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "Delete failed or unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

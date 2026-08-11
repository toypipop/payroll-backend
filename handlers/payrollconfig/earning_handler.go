package payrollconfig

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

// GET: Fetch all earning types with data scope (English Comment)
func GetEarningTypes(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)
	var currentUser models.Employees

	if err := config.DB.Select("id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var earnings []models.EarningType
	err := config.DB.Scopes(
		repository.ApplyCompanyScopes(c, currentUser, "earning_type.companies_id"),
	).Preload("Companies").Find(&earnings).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, earnings)
}

// GetEarningTypeOptions returns a minimal id/name list for dropdowns.
// Ungated by page permission (mirrors GetCompanies) so any role assigning
// employee/cycle earnings can see whether earning types exist for a company.
func GetEarningTypeOptions(c *gin.Context) {
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

	var earnings []models.EarningType
	err := config.DB.Scopes(
		repository.ApplyCompanyScopes(c, currentUser, "earning_type.companies_id"),
	).Find(&earnings).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	options := make([]option, 0, len(earnings))
	for _, e := range earnings {
		options = append(options, option{ID: e.ID, Name: e.Name, CompaniesID: e.CompaniesID})
	}
	c.JSON(http.StatusOK, options)
}

// POST: Create a new earning type (English Comment)
func CreateEarningType(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, companies_id").First(&currentUser, userID)

	var input models.EarningType
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate if the user has permission to create for the target company (English Comment)
	var targetCompany models.Companies
	if err := config.DB.Scopes(
		repository.ApplyCompanyScopes(c, currentUser, "id"),
	).Where("id = ?", input.CompaniesID).First(&targetCompany).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for this company"})
		return
	}

	input.ID = 0
	if err := config.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create earning type"})
		return
	}
	c.JSON(http.StatusCreated, input)
}

// PUT: Update an existing earning type by ID with Scope (English Comment)
func UpdateEarningType(c *gin.Context) {
	id := c.Param("id")
	userID, _ := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, companies_id").First(&currentUser, userID)

	var earning models.EarningType
	// Apply scope to ensure user can only find records they are authorized to edit (English Comment)
	if err := config.DB.Scopes(
		repository.ApplyCompanyScopes(c, currentUser, "earning_type.companies_id"),
	).First(&earning, id).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied or record not found"})
		return
	}

	var input models.EarningType
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Company is immutable after creation — ignore companies_id from the request
	updateData := map[string]interface{}{
		"name":         input.Name,
		"is_taxable":   input.IsTaxable,
		"is_sso":       input.IsSso,
		"is_permanent": input.IsPermanent,
	}

	if err := config.DB.Model(&earning).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed"})
		return
	}

	c.JSON(http.StatusOK, earning)
}

// DELETE: Soft delete an earning type with Scope (English Comment)
func DeleteEarningType(c *gin.Context) {
	id := c.Param("id")
	userID, _ := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, companies_id").First(&currentUser, userID)

	// Ensure the delete operation only affects authorized records (English Comment)
	var earning models.EarningType
	if err := config.DB.Scopes(
		repository.ApplyCompanyScopes(c, currentUser, "earning_type.companies_id"),
	).Where("id = ?", id).First(&earning).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Delete failed or unauthorized"})
		return
	}

	result := config.DB.Scopes(
		repository.ApplyCompanyScopes(c, currentUser, "earning_type.companies_id"),
	).Where("id = ?", id).Delete(&models.EarningType{})

	if result.RowsAffected == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "Delete failed or unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

package settings

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

// GET: Fetch all roles within data scope (English Comment)
func GetRoles(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var roles []models.Roles
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "roles.companies_id", "roles.id"),
	).
		Preload("Companies").
		Find(&roles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch roles"})
		return
	}

	c.JSON(http.StatusOK, roles)
}

// POST: Create a new role (English Comment)
func CreateRole(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)
	var input models.Roles
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var currentUser models.Employees
	config.DB.Select("id, companies_id").First(&currentUser, userID)

	// Validate if the user has permission to create role for this company (English Comment)
	var targetCompany models.Companies
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "id", "unused"),
	).Where("id = ?", input.CompaniesID).First(&targetCompany).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for this company"})
		return
	}

	// Force safety constraints (English Comment)
	input.ID = 0

	if err := config.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create role"})
		return
	}
	c.JSON(http.StatusCreated, input)
}

// PUT: Update an existing role (English Comment)
func UpdateRole(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, companies_id").First(&currentUser, userID)

	var role models.Roles
	// Ensure the role belongs to an authorized company.
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "roles.companies_id", "roles.id"),
	).First(&role, id).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied or record not found"})
		return
	}

	var input models.Roles
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updateData := map[string]interface{}{
		"roles_name":   input.RolesName,
		"companies_id": input.CompaniesID,
	}
	if _, ok := repository.ResolveCompanyID(currentUser, dataScope, input.CompaniesID); !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for this company"})
		return
	}

	if err := config.DB.Model(&role).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed"})
		return
	}

	c.JSON(http.StatusOK, role)
}

// DELETE: Delete a role with scope protection (English Comment)
func DeleteRole(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, companies_id").First(&currentUser, userID)

	// Protect against deleting roles outside scope.
	result := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "roles.companies_id", "roles.id"),
	).Where("id = ?", id).Delete(&models.Roles{})

	if result.RowsAffected == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "Delete failed or unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

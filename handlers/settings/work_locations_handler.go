package settings

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

func GetWorkLocations(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var locations []models.WorkLocation
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "work_location.companies_id", "unused"),
	).Preload("Companies").Order("name").Find(&locations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch work locations"})
		return
	}
	c.JSON(http.StatusOK, locations)
}

// GetWorkLocationOptions returns a minimal id/name list for dropdowns.
// Ungated by page permission (mirrors GetCompanies) so any role filling
// out a time log needs this dropdown, not just WorkLocationPage users.
func GetWorkLocationOptions(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	type option struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		CompaniesID uint   `json:"companies_id"`
	}

	var locations []models.WorkLocation
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "work_location.companies_id", "unused"),
	).Order("name").Find(&locations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch work locations"})
		return
	}

	options := make([]option, 0, len(locations))
	for _, l := range locations {
		options = append(options, option{ID: l.ID, Name: l.Name, CompaniesID: l.CompaniesID})
	}
	c.JSON(http.StatusOK, options)
}

func CreateWorkLocation(c *gin.Context) {
	var loc models.WorkLocation
	if err := c.ShouldBindJSON(&loc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)
	companyID, ok := repository.ResolveCompanyID(c, currentUser, dataScope, loc.CompaniesID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for this company"})
		return
	}
	loc.CompaniesID = companyID

	if err := config.DB.Create(&loc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create work location"})
		return
	}
	c.JSON(http.StatusCreated, loc)
}

func UpdateWorkLocation(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var loc models.WorkLocation
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "work_location.companies_id", "unused"),
	).Where("id = ?", id).First(&loc).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Work location not found or unauthorized"})
		return
	}

	var input struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	loc.Name = input.Name
	config.DB.Save(&loc)
	c.JSON(http.StatusOK, loc)
}

func DeleteWorkLocation(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	var loc models.WorkLocation
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "work_location.companies_id", "unused"),
	).Where("id = ?", id).First(&loc).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Work location not found or unauthorized"})
		return
	}
	config.DB.Delete(&loc)
	c.JSON(http.StatusOK, gin.H{"message": "Work location deleted"})
}

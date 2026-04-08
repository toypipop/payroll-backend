package settings

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

func GetDayOffs(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var dayOffs []models.DayOff
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "day_off.companies_id", "unused"),
	).Preload("Companies").Order("weekday ASC, name ASC").Find(&dayOffs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch day offs"})
		return
	}

	c.JSON(http.StatusOK, dayOffs)
}

func GetDayOffCompanies(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var companies []models.Companies
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "companies.id", "unused"),
	).Order("name ASC").Find(&companies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch companies"})
		return
	}

	c.JSON(http.StatusOK, companies)
}

func CreateDayOff(c *gin.Context) {
	var input struct {
		CompaniesID uint   `json:"companies_id"`
		Weekday     int    `json:"weekday" binding:"required,min=0,max=6"`
		Name        string `json:"name" binding:"required"`
		IsPaid      bool   `json:"is_paid"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("companies_id").First(&currentUser, userID)

	targetCompanyID := currentUser.CompaniesID
	if isSuperAdmin, ok := c.Get("is_super_admin"); ok {
		if superAdmin, isBool := isSuperAdmin.(bool); isBool && superAdmin && input.CompaniesID > 0 {
			targetCompanyID = input.CompaniesID
		}
	}

	var company models.Companies
	if err := config.DB.Select("id").First(&company, targetCompanyID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid company"})
		return
	}

	dayOff := models.DayOff{
		CompaniesID: targetCompanyID,
		Weekday:     input.Weekday,
		Name:        input.Name,
		IsPaid:      input.IsPaid,
	}

	if err := config.DB.Create(&dayOff).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create day off"})
		return
	}

	c.JSON(http.StatusCreated, dayOff)
}

func UpdateDayOff(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var dayOff models.DayOff
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "day_off.companies_id", "unused"),
	).Where("id = ?", id).First(&dayOff).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Day off not found or unauthorized"})
		return
	}

	var input struct {
		CompaniesID uint   `json:"companies_id"`
		Weekday     int    `json:"weekday" binding:"required,min=0,max=6"`
		Name        string `json:"name" binding:"required"`
		IsPaid      bool   `json:"is_paid"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	targetCompanyID := dayOff.CompaniesID
	if isSuperAdmin, ok := c.Get("is_super_admin"); ok {
		if superAdmin, isBool := isSuperAdmin.(bool); isBool && superAdmin && input.CompaniesID > 0 {
			targetCompanyID = input.CompaniesID
		}
	}

	var company models.Companies
	if err := config.DB.Select("id").First(&company, targetCompanyID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid company"})
		return
	}

	dayOff.CompaniesID = targetCompanyID
	dayOff.Weekday = input.Weekday
	dayOff.Name = input.Name
	dayOff.IsPaid = input.IsPaid
	config.DB.Save(&dayOff)
	c.JSON(http.StatusOK, dayOff)
}

func DeleteDayOff(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	var dayOff models.DayOff
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "day_off.companies_id", "unused"),
	).Where("id = ?", id).First(&dayOff).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Day off not found or unauthorized"})
		return
	}

	config.DB.Delete(&dayOff)
	c.JSON(http.StatusOK, gin.H{"message": "Day off deleted"})
}

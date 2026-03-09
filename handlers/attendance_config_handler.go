package handlers

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

func GetAttendanceConfigs(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var companies []models.Companies
	err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "companies.id", "unused"),
	).Preload("AttendanceConfig").Find(&companies).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, companies)
}

func UpdateAttendanceConfig(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var input models.AttendanceConfig
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var targetCompany models.Companies
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "companies.id", "unused"),
	).Where("id = ?", input.CompaniesID).First(&targetCompany).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for this company"})
		return
	}

	err := config.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "companies_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"late_rate_per_minute": input.LateRatePerMinute,
			"absent_rate_per_day":  input.AbsentRatePerDay,
			"work_start_hour":      input.WorkStartHour,
			"work_start_min":       input.WorkStartMin,
			"updated_at":           time.Now(),
		}),
	}).Create(&input).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Configuration saved", "data": input})
}

package attendance

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

func GetTimeLogs(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var logs []models.TimeLog
	err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "time_logs.companies_id", "time_logs.employees_id"),
	).Preload("Employees").Preload("Employees.Companies").Find(&logs).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, logs)
}

func CreateTimeLog(c *gin.Context) {
	var input models.TimeLog
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. Validate CheckIn and CheckOut sequence (English Comment)
	if input.CheckIn != nil && input.CheckOut != nil {
		if input.CheckOut.Before(*input.CheckIn) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Check-out time cannot be earlier than check-in time"})
			return
		}
	}

	// 2. Get Context and Security (English Comment)
	userID, _ := repository.GetContextAuth(c)
	var currentUser models.Employees
	if err := config.DB.Select("companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context invalid"})
		return
	}

	// 3. Force Companie ID for security (English Comment)
	input.CompaniesID = currentUser.CompaniesID

	// 4. Save to Database (English Comment)
	if err := config.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, input)
}

func UpdateTimeLog(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	var log models.TimeLog
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "time_logs.companies_id", "time_logs.employees_id"),
	).First(&log, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or access denied"})
		return
	}

	if err := c.ShouldBindJSON(&log); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.CompaniesID = currentUser.CompaniesID
	config.DB.Save(&log)
	c.JSON(http.StatusOK, log)
}

func DeleteTimeLog(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	result := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "time_logs.companies_id", "time_logs.employees_id"),
	).Where("id = ?", id).Delete(&models.TimeLog{})

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

package handlers

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

func GetHolidays(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var holidays []models.Holiday
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "holiday.companies_id", "unused"),
	).Preload("Companies").Order("date").Find(&holidays).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch holidays"})
		return
	}
	c.JSON(http.StatusOK, holidays)
}

func CreateHoliday(c *gin.Context) {
	var holiday models.Holiday
	if err := c.ShouldBindJSON(&holiday); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, _ := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("companies_id").First(&currentUser, userID)
	holiday.CompaniesID = currentUser.CompaniesID

	if err := config.DB.Create(&holiday).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create holiday"})
		return
	}
	c.JSON(http.StatusCreated, holiday)
}

func UpdateHoliday(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var holiday models.Holiday
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "holiday.companies_id", "unused"),
	).Where("id = ?", id).First(&holiday).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Holiday not found or unauthorized"})
		return
	}

	var input struct {
		Date   string `json:"date" binding:"required"`
		Name   string `json:"name" binding:"required"`
		IsPaid bool   `json:"is_paid"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	holiday.Name = input.Name
	holiday.IsPaid = input.IsPaid
	config.DB.Save(&holiday)
	c.JSON(http.StatusOK, holiday)
}

func DeleteHoliday(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	var holiday models.Holiday
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "holiday.companies_id", "unused"),
	).Where("id = ?", id).First(&holiday).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Holiday not found or unauthorized"})
		return
	}
	config.DB.Delete(&holiday)
	c.JSON(http.StatusOK, gin.H{"message": "Holiday deleted"})
}

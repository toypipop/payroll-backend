package handlers

import (
	"net/http"

	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TimeLogHandler struct {
	db *gorm.DB
}

func (h *TimeLogHandler) GetTimeLogs(c *gin.Context) {
	userID := uint(c.GetFloat64("user_id"))
	companiesID := uint(c.GetFloat64("companies_id"))

	valScope, exists := c.Get("data_scope")
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission scope missing"})
		return
	}
	dataScope := valScope.(models.DataScope)

	currentUser := models.Employees{
		ID:          userID,
		CompaniesID: companiesID,
	}

	if dataScope == models.ScopeDepartment {
		h.db.First(&currentUser, userID)
	}

	var logs []models.TimeLog

	err := h.db.Scopes(
		repository.ScopeCompanies(companiesID),
		repository.ScopeDataAccess(currentUser, dataScope),
	).Find(&logs).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, logs)
}

func (h *TimeLogHandler) CreateTimeLog(c *gin.Context) {
	var input models.TimeLog
	compID := c.GetUint("companies_id")

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input.CompaniesID = compID

	if err := h.db.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, input)
}

func (h *TimeLogHandler) UpdateTimeLog(c *gin.Context) {
	id := c.Param("id")
	compID := c.GetUint("companies_id")
	var log models.TimeLog

	// Ensure the record belongs to the user's company before updating
	result := h.db.Scopes(repository.ScopeCompanies(compID)).First(&log, id)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
		return
	}

	if err := c.ShouldBindJSON(&log); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.db.Save(&log)
	c.JSON(http.StatusOK, log)
}

func (h *TimeLogHandler) DeleteTimeLog(c *gin.Context) {
	id := c.Param("id")
	compID := c.GetUint("companies_id")

	// Scoped delete to prevent cross-tenant deletion
	result := h.db.Scopes(repository.ScopeCompanies(compID)).Delete(&models.TimeLog{}, id)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

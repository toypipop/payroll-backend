package payroll

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"
	"time"

	"github.com/gin-gonic/gin"
)

func GetPayrollCycles(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var cycles []models.PayrollCycle
	if err := config.DB.Scopes(repository.ApplyContextScopes(c, currentUser, dataScope, "payroll_cycle.companies_id", "unused")).
		Preload("Companies").
		Order("created_at DESC").Find(&cycles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, cycles)
}

func CreatePayrollCycle(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var input struct {
		CycleName   string    `json:"cycle_name" binding:"required"`
		StartDate   time.Time `json:"start_date" binding:"required"`
		EndDate     time.Time `json:"end_date" binding:"required"`
		CompaniesID uint      `json:"companies_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !input.EndDate.After(input.StartDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end_date must be after start_date"})
		return
	}

	// Resolve target company — validate access via ApplyScopes
	targetCompaniesID, ok := repository.ResolveCompanyID(currentUser, dataScope, input.CompaniesID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to the specified company"})
		return
	}

	cycle := models.PayrollCycle{
		CompaniesID: targetCompaniesID,
		CycleName:   input.CycleName,
		StartDate:   input.StartDate,
		EndDate:     input.EndDate,
	}

	if err := config.DB.Create(&cycle).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create payroll cycle"})
		return
	}

	c.JSON(http.StatusCreated, cycle)
}

func ClosePayrollCycle(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	cycleID := c.Param("id")
	var cycle models.PayrollCycle
	if err := config.DB.Scopes(repository.ApplyContextScopes(c, currentUser, dataScope, "payroll_cycle.companies_id", "unused")).
		First(&cycle, cycleID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Payroll cycle not found"})
		return
	}

	if cycle.IsClosed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cycle is already closed"})
		return
	}

	processedCount, skippedCount, snapshotErr := runPayrollSnapshot(cycle.CompaniesID, cycle)
	if snapshotErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to snapshot transactions before closing cycle: " + snapshotErr.Error()})
		return
	}

	if err := config.DB.Model(&cycle).Update("is_closed", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to close cycle"})
		return
	}

	cycle.IsClosed = true
	c.JSON(http.StatusOK, gin.H{
		"message":         "Cycle closed successfully and transactions snapshotted",
		"data":            cycle,
		"processed_count": processedCount,
		"skipped_count":   skippedCount,
	})
}

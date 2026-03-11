package handlers

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

func GetCycleEarnings(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	cycleID := c.Query("cycle_id")
	if cycleID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cycle_id query param required"})
		return
	}

	var items []models.PayrollCycleEarning
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "payroll_cycle_earnings.companies_id", "payroll_cycle_earnings.employees_id"),
	).
		Preload("EarningType").
		Preload("Employees").
		Where("cycle_id = ?", cycleID).
		Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func CreateCycleEarning(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, companies_id").First(&currentUser, userID)

	var input models.PayrollCycleEarning
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate that the EarningType is variable (not permanent)
	var et models.EarningType
	if err := config.DB.First(&et, input.EarningTypeID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Earning type not found"})
		return
	}
	if et.IsPermanent {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Fixed earning types are assigned via Employee Earnings, not per-cycle"})
		return
	}

	// Validate cycle belongs to company
	if err := repository.EnsurePayrollCycleEditable(input.CycleID, currentUser.CompaniesID); err != nil {
		if err == repository.ErrPayrollCycleNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cycle not found or access denied"})
			return
		}
		if err == repository.ErrPayrollCycleClosed {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot add earnings to a closed cycle"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	input.ID = 0
	input.CompaniesID = currentUser.CompaniesID

	if err := config.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create cycle earning"})
		return
	}
	config.DB.Preload("EarningType").Preload("Employees").First(&input, input.ID)
	c.JSON(http.StatusCreated, input)
}

func UpdateCycleEarning(c *gin.Context) {
	id := c.Param("id")
	userID, _ := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, companies_id").First(&currentUser, userID)

	var item models.PayrollCycleEarning
	if err := config.DB.Where("id = ? AND companies_id = ?", id, currentUser.CompaniesID).
		First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or access denied"})
		return
	}
	if err := repository.EnsurePayrollCycleEditable(item.CycleID, currentUser.CompaniesID); err != nil {
		if err == repository.ErrPayrollCycleClosed {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot update earnings in a closed cycle"})
			return
		}
		if err == repository.ErrPayrollCycleNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cycle not found or access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var input models.PayrollCycleEarning
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := config.DB.Model(&item).Updates(map[string]interface{}{
		"amount":          input.Amount,
		"earning_type_id": input.EarningTypeID,
		"employees_id":    input.EmployeesID,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed"})
		return
	}
	config.DB.Preload("EarningType").Preload("Employees").First(&item, item.ID)
	c.JSON(http.StatusOK, item)
}

func DeleteCycleEarning(c *gin.Context) {
	id := c.Param("id")
	userID, _ := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, companies_id").First(&currentUser, userID)

	result := config.DB.Where("id = ? AND companies_id = ?", id, currentUser.CompaniesID).
		First(&models.PayrollCycleEarning{})
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or unauthorized"})
		return
	}

	var item models.PayrollCycleEarning
	if err := config.DB.Where("id = ? AND companies_id = ?", id, currentUser.CompaniesID).First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or unauthorized"})
		return
	}
	if err := repository.EnsurePayrollCycleEditable(item.CycleID, currentUser.CompaniesID); err != nil {
		if err == repository.ErrPayrollCycleClosed {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete earnings in a closed cycle"})
			return
		}
		if err == repository.ErrPayrollCycleNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cycle not found or access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result = config.DB.Where("id = ? AND companies_id = ?", id, currentUser.CompaniesID).
		Delete(&models.PayrollCycleEarning{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or unauthorized"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

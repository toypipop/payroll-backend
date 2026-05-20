package payroll

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
		repository.ApplyContextScopes(c, currentUser, dataScope, "payroll_cycle_earning.companies_id", "payroll_cycle_earning.employees_id"),
	).
		Preload("EarningType").
		Preload("Employees").
		Preload("Employees.Companies").
		Where("cycle_id = ?", cycleID).
		Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func CreateCycleEarning(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var input models.PayrollCycleEarning
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var et models.EarningType
	if err := config.DB.First(&et, input.EarningTypeID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Earning type not found"})
		return
	}

	cycle, err := repository.EnsurePayrollCycleOpen(input.CycleID, currentUser, dataScope)
	if err != nil {
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
	input.CompaniesID = cycle.CompaniesID

	if err := config.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create cycle earning"})
		return
	}
	config.DB.Preload("EarningType").Preload("Employees").First(&input, input.ID)
	c.JSON(http.StatusCreated, input)
}

func UpdateCycleEarning(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var item models.PayrollCycleEarning
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "payroll_cycle_earning.companies_id", "payroll_cycle_earning.employees_id"),
	).First(&item, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or access denied"})
		return
	}

	if _, err := repository.EnsurePayrollCycleOpen(item.CycleID, currentUser, dataScope); err != nil {
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
	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var item models.PayrollCycleEarning
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "payroll_cycle_earning.companies_id", "payroll_cycle_earning.employees_id"),
	).First(&item, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or unauthorized"})
		return
	}

	if _, err := repository.EnsurePayrollCycleOpen(item.CycleID, currentUser, dataScope); err != nil {
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

	if err := config.DB.Delete(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

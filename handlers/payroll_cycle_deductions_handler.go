package handlers

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

func GetCycleDeductions(c *gin.Context) {
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

	var items []models.PayrollCycleDeduction
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "payroll_cycle_deductions.companies_id", "payroll_cycle_deductions.employees_id"),
	).
		Preload("DeductionType").
		Preload("Employees").
		Where("cycle_id = ?", cycleID).
		Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func CreateCycleDeduction(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, companies_id").First(&currentUser, userID)

	var input models.PayrollCycleDeduction
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate deduction type exists and belongs to the same company.
	var dt models.DeductionType
	if err := config.DB.Where("id = ? AND companies_id = ?", input.DeductionTypeID, currentUser.CompaniesID).
		First(&dt).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Deduction type not found or access denied"})
		return
	}

	// Validate cycle belongs to company.
	if err := repository.EnsurePayrollCycleEditable(input.CycleID, currentUser.CompaniesID); err != nil {
		if err == repository.ErrPayrollCycleNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cycle not found or access denied"})
			return
		}
		if err == repository.ErrPayrollCycleClosed {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot add deductions to a closed cycle"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	input.ID = 0
	input.CompaniesID = currentUser.CompaniesID

	if err := config.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create cycle deduction"})
		return
	}
	config.DB.Preload("DeductionType").Preload("Employees").First(&input, input.ID)
	c.JSON(http.StatusCreated, input)
}

func UpdateCycleDeduction(c *gin.Context) {
	id := c.Param("id")
	userID, _ := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, companies_id").First(&currentUser, userID)

	var item models.PayrollCycleDeduction
	if err := config.DB.Where("id = ? AND companies_id = ?", id, currentUser.CompaniesID).
		First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or access denied"})
		return
	}
	if err := repository.EnsurePayrollCycleEditable(item.CycleID, currentUser.CompaniesID); err != nil {
		if err == repository.ErrPayrollCycleClosed {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot update deductions in a closed cycle"})
			return
		}
		if err == repository.ErrPayrollCycleNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cycle not found or access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var input models.PayrollCycleDeduction
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := config.DB.Model(&item).Updates(map[string]interface{}{
		"amount":            input.Amount,
		"deduction_type_id": input.DeductionTypeID,
		"employees_id":      input.EmployeesID,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed"})
		return
	}
	config.DB.Preload("DeductionType").Preload("Employees").First(&item, item.ID)
	c.JSON(http.StatusOK, item)
}

func DeleteCycleDeduction(c *gin.Context) {
	id := c.Param("id")
	userID, _ := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, companies_id").First(&currentUser, userID)

	var item models.PayrollCycleDeduction
	if err := config.DB.Where("id = ? AND companies_id = ?", id, currentUser.CompaniesID).First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or unauthorized"})
		return
	}
	if err := repository.EnsurePayrollCycleEditable(item.CycleID, currentUser.CompaniesID); err != nil {
		if err == repository.ErrPayrollCycleClosed {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete deductions in a closed cycle"})
			return
		}
		if err == repository.ErrPayrollCycleNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cycle not found or access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result := config.DB.Where("id = ? AND companies_id = ?", id, currentUser.CompaniesID).
		Delete(&models.PayrollCycleDeduction{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or unauthorized"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

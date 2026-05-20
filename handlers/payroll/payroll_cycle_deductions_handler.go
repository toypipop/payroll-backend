package payroll

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
		repository.ApplyContextScopes(c, currentUser, dataScope, "payroll_cycle_deduction.companies_id", "payroll_cycle_deduction.employees_id"),
	).
		Preload("DeductionType").
		Preload("Employees").
		Preload("Employees.Companies").
		Where("cycle_id = ?", cycleID).
		Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func CreateCycleDeduction(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var input models.PayrollCycleDeduction
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate deduction type exists
	var dt models.DeductionType
	if err := config.DB.First(&dt, input.DeductionTypeID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Deduction type not found"})
		return
	}

	cycle, err := repository.EnsurePayrollCycleOpen(input.CycleID, currentUser, dataScope)
	if err != nil {
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
	if input.CompaniesID != 0 && input.CompaniesID != cycle.CompaniesID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Selected company does not match payroll cycle company"})
		return
	}
	input.CompaniesID = cycle.CompaniesID
	if _, ok := repository.ResolveEmployeeForCompany(c, currentUser, dataScope, input.EmployeesID, cycle.CompaniesID); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Employee does not belong to payroll cycle company"})
		return
	}
	if dt.CompaniesID != cycle.CompaniesID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Deduction type does not belong to payroll cycle company"})
		return
	}

	if err := config.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create cycle deduction"})
		return
	}
	config.DB.Preload("DeductionType").Preload("Employees").First(&input, input.ID)
	c.JSON(http.StatusCreated, input)
}

func UpdateCycleDeduction(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var item models.PayrollCycleDeduction
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "payroll_cycle_deduction.companies_id", "payroll_cycle_deduction.employees_id"),
	).First(&item, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or access denied"})
		return
	}

	cycle, err := repository.EnsurePayrollCycleOpen(item.CycleID, currentUser, dataScope)
	if err != nil {
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
	if input.CompaniesID != 0 && input.CompaniesID != cycle.CompaniesID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Selected company does not match payroll cycle company"})
		return
	}
	var dt models.DeductionType
	if err := config.DB.First(&dt, input.DeductionTypeID).Error; err != nil || dt.CompaniesID != cycle.CompaniesID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Deduction type does not belong to payroll cycle company"})
		return
	}
	if _, ok := repository.ResolveEmployeeForCompany(c, currentUser, dataScope, input.EmployeesID, cycle.CompaniesID); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Employee does not belong to payroll cycle company"})
		return
	}

	if err := config.DB.Model(&item).Updates(map[string]interface{}{
		"amount":            input.Amount,
		"companies_id":      cycle.CompaniesID,
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
	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var item models.PayrollCycleDeduction
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "payroll_cycle_deduction.companies_id", "payroll_cycle_deduction.employees_id"),
	).First(&item, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or unauthorized"})
		return
	}

	if _, err := repository.EnsurePayrollCycleOpen(item.CycleID, currentUser, dataScope); err != nil {
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

	if err := config.DB.Delete(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

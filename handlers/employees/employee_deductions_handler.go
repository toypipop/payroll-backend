package employees

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

func GetEmployeeDeductions(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var items []models.EmployeeDeduction
	if err := config.DB.Scopes(
		repository.ApplyEmployeeScopes(c, currentUser, dataScope, "employee_deduction.companies_id", "employee_deduction.employees_id"),
	).
		Preload("DeductionType").
		Preload("Employees").
		Preload("Employees.Companies").
		Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func CreateEmployeeDeduction(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	var input models.EmployeeDeduction
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	input.ID = 0
	targetEmployee, ok := repository.ResolveEmployeeForCompany(c, currentUser, dataScope, input.EmployeesID, input.CompaniesID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Employee does not belong to selected company or access denied"})
		return
	}
	input.CompaniesID = targetEmployee.CompaniesID
	var deductionType models.DeductionType
	if err := config.DB.Scopes(repository.ApplyCompanyScopes(c, currentUser, "deduction_type.companies_id")).
		First(&deductionType, input.DeductionTypeID).Error; err != nil || deductionType.CompaniesID != input.CompaniesID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Deduction type does not belong to selected company"})
		return
	}

	if err := config.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create employee deduction"})
		return
	}
	config.DB.Preload("DeductionType").First(&input, input.ID)
	c.JSON(http.StatusCreated, input)
}

func UpdateEmployeeDeduction(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	var item models.EmployeeDeduction
	if err := config.DB.Scopes(repository.ApplyEmployeeScopes(c, currentUser, dataScope, "employee_deduction.companies_id", "employee_deduction.employees_id")).
		Where("id = ?", id).First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or access denied"})
		return
	}

	var input models.EmployeeDeduction
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	targetEmployee, ok := repository.ResolveEmployeeForCompany(c, currentUser, dataScope, input.EmployeesID, input.CompaniesID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Employee does not belong to selected company or access denied"})
		return
	}
	var deductionType models.DeductionType
	if err := config.DB.Scopes(repository.ApplyCompanyScopes(c, currentUser, "deduction_type.companies_id")).
		First(&deductionType, input.DeductionTypeID).Error; err != nil || deductionType.CompaniesID != targetEmployee.CompaniesID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Deduction type does not belong to selected company"})
		return
	}

	if err := config.DB.Model(&item).Updates(map[string]interface{}{
		"amount":            input.Amount,
		"companies_id":      targetEmployee.CompaniesID,
		"deduction_type_id": input.DeductionTypeID,
		"employees_id":      input.EmployeesID,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed"})
		return
	}
	config.DB.Preload("DeductionType").First(&item, item.ID)
	c.JSON(http.StatusOK, item)
}

func DeleteEmployeeDeduction(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	result := config.DB.Scopes(repository.ApplyEmployeeScopes(c, currentUser, dataScope, "employee_deduction.companies_id", "employee_deduction.employees_id")).
		Where("id = ?", id).
		Delete(&models.EmployeeDeduction{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or unauthorized"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

package employees

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

func GetEmployeeEarnings(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var items []models.EmployeeEarning
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "employee_earnings.companies_id", "employee_earnings.employees_id"),
	).
		Preload("EarningType").
		Preload("Employees").
		Preload("Employees.Companies").
		Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func CreateEmployeeEarning(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, companies_id").First(&currentUser, userID)

	var input models.EmployeeEarning
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	input.ID = 0
	input.CompaniesID = currentUser.CompaniesID

	if err := config.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create employee earning"})
		return
	}
	config.DB.Preload("EarningType").First(&input, input.ID)
	c.JSON(http.StatusCreated, input)
}

func UpdateEmployeeEarning(c *gin.Context) {
	id := c.Param("id")
	userID, _ := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, companies_id").First(&currentUser, userID)

	var item models.EmployeeEarning
	if err := config.DB.Where("id = ? AND companies_id = ?", id, currentUser.CompaniesID).
		First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or access denied"})
		return
	}

	var input models.EmployeeEarning
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
	config.DB.Preload("EarningType").First(&item, item.ID)
	c.JSON(http.StatusOK, item)
}

func DeleteEmployeeEarning(c *gin.Context) {
	id := c.Param("id")
	userID, _ := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, companies_id").First(&currentUser, userID)

	result := config.DB.Where("id = ? AND companies_id = ?", id, currentUser.CompaniesID).
		Delete(&models.EmployeeEarning{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or unauthorized"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

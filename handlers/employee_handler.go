package handlers

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository" // Added repository import

	"github.com/gin-gonic/gin"
)

// GetEmployees fetches all employees within the same company
func GetEmployees(c *gin.Context) {
	// 1. Get company_id from context (English Comment)
	companyID, exists := c.Get("company_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tenant information not found"})
		return
	}

	var employees []models.Employee

	// 2. Apply ScopeCompany before finding data (English Comment)
	result := config.DB.Scopes(repository.ScopeCompany(companyID.(uint))).
		Preload("Department").
		Preload("Position").
		Find(&employees)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, employees)
}

// CreateEmployee adds a new employee record tied to the user's company
func CreateEmployee(c *gin.Context) {
	var input models.Employee
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. Force set CompanyID from token context (English Comment)
	companyID, _ := c.Get("company_id")
	input.CompanyID = companyID.(uint)

	if err := config.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, input)
}

// UpdateEmployee handles updating employee data within the same company
func UpdateEmployee(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("company_id")
	var employee models.Employee

	// 1. Find employee only within the user's company (English Comment)
	if err := config.DB.Scopes(repository.ScopeCompany(companyID.(uint))).First(&employee, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employee not found in your company"})
		return
	}

	var updateData struct {
		EmployeeCode string `json:"employee_code"`
		FirstName    string `json:"first_name"`
		LastName     string `json:"last_name"`
		Email        string `json:"email"`
		DeptID       uint   `json:"dept_id"`
		PosID        uint   `json:"pos_id"`
		Status       string `json:"status"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 2. Perform update (English Comment)
	if err := config.DB.Model(&employee).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed: " + err.Error()})
		return
	}

	config.DB.Preload("Department").Preload("Position").First(&employee, id)
	c.JSON(http.StatusOK, employee)
}

/* --- Delete Employee within company scope (English Comment) --- */
func DeleteEmployee(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("company_id")

	// 1. Ensure the record belongs to the company before deleting (English Comment)
	result := config.DB.Scopes(repository.ScopeCompany(companyID.(uint))).Delete(&models.Employee{}, id)

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or access denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

// GetMyProfile fetches the profile of the logged-in user (OWN scope)
func GetMyProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	var employee models.Employee
	// Profile is already naturally isolated by UserID (English Comment)
	if err := config.DB.Preload("Department").Preload("Position").First(&employee, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employee record not found"})
		return
	}

	c.JSON(http.StatusOK, employee)
}

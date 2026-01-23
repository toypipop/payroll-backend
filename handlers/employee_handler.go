package handlers

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

// GetEmployees updated with Role Preload (English Comment)
func GetEmployees(c *gin.Context) {
	companyID, _ := c.Get("company_id")
	var employees []models.Employee

	result := config.DB.Scopes(repository.ScopeCompany(companyID.(uint))).
		Preload("Department").
		Preload("Position").
		Preload("Role"). // Added Preload for Role name in table
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

/* --- Updated UpdateEmployee Handler (English Comment) --- */
func UpdateEmployee(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("company_id")
	var employee models.Employee

	if err := config.DB.Scopes(repository.ScopeCompany(companyID.(uint))).First(&employee, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employee not found"})
		return
	}

	var updateData struct {
		EmployeeCode string `json:"employee_code"`
		FirstName    string `json:"first_name"`
		LastName     string `json:"last_name"`
		Email        string `json:"email"`
		DeptID       uint   `json:"dept_id"`
		PosID        uint   `json:"pos_id"`
		RoleID       uint   `json:"role_id"`
		IsActive     bool   `json:"is_active"`
		Password     string `json:"password"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Assigning to the object (English Comment)
	employee.EmployeeCode = updateData.EmployeeCode
	employee.FirstName = updateData.FirstName
	employee.LastName = updateData.LastName
	employee.Email = updateData.Email
	employee.DeptID = updateData.DeptID
	employee.PosID = updateData.PosID
	employee.RoleID = updateData.RoleID
	employee.IsActive = updateData.IsActive

	// Simply set password if not empty, BeforeSave hook will handle the rest (English Comment)
	if updateData.Password != "" {
		employee.Password = updateData.Password
	}

	if err := config.DB.Save(&employee).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed"})
		return
	}

	config.DB.Preload("Department").Preload("Position").Preload("Role").First(&employee, id)
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
	userID, _ := c.Get("user_id")
	companyID, _ := c.Get("company_id") // Get from context

	var employee models.Employee
	// Add company scope for strict isolation (English Comment)
	if err := config.DB.Scopes(repository.ScopeCompany(companyID.(uint))).
		Preload("Department").
		Preload("Position").
		First(&employee, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employee record not found"})
		return
	}
	c.JSON(http.StatusOK, employee)
}

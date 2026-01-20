package handlers

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
)

// GetEmployees fetches all employees with their department and position
func GetEmployees(c *gin.Context) {
	var employees []models.Employee

	// Use GORM Preload to fetch related data (Joins)
	result := config.DB.Preload("Department").Preload("Position").Find(&employees)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, employees)
}

// CreateEmployee adds a new employee record
func CreateEmployee(c *gin.Context) {
	var input models.Employee
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Save to database
	if err := config.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, input)
}

// UpdateEmployee handles updating employee data (English Comment)
func UpdateEmployee(c *gin.Context) {
	id := c.Param("id")
	var employee models.Employee

	// 1. Check if employee exists
	if err := config.DB.First(&employee, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employee not found"})
		return
	}

	// 2. Create a struct for binding update data (Exclude ID and Password)
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

	// 3. Use Updates with a map or struct to avoid unique constraint with itself
	// GORM will skip fields that haven't changed
	result := config.DB.Model(&employee).Updates(updateData)

	if result.Error != nil {
		// Return clear error message if code or email is taken by ANOTHER user
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed: " + result.Error.Error()})
		return
	}

	// 4. Reload the employee with associations to return to frontend
	config.DB.Preload("Department").Preload("Position").First(&employee, id)

	c.JSON(http.StatusOK, employee)
}

/* --- Delete Employee (English Comment) --- */
func DeleteEmployee(c *gin.Context) {
	id := c.Param("id")

	// Perform Unscoped() for permanent delete or just Delete() for Soft Delete
	if err := config.DB.Delete(&models.Employee{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete record"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

// GetMyProfile fetches the profile of the logged-in user
func GetMyProfile(c *gin.Context) {
	// 1. Get user_id from context (set by Middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	var employee models.Employee
	// 2. Query only the data belonging to this user (Data Scope: OWN)
	if err := config.DB.First(&employee, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employee record not found"})
		return
	}

	c.JSON(http.StatusOK, employee)
}

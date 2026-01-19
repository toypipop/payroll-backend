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

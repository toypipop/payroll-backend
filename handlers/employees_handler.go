package handlers

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// handlers/employees_handler.go
func GetEmployees(c *gin.Context) {
	companiesID, _ := c.Get("companies_id")
	var employees []models.Employees

	query := config.DB.Debug()

	// 1. Join with roles table to access is_super_admin column (English Comment)
	// 2. Filter out super admins (English Comment)
	if err := query.Scopes(repository.ScopeCompanies(companiesID.(uint), "employees.companies_id")).
		Joins("JOIN roles ON roles.id = employees.roles_id").
		Where("roles.is_super_admin = ?", false).
		Preload("Department").
		Preload("Position").
		Preload("SalariesStructure").
		Find(&employees).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, employees)
}

// CreateEmployees - Create record with roles validation (English Comment)
func CreateEmployees(c *gin.Context) {
	var input models.Employees
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Guard: Prevent assigning Super Admin roles (English Comment)
	var roles models.Roles
	if err := config.DB.First(&roles, input.RolesID).Error; err == nil && roles.IsSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized roles selection"})
		return
	}

	if input.Password != "" {
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(input.Password), 10)
		input.Password = string(hashedPassword)
	}

	companiesID, _ := c.Get("companies_id")
	input.CompaniesID = companiesID.(uint)

	if err := config.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, input)
}

// UpdateEmployees - Standard update with roles guard (English Comment)
func UpdateEmployees(c *gin.Context) {
	id := c.Param("id")
	companiesID, _ := c.Get("companies_id")
	var emp models.Employees

	// If ID belongs to Super Admin, this query will naturally fail due to Scope (English Comment)
	if err := config.DB.Scopes(repository.ScopeCompanies(companiesID.(uint))).First(&emp, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employees not found"})
		return
	}

	var data map[string]interface{}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Guard: Prevent promoting to Super Admin roles (English Comment)
	if rolesID, ok := data["roles_id"].(float64); ok {
		var newRoles models.Roles
		if err := config.DB.First(&newRoles, uint(rolesID)).Error; err == nil && newRoles.IsSuperAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: Cannot promote to Super Admin"})
			return
		}
	}

	if pwd, ok := data["password"].(string); ok && pwd != "" {
		// If password is provided, hash it (English Comment)
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(pwd), 10)
		data["password"] = string(hashedPassword)
	} else {
		// If password is blank or not provided, remove it from map to keep existing password (English Comment)
		delete(data, "password")
	}

	if err := config.DB.Model(&emp).Updates(data).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed"})
		return
	}

	config.DB.Preload("Department").Preload("Position").Preload("SalariesStructure").First(&emp, id)
	c.JSON(http.StatusOK, emp)
}

// DeleteEmployees - Simple delete with ownership validation (English Comment)
func DeleteEmployees(c *gin.Context) {
	id := c.Param("id")
	companiesID, _ := c.Get("companies_id")
	var employees models.Employees

	// ScopeCompanies ensures the user can only delete their own company's employees (English Comment)
	if err := config.DB.Scopes(repository.ScopeCompanies(companiesID.(uint))).First(&employees, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
		return
	}

	if err := config.DB.Delete(&employees).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

package handlers

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// GetEmployees updated with Role Preload (English Comment)
func GetEmployees(c *gin.Context) {
	companiesID, _ := c.Get("companies_id")
	var employees []models.Employee

	result := config.DB.Scopes(repository.ScopeCompanies(companiesID.(uint))).
		Preload("Department").
		Preload("Position").
		Preload("Role").
		Find(&employees)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, employees)
}

// CreateEmployee adds a new employee record tied to the user's companies
func CreateEmployee(c *gin.Context) {
	var input models.Employee
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. Validate: Prevent creating Super Admin (English Comment)
	var role models.Role
	if err := config.DB.First(&role, input.RoleID).Error; err == nil && role.IsSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot create employee with Super Admin role"})
		return
	}

	// --- NEW: Hash Password before Create (English Comment) ---
	if input.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), 10)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt password"})
			return
		}
		input.Password = string(hashedPassword)
	}

	// 2. Force set CompaniesID from token context (English Comment)
	companiesID, _ := c.Get("companies_id")
	input.CompaniesID = companiesID.(uint)

	// 3. Create record (English Comment)
	if err := config.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, input)
}

func UpdateEmployee(c *gin.Context) {
	id := c.Param("id")
	companiesID, _ := c.Get("companies_id")
	var emp models.Employee

	// 1. Fetch & Guard: Block if target is a Super Admin (English Comment)
	err := config.DB.Scopes(repository.ScopeCompanies(companiesID.(uint))).Preload("Role").First(&emp, id).Error
	if err != nil || emp.Role.IsSuperAdmin {
		msg := "Employee not found"
		if err == nil {
			msg = "Forbidden: Cannot modify a Super Admin"
		}
		c.JSON(http.StatusForbidden, gin.H{"error": msg})
		return
	}

	var data map[string]interface{}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// --- NEW: Hash Password for map updates (English Comment) ---
	if pwd, ok := data["password"].(string); ok && pwd != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(pwd), 10)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt password"})
			return
		}
		data["password"] = string(hashedPassword)
	}

	// 2. Guard: Block if trying to change to a Super Admin role (English Comment)
	if roleID, ok := data["role_id"].(float64); ok {
		var newRole models.Role
		if err := config.DB.First(&newRole, uint(roleID)).Error; err == nil && newRole.IsSuperAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: Cannot promote to Super Admin"})
			return
		}
	}

	// 3. Perform Update (English Comment)
	if err := config.DB.Model(&emp).Updates(data).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed"})
		return
	}

	config.DB.Preload("Department").Preload("Position").Preload("Role").First(&emp, id)
	c.JSON(http.StatusOK, emp)
}

func DeleteEmployee(c *gin.Context) {
	id := c.Param("id")
	companiesID, _ := c.Get("companies_id")
	var employee models.Employee

	// 1. Find employee and check their role status first (English Comment)
	if err := config.DB.Scopes(repository.ScopeCompanies(companiesID.(uint))).
		Preload("Role").
		First(&employee, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
		return
	}

	// 2. Prevent deletion of Super Admin (English Comment)
	if employee.Role.IsSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Super Admin records cannot be deleted"})
		return
	}

	// 3. Perform soft delete (English Comment)
	if err := config.DB.Delete(&employee).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

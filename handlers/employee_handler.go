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
		Preload("Role").
		Find(&employees)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, employees)
}

// CreateEmployee adds a new employee record tied to the user's company
// CreateEmployee adds a new employee record tied to the user's company
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

	// 2. Force set CompanyID from token context (English Comment)
	companyID, _ := c.Get("company_id")
	input.CompanyID = companyID.(uint)

	// 3. Create record (English Comment)
	if err := config.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, input)
}
func UpdateEmployee(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("company_id")
	var emp models.Employee

	// 1. Fetch & Guard: Block if target is currently a Super Admin (English Comment)
	err := config.DB.Scopes(repository.ScopeCompany(companyID.(uint))).Preload("Role").First(&emp, id).Error
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
	companyID, _ := c.Get("company_id")
	var employee models.Employee

	// 1. Find employee and check their role status first (English Comment)
	if err := config.DB.Scopes(repository.ScopeCompany(companyID.(uint))).
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

// GetMyProfile fetches the profile of the logged-in user (OWN scope)
func GetMyProfile(c *gin.Context) {
	userID, _ := c.Get("user_id")
	companyID, _ := c.Get("company_id")

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

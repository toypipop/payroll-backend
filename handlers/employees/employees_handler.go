package employees

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func GetEmployees(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var employees []models.Employees
	if err := config.DB.
		Scopes(repository.ApplyScopes(currentUser, dataScope, "employees.companies_id", "employees.id")).
		Preload("Department").
		Preload("Position").
		Preload("Companies").
		Preload("Roles").
		Preload("SalariesStructure").
		Preload("Bank").
		Preload("EmploymentType").
		Find(&employees).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, employees)
}

func CreateEmployees(c *gin.Context) {
	var input models.Employees
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var roles models.Roles
	if err := config.DB.First(&roles, input.RolesID).Error; err == nil && roles.IsSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized roles selection"})
		return
	}

	if input.Password != "" {
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(input.Password), 10)
		input.Password = string(hashedPassword)
	}

	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	if err := config.DB.Select("companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Companies context not found"})
		return
	}

	// ScopeAll (SUPER_ADMIN) can create employees for any company if companies_id is specified
	if dataScope != models.ScopeAll || input.CompaniesID == 0 {
		input.CompaniesID = currentUser.CompaniesID
	}

	// Resolve bank_name from bank_id if provided
	if input.BankID != nil && *input.BankID != 0 {
		var bank models.Bank
		if err := config.DB.First(&bank, *input.BankID).Error; err == nil {
			input.BankName = bank.BankName
		}
	}

	if err := config.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, input)
}

func UpdateEmployees(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var emp models.Employees
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "employees.companies_id", "employees.id"),
	).First(&emp, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employees not found or access denied"})
		return
	}

	var data map[string]interface{}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if rolesID, ok := data["roles_id"].(float64); ok {
		var newRoles models.Roles
		if err := config.DB.First(&newRoles, uint(rolesID)).Error; err == nil && newRoles.IsSuperAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: Cannot promote to Super Admin"})
			return
		}
	}

	if pwd, ok := data["password"].(string); ok && pwd != "" {
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(pwd), 10)
		data["password"] = string(hashedPassword)
	} else {
		delete(data, "password")
	}

	// Handle bank_id: convert float64 (from JSON) to *uint, or explicit null → nil + clear bank_name
	switch v := data["bank_id"].(type) {
	case float64:
		bankIDUint := uint(v)
		if bankIDUint == 0 {
			data["bank_id"] = gorm.Expr("NULL")
			data["bank_name"] = ""
		} else {
			data["bank_id"] = bankIDUint
			var bank models.Bank
			if err := config.DB.First(&bank, bankIDUint).Error; err == nil {
				data["bank_name"] = bank.BankName
			}
		}
	case nil:
		if _, exists := data["bank_id"]; exists {
			data["bank_id"] = gorm.Expr("NULL")
			data["bank_name"] = ""
		}
	}

	if err := config.DB.Model(&emp).Updates(data).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed"})
		return
	}

	config.DB.Preload("Department").Preload("Position").Preload("SalariesStructure").Preload("Bank").First(&emp, id)
	c.JSON(http.StatusOK, emp)
}

func DeleteEmployees(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var employees models.Employees
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "employees.companies_id", "employees.id"),
	).First(&employees, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or access denied"})
		return
	}

	if err := config.DB.Delete(&employees).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

package employees

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"
	"time"

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

	// Define current time for filtering effective date
	now := time.Now()

	if err := config.DB.Debug().Scopes(
		repository.ApplyScopes(currentUser, dataScope, "employees.companies_id", "employees.id"),
	).
		Joins("JOIN roles ON roles.id = employees.roles_id").
		Where("roles.is_super_admin = ?", false).
		Preload("Department").
		Preload("Position").
		Preload("Companies").
		// Filter only valid salary records that are already effective (English Comment)
		Preload("SalariesStructure", func(db *gorm.DB) *gorm.DB {
			return db.Where("effective_date <= ? AND deleted_at = 0", now).
				Order("effective_date DESC, created_at DESC")
		}).
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

	if err := config.DB.Model(&emp).Updates(data).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed"})
		return
	}

	config.DB.Preload("Department").Preload("Position").Preload("SalariesStructure").First(&emp, id)
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

// GetEmployeeBankAccount returns the bank account info for a single employee.
func GetEmployeeBankAccount(c *gin.Context) {
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
		c.JSON(http.StatusNotFound, gin.H{"error": "Employee not found or access denied"})
		return
	}

	var bank models.EmployeeBankAccount
	if err := config.DB.Where("employees_id = ? AND companies_id = ?", emp.ID, emp.CompaniesID).First(&bank).Error; err != nil {
		// Return an empty record so the frontend knows there is no bank account yet
		c.JSON(http.StatusOK, models.EmployeeBankAccount{EmployeesID: emp.ID, CompaniesID: emp.CompaniesID})
		return
	}
	c.JSON(http.StatusOK, bank)
}

// UpsertEmployeeBankAccount creates or updates the bank account for a single employee.
func UpsertEmployeeBankAccount(c *gin.Context) {
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
		c.JSON(http.StatusNotFound, gin.H{"error": "Employee not found or access denied"})
		return
	}

	var input struct {
		BankID      *uint  `json:"bank_id"`
		AccountName string `json:"account_name"`
		AccountNo   string `json:"account_no"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.AccountName == "" || input.AccountNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Account name and account number are required"})
		return
	}

	var bank models.EmployeeBankAccount
	result := config.DB.Where("employees_id = ? AND companies_id = ?", emp.ID, emp.CompaniesID).First(&bank)

	// Resolve bank name from bank_id if provided
	bankName := ""
	if input.BankID != nil && *input.BankID > 0 {
		var b models.Bank
		if err := config.DB.First(&b, *input.BankID).Error; err == nil {
			bankName = b.BankName
		}
	}

	if result.Error != nil {
		// Create new record
		bank = models.EmployeeBankAccount{
			CompaniesID: emp.CompaniesID,
			EmployeesID: emp.ID,
			BankID:      input.BankID,
			BankName:    bankName,
			AccountName: input.AccountName,
			AccountNo:   input.AccountNo,
		}
		if err := config.DB.Create(&bank).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save bank account"})
			return
		}
	} else {
		// Update existing record - use Updates map to handle nullable bank_id
		updates := map[string]interface{}{
			"bank_id":      input.BankID,
			"bank_name":    bankName,
			"account_name": input.AccountName,
			"account_no":   input.AccountNo,
		}
		if err := config.DB.Model(&bank).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update bank account"})
			return
		}
	}

	c.JSON(http.StatusOK, bank)
}

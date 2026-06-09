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
		Scopes(repository.ApplyContextScopes(c, currentUser, dataScope, "employees.companies_id", "employees.id")).
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

	if input.Password != "" {
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
		input.Password = string(hashedPassword)
	}

	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Companies context not found"})
		return
	}

	companyID, ok := repository.ResolveCompanyID(currentUser, dataScope, input.CompaniesID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for this company"})
		return
	}
	input.CompaniesID = companyID

	var roles models.Roles
	if err := config.DB.First(&roles, input.RolesID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Role not found"})
		return
	}
	if roles.CompaniesID != input.CompaniesID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Role does not belong to selected company"})
		return
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
	mapping := models.EmployeeCompanyAccess{
		EmployeeID: input.ID,
		CompanyID:  input.CompaniesID,
		RoleID:     input.RolesID,
		IsDefault:  true,
		IsActive:   true,
	}
	config.DB.Where(models.EmployeeCompanyAccess{EmployeeID: input.ID, CompanyID: input.CompaniesID, RoleID: input.RolesID}).
		Assign(mapping).
		FirstOrCreate(&mapping)

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
		repository.ApplyContextScopes(c, currentUser, dataScope, "employees.companies_id", "employees.id"),
	).First(&emp, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employees not found or access denied"})
		return
	}

	var data map[string]interface{}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	nextCompanyID := emp.CompaniesID
	if companiesID, ok := data["companies_id"].(float64); ok {
		nextCompanyID = uint(companiesID)
	}
	companyID, ok := repository.ResolveCompanyID(currentUser, dataScope, nextCompanyID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for this company"})
		return
	}
	data["companies_id"] = companyID

	nextRoleID := emp.RolesID
	if rolesID, ok := data["roles_id"].(float64); ok {
		nextRoleID = uint(rolesID)
	}
	var role models.Roles
	if err := config.DB.First(&role, nextRoleID).Error; err != nil || role.CompaniesID != companyID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Role does not belong to selected company"})
		return
	}

	if pwd, ok := data["password"].(string); ok && pwd != "" {
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(pwd), 12)
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
	mapping := models.EmployeeCompanyAccess{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
		RoleID:     nextRoleID,
		IsDefault:  true,
		IsActive:   true,
	}
	config.DB.Model(&models.EmployeeCompanyAccess{}).
		Where("employee_id = ?", emp.ID).
		Update("is_default", false)
	config.DB.Where(models.EmployeeCompanyAccess{EmployeeID: emp.ID, CompanyID: companyID, RoleID: nextRoleID}).
		Assign(mapping).
		FirstOrCreate(&mapping)

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
		repository.ApplyContextScopes(c, currentUser, dataScope, "employees.companies_id", "employees.id"),
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

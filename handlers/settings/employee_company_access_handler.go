package settings

import (
	"errors"
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type employeeCompanyAccessInput struct {
	EmployeeID uint `json:"employee_id" binding:"required"`
	CompanyID  uint `json:"company_id" binding:"required"`
	RoleID     uint `json:"role_id" binding:"required"`
	IsDefault  bool `json:"is_default"`
	IsActive   bool `json:"is_active"`
}

func GetEmployeeCompanyAccess(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var mappings []models.EmployeeCompanyRole
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "employee_company_role.company_id", "employee_company_role.employee_id"),
	).
		Preload("Employee").
		Preload("Company").
		Preload("Role").
		Order("employee_company_role.employee_id ASC, employee_company_role.is_default DESC, employee_company_role.company_id ASC").
		Find(&mappings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch employee company access"})
		return
	}

	c.JSON(http.StatusOK, mappings)
}

func CreateEmployeeCompanyAccess(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var input employeeCompanyAccessInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mapping, err := buildEmployeeCompanyAccess(c, currentUser, dataScope, input)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	mapping.ID = 0

	err = config.DB.Transaction(func(tx *gorm.DB) error {
		if mapping.IsDefault {
			if err := tx.Model(&models.EmployeeCompanyRole{}).
				Where("employee_id = ?", mapping.EmployeeID).
				Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Create(&mapping).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create employee company access"})
		return
	}

	config.DB.Preload("Employee").Preload("Company").Preload("Role").First(&mapping, mapping.ID)
	c.JSON(http.StatusCreated, mapping)
}

func UpdateEmployeeCompanyAccess(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var existing models.EmployeeCompanyRole
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "employee_company_role.company_id", "employee_company_role.employee_id"),
	).First(&existing, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Access mapping not found"})
		return
	}

	var input employeeCompanyAccessInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mapping, err := buildEmployeeCompanyAccess(c, currentUser, dataScope, input)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	err = config.DB.Transaction(func(tx *gorm.DB) error {
		if mapping.IsDefault {
			if err := tx.Model(&models.EmployeeCompanyRole{}).
				Where("employee_id = ? AND id <> ?", mapping.EmployeeID, existing.ID).
				Update("is_default", false).Error; err != nil {
				return err
			}
		}

		return tx.Model(&existing).Updates(map[string]interface{}{
			"employee_id": mapping.EmployeeID,
			"company_id":  mapping.CompanyID,
			"role_id":     mapping.RoleID,
			"is_default":  mapping.IsDefault,
			"is_active":   mapping.IsActive,
		}).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update employee company access"})
		return
	}

	config.DB.Preload("Employee").Preload("Company").Preload("Role").First(&existing, existing.ID)
	c.JSON(http.StatusOK, existing)
}

func DeleteEmployeeCompanyAccess(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	result := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "employee_company_role.company_id", "employee_company_role.employee_id"),
	).Where("id = ?", id).Delete(&models.EmployeeCompanyRole{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Access mapping not found or unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

func buildEmployeeCompanyAccess(c *gin.Context, currentUser models.Employees, dataScope models.DataScope, input employeeCompanyAccessInput) (models.EmployeeCompanyRole, error) {
	if _, ok := repository.ResolveCompanyID(currentUser, dataScope, input.CompanyID); !ok {
		return models.EmployeeCompanyRole{}, errors.New("access denied for this company")
	}

	var employee models.Employees
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "employees.companies_id", "employees.id"),
	).Select("id").First(&employee, input.EmployeeID).Error; err != nil {
		return models.EmployeeCompanyRole{}, errors.New("access denied to this employee")
	}

	var role models.Roles
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "roles.companies_id", "roles.id"),
	).First(&role, input.RoleID).Error; err != nil {
		return models.EmployeeCompanyRole{}, errors.New("access denied to this role")
	}
	if role.CompaniesID != input.CompanyID {
		return models.EmployeeCompanyRole{}, errors.New("role does not belong to selected company")
	}

	if input.IsDefault && !input.IsActive {
		return models.EmployeeCompanyRole{}, errors.New("default access must be active")
	}

	return models.EmployeeCompanyRole{
		EmployeeID: input.EmployeeID,
		CompanyID:  input.CompanyID,
		RoleID:     input.RoleID,
		IsDefault:  input.IsDefault,
		IsActive:   input.IsActive,
	}, nil
}

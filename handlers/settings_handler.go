package handlers

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

// --- ROLES ---
func GetRoles(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var roles []models.Roles
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "roles.companies_id", "roles.id"),
	).
		Preload("Companies").
		Where("is_super_admin = ?", false).
		Find(&roles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch roles"})
		return
	}

	c.JSON(http.StatusOK, roles)
}

// --- DEPARTMENTS ---
func GetDepartments(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var departments []models.Department
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "departments.companies_id", "departments.id"),
	).
		Preload("Companies").
		Find(&departments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch departments"})
		return
	}
	c.JSON(http.StatusOK, departments)
}

func CreateDepartment(c *gin.Context) {
	var dept models.Department
	if err := c.ShouldBindJSON(&dept); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("companies_id").First(&currentUser, userID)

	dept.CompaniesID = currentUser.CompaniesID

	if err := config.DB.Create(&dept).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create department"})
		return
	}
	c.JSON(http.StatusCreated, dept)
}

func UpdateDepartment(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var dept models.Department
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "departments.companies_id", "departments.id"),
	).Where("id = ?", id).First(&dept).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Department not found or unauthorized"})
		return
	}

	if err := c.ShouldBindJSON(&dept); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dept.CompaniesID = currentUser.CompaniesID
	config.DB.Save(&dept)
	c.JSON(http.StatusOK, dept)
}

func DeleteDepartment(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	result := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "departments.companies_id", "departments.id"),
	).Where("id = ?", id).Delete(&models.Department{})

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Department not found or unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Department deleted successfully"})
}

// --- POSITIONS ---

func GetPositions(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	var positions []models.Position
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "positions.companies_id", "positions.id"),
	).
		Preload("Companies").
		Find(&positions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch positions"})
		return
	}
	c.JSON(http.StatusOK, positions)
}

func CreatePosition(c *gin.Context) {
	var pos models.Position
	if err := c.ShouldBindJSON(&pos); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("companies_id").First(&currentUser, userID)

	pos.CompaniesID = currentUser.CompaniesID

	if err := config.DB.Create(&pos).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create position"})
		return
	}
	c.JSON(http.StatusCreated, pos)
}

func UpdatePosition(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	var pos models.Position
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "positions.companies_id", "positions.id"),
	).Where("id = ?", id).First(&pos).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position not found or unauthorized"})
		return
	}

	if err := c.ShouldBindJSON(&pos); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pos.CompaniesID = currentUser.CompaniesID
	config.DB.Save(&pos)
	c.JSON(http.StatusOK, pos)
}

func DeletePosition(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	result := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "positions.companies_id", "positions.id"),
	).Where("id = ?", id).Delete(&models.Position{})

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position not found or unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Position deleted successfully"})
}

// --- COMPANIES ---

func GetCompanies(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var companies []models.Companies
	// For Companies table, we use "id" as the companies filter column
	dbResult := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "id", "id"),
	).Find(&companies)

	if dbResult.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": dbResult.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, companies)
}

func GetAllCompanies(c *gin.Context) {
	// This is typically for Super Admin, but we can apply ApplyScopes with ScopeAll logic
	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	var companies []models.Companies
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "id", "id"),
	).Order("id desc").Find(&companies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch companies"})
		return
	}

	c.JSON(http.StatusOK, companies)
}

func CreateCompanies(c *gin.Context) {
	var companies models.Companies
	if err := c.ShouldBindJSON(&companies); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := config.DB.Create(&companies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create companies"})
		return
	}
	c.JSON(http.StatusCreated, companies)
}

func UpdateCompanies(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	var companies models.Companies
	// Ensure the user can only update the companies they belong to (unless ScopeAll)
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "id", "id"),
	).First(&companies, id).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this companies"})
		return
	}

	var updateData struct {
		Name    string `json:"name"`
		TaxID   string `json:"tax_id"`
		Address string `json:"address"`
		Phone   string `json:"phone"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config.DB.Model(&companies).Updates(updateData)
	c.JSON(http.StatusOK, companies)
}

func DeleteCompanies(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	// Apply strict scope for deletion
	result := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "id", "id"),
	).Delete(&models.Companies{}, id)

	if result.RowsAffected == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "Delete failed or unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

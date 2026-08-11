package settings

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

// --- DEPARTMENTS ---
func GetDepartments(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var departments []models.Department
	if err := config.DB.Scopes(
		repository.ApplyCompanyScopes(c, currentUser, "department.companies_id"),
	).Preload("Companies").Preload("Category").Find(&departments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch departments"})
		return
	}
	c.JSON(http.StatusOK, departments)
}

// GetDepartmentOptions returns a minimal id/name list for dropdowns. Ungated
// by page permission (mirrors GetCompanies) since any role filling out an
// employee-linked form needs to know whether departments exist for a
// company, not just users who can manage DepartmentSettings.
func GetDepartmentOptions(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	type option struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		CompaniesID uint   `json:"companies_id"`
	}

	var departments []models.Department
	if err := config.DB.Scopes(
		repository.ApplyCompanyScopes(c, currentUser, "department.companies_id"),
	).Find(&departments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch departments"})
		return
	}

	options := make([]option, 0, len(departments))
	for _, d := range departments {
		options = append(options, option{ID: d.ID, Name: d.DeptName, CompaniesID: d.CompaniesID})
	}
	c.JSON(http.StatusOK, options)
}

func CreateDepartment(c *gin.Context) {
	var dept models.Department
	if err := c.ShouldBindJSON(&dept); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if dept.CategoryID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category_id is required"})
		return
	}

	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	companyID, ok := repository.ResolveCompanyID(c, currentUser, dataScope, dept.CompaniesID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for this company"})
		return
	}
	dept.CompaniesID = companyID

	if err := config.DB.Create(&dept).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create department"})
		return
	}
	c.JSON(http.StatusCreated, dept)
}

func UpdateDepartment(c *gin.Context) {
	id := c.Param("id")
	userID, _ := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var dept models.Department
	if err := config.DB.Scopes(
		repository.ApplyCompanyScopes(c, currentUser, "department.companies_id"),
	).Where("id = ?", id).First(&dept).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Department not found or unauthorized"})
		return
	}

	var input struct {
		DeptName    string `json:"dept_name" binding:"required"`
		CompaniesID uint   `json:"companies_id"`
		CategoryID  uint   `json:"category_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Department name is non-calculation metadata and can be updated even if used.
	// Company is immutable after creation — ignore companies_id from the request.
	dept.DeptName = input.DeptName
	dept.CategoryID = input.CategoryID
	config.DB.Save(&dept)
	c.JSON(http.StatusOK, dept)
}

func DeleteDepartment(c *gin.Context) {
	id := c.Param("id")
	userID, _ := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	var dept models.Department
	if err := config.DB.Scopes(
		repository.ApplyCompanyScopes(c, currentUser, "department.companies_id"),
	).Where("id = ?", id).First(&dept).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Department not found or unauthorized"})
		return
	}

	result := config.DB.Scopes(
		repository.ApplyCompanyScopes(c, currentUser, "department.companies_id"),
	).Where("id = ?", id).Delete(&models.Department{})

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Department not found or unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Department deleted successfully"})
}

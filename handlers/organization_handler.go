package handlers

import (
	"fmt"
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

/* --- Master Data Handlers (English Comments) --- */

// GetRoles fetches all available roles excluding Super Admins (English Comment)
func GetRoles(c *gin.Context) {
	var roles []models.Roles
	if err := config.DB.Where("is_super_admin = ?", false).Find(&roles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch roles"})
		return
	}

	c.JSON(http.StatusOK, roles)
}

// GetDepartments - Fetch departments within the companies scope (English Comment)
func GetDepartments(c *gin.Context) {
	companiesID, _ := c.Get("companies_id")
	var departments []models.Department

	// Apply ScopeCompanies and Preload Companies details (English Comment)
	if err := config.DB.Scopes(repository.ScopeCompanies(companiesID.(uint))).
		Preload("Companies").
		Find(&departments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch departments"})
		return
	}
	c.JSON(http.StatusOK, departments)
}

// CreateDepartment - Create a new department for the user's companies (English Comment)
func CreateDepartment(c *gin.Context) {
	var dept models.Department
	if err := c.ShouldBindJSON(&dept); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Force CompaniesID from token to ensure data integrity (English Comment)
	companiesID, _ := c.Get("companies_id")
	dept.CompaniesID = companiesID.(uint)

	if err := config.DB.Create(&dept).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create department"})
		return
	}
	c.JSON(http.StatusCreated, dept)
}

// UpdateDepartment - Update department with ownership validation via Scope (English Comment)
func UpdateDepartment(c *gin.Context) {
	id := c.Param("id")
	companiesID, _ := c.Get("companies_id")
	var dept models.Department

	// Use Scopes to ensure the user can only find their companies's record (English Comment)
	if err := config.DB.Scopes(repository.ScopeCompanies(companiesID.(uint))).
		Where("id = ?", id).First(&dept).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Department not found or unauthorized"})
		return
	}

	if err := c.ShouldBindJSON(&dept); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Re-assign CompaniesID to prevent unauthorized change (English Comment)
	dept.CompaniesID = companiesID.(uint)

	config.DB.Save(&dept)
	c.JSON(http.StatusOK, dept)
}

// DeleteDepartment - Remove department within the allowed companies scope (English Comment)
func DeleteDepartment(c *gin.Context) {
	id := c.Param("id")
	companiesID, _ := c.Get("companies_id")

	// Use Scopes to prevent deleting records from other companies (English Comment)
	result := config.DB.Scopes(repository.ScopeCompanies(companiesID.(uint))).
		Where("id = ?", id).Delete(&models.Department{})

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Department not found or unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Department deleted successfully"})
}

// GetPositions - Fetch positions within the companies scope (English Comment)
func GetPositions(c *gin.Context) {
	compID, _ := c.Get("companies_id")
	var positions []models.Position

	// Use Scopes to filter by companies and Preload for companies details (English Comment)
	if err := config.DB.Scopes(repository.ScopeCompanies(compID.(uint))).
		Preload("Companies").
		Find(&positions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch positions"})
		return
	}
	c.JSON(http.StatusOK, positions)
}

// CreatePosition - Create a new position for the companies (English Comment)
func CreatePosition(c *gin.Context) {
	var pos models.Position
	if err := c.ShouldBindJSON(&pos); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Force CompaniesID from token context (English Comment)
	compID, _ := c.Get("companies_id")
	pos.CompaniesID = compID.(uint)

	if err := config.DB.Create(&pos).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create position"})
		return
	}
	c.JSON(http.StatusCreated, pos)
}

// UpdatePosition - Update position data with ownership check (English Comment)
func UpdatePosition(c *gin.Context) {
	id := c.Param("id")
	compID, _ := c.Get("companies_id")
	var pos models.Position

	// Check ownership using Scope before updating (English Comment)
	if err := config.DB.Scopes(repository.ScopeCompanies(compID.(uint))).
		Where("id = ?", id).First(&pos).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position not found or unauthorized"})
		return
	}

	if err := c.ShouldBindJSON(&pos); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Ensure CompaniesID remains the same (English Comment)
	pos.CompaniesID = compID.(uint)

	config.DB.Save(&pos)
	c.JSON(http.StatusOK, pos)
}

// DeletePosition - Delete position with companies scope validation (English Comment)
func DeletePosition(c *gin.Context) {
	id := c.Param("id")
	compID, _ := c.Get("companies_id")

	// Delete with Scope to prevent deleting other companies's data (English Comment)
	result := config.DB.Scopes(repository.ScopeCompanies(compID.(uint))).
		Where("id = ?", id).Delete(&models.Position{})

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position not found or unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Position deleted successfully"})
}

func GetCompanies(c *gin.Context) {
	fmt.Println(">>>> GET_COMPANY_START <<<<") // พิมพ์ให้เห็นชัดๆ ใน Terminal

	val, exists := c.Get("companies_id")
	if !exists {
		fmt.Println("!!!! ERROR: No companies_id in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	compID := val.(uint)
	fmt.Printf(">>>> DEBUG: CompID = %d, Type = %T\n", compID, val)

	var companies []models.Companies

	// บังคับ Debug และเก็บ Result เพื่อเช็ค Error
	dbResult := config.DB.Debug().Scopes(repository.ScopeCompanies(compID, "id")).Find(&companies)

	if dbResult.Error != nil {
		fmt.Println("!!!! DB_ERROR:", dbResult.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": dbResult.Error.Error()})
		return
	}

	fmt.Printf(">>>> SUCCESS: Found %d rows\n", dbResult.RowsAffected)
	c.JSON(http.StatusOK, companies)
}

func GetAllCompanies(c *gin.Context) {
	var companies []models.Companies

	if err := config.DB.Order("id desc").Find(&companies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch all companies"})
		return
	}

	c.JSON(http.StatusOK, companies)
}

// POST: Create new companies
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

// PUT: Update companies (Own Companies Only)
func UpdateCompanies(c *gin.Context) {
	userCompaniesID, _ := c.Get("companies_id")
	targetID := c.Param("id")

	var companies models.Companies

	// Verify if the target ID matches the user's authorized companies ID
	if targetID != fmt.Sprintf("%v", userCompaniesID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only update your own companies"})
		return
	}

	if err := config.DB.First(&companies, targetID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Companies not found"})
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

// DELETE: Remove companies
func DeleteCompanies(c *gin.Context) {
	id := c.Param("id")

	// Perform deletion based on the provided ID
	if err := config.DB.Delete(&models.Companies{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

package settings

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

// --- POSITIONS ---

func GetPositions(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	var positions []models.Position
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "position.companies_id", "unused"),
	).Preload("Companies").Find(&positions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch positions"})
		return
	}
	c.JSON(http.StatusOK, positions)
}

// GetPositionOptions returns a minimal id/name list for dropdowns. Ungated
// by page permission (mirrors GetCompanies) so any role filling out an
// employee-linked form can see whether positions exist for a company.
func GetPositionOptions(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	type option struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		CompaniesID uint   `json:"companies_id"`
	}

	var positions []models.Position
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "position.companies_id", "unused"),
	).Find(&positions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch positions"})
		return
	}

	options := make([]option, 0, len(positions))
	for _, p := range positions {
		options = append(options, option{ID: p.ID, Name: p.PosName, CompaniesID: p.CompaniesID})
	}
	c.JSON(http.StatusOK, options)
}

func CreatePosition(c *gin.Context) {
	var pos models.Position
	if err := c.ShouldBindJSON(&pos); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	companyID, ok := repository.ResolveCompanyID(c, currentUser, dataScope, pos.CompaniesID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for this company"})
		return
	}
	pos.CompaniesID = companyID

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
		repository.ApplyContextScopes(c, currentUser, dataScope, "position.companies_id", "position.id"),
	).Where("id = ?", id).First(&pos).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position not found or unauthorized"})
		return
	}

	var input struct {
		PosName     string `json:"pos_name" binding:"required"`
		CompaniesID uint   `json:"companies_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Position name is non-calculation metadata and can be updated even if used.
	companyID, ok := repository.ResolveCompanyID(c, currentUser, dataScope, input.CompaniesID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for this company"})
		return
	}
	pos.PosName = input.PosName
	pos.CompaniesID = companyID
	config.DB.Save(&pos)
	c.JSON(http.StatusOK, pos)
}

func DeletePosition(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	var pos models.Position
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "position.companies_id", "position.id"),
	).Where("id = ?", id).First(&pos).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position not found or unauthorized"})
		return
	}

	result := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "position.companies_id", "position.id"),
	).Where("id = ?", id).Delete(&models.Position{})

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position not found or unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Position deleted successfully"})
}

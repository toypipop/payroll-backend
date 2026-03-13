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
		repository.ApplyScopes(currentUser, dataScope, "position.companies_id", "unused"),
	).Preload("Companies").Find(&positions).Error; err != nil {
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
		repository.ApplyScopes(currentUser, dataScope, "position.companies_id", "position.id"),
	).Where("id = ?", id).First(&pos).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position not found or unauthorized"})
		return
	}

	var input struct {
		PosName string `json:"pos_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Position name is non-calculation metadata and can be updated even if used.
	pos.PosName = input.PosName
	pos.CompaniesID = currentUser.CompaniesID
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
		repository.ApplyScopes(currentUser, dataScope, "position.companies_id", "position.id"),
	).Where("id = ?", id).First(&pos).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position not found or unauthorized"})
		return
	}

	isLocked, err := repository.IsRecordLocked("position", pos.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if isLocked {
		c.JSON(http.StatusForbidden, gin.H{"error": repository.ErrRecordInUse.Error()})
		return
	}

	result := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "position.companies_id", "position.id"),
	).Where("id = ?", id).Delete(&models.Position{})

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position not found or unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Position deleted successfully"})
}

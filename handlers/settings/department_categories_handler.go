package settings

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
)

// --- DEPARTMENT CATEGORIES ---
// Global classification shared by every company — no company scoping.

func GetDepartmentCategories(c *gin.Context) {
	var categories []models.DepartmentCategory
	if err := config.DB.Order("name").Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch department categories"})
		return
	}
	c.JSON(http.StatusOK, categories)
}

func CreateDepartmentCategory(c *gin.Context) {
	var input struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	category := models.DepartmentCategory{Name: input.Name}
	if err := config.DB.Create(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create department category"})
		return
	}
	c.JSON(http.StatusCreated, category)
}

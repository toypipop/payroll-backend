package payrollconfig

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
)

// ── Earning & Deduction (global) ──────────────────────────────

func GetEarningCategories(c *gin.Context) {
	var cats []models.EarningCategory
	if err := config.DB.Order("name").Find(&cats).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Earning & Deduction"})
		return
	}
	c.JSON(http.StatusOK, cats)
}

func CreateEarningCategory(c *gin.Context) {
	var cat models.EarningCategory
	if err := c.ShouldBindJSON(&cat); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := config.DB.Create(&cat).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create earning category"})
		return
	}
	c.JSON(http.StatusCreated, cat)
}

func UpdateEarningCategory(c *gin.Context) {
	id := c.Param("id")
	var cat models.EarningCategory
	if err := config.DB.Where("id = ?", id).First(&cat).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Earning category not found"})
		return
	}
	var input struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cat.Name = input.Name
	config.DB.Save(&cat)
	c.JSON(http.StatusOK, cat)
}

func DeleteEarningCategory(c *gin.Context) {
	id := c.Param("id")
	var cat models.EarningCategory
	if err := config.DB.Where("id = ?", id).First(&cat).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Earning category not found"})
		return
	}
	config.DB.Delete(&cat)
	c.JSON(http.StatusOK, gin.H{"message": "Earning category deleted"})
}

// ── Deduction Categories (global) ────────────────────────────

func GetDeductionCategories(c *gin.Context) {
	var cats []models.DeductionCategory
	if err := config.DB.Order("name").Find(&cats).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch deduction categories"})
		return
	}
	c.JSON(http.StatusOK, cats)
}

func CreateDeductionCategory(c *gin.Context) {
	var cat models.DeductionCategory
	if err := c.ShouldBindJSON(&cat); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := config.DB.Create(&cat).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create deduction category"})
		return
	}
	c.JSON(http.StatusCreated, cat)
}

func UpdateDeductionCategory(c *gin.Context) {
	id := c.Param("id")
	var cat models.DeductionCategory
	if err := config.DB.Where("id = ?", id).First(&cat).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deduction category not found"})
		return
	}
	var input struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cat.Name = input.Name
	config.DB.Save(&cat)
	c.JSON(http.StatusOK, cat)
}

func DeleteDeductionCategory(c *gin.Context) {
	id := c.Param("id")
	var cat models.DeductionCategory
	if err := config.DB.Where("id = ?", id).First(&cat).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deduction category not found"})
		return
	}
	config.DB.Delete(&cat)
	c.JSON(http.StatusOK, gin.H{"message": "Deduction category deleted"})
}

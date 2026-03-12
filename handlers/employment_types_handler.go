package handlers

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
)

func GetEmploymentTypes(c *gin.Context) {
	var types []models.EmploymentType
	if err := config.DB.Order("name").Find(&types).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch employment types"})
		return
	}
	c.JSON(http.StatusOK, types)
}

func CreateEmploymentType(c *gin.Context) {
	var et models.EmploymentType
	if err := c.ShouldBindJSON(&et); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := config.DB.Create(&et).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create employment type"})
		return
	}
	c.JSON(http.StatusCreated, et)
}

func UpdateEmploymentType(c *gin.Context) {
	id := c.Param("id")
	var et models.EmploymentType
	if err := config.DB.Where("id = ?", id).First(&et).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employment type not found"})
		return
	}
	var input struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	et.Name = input.Name
	et.Description = input.Description
	config.DB.Save(&et)
	c.JSON(http.StatusOK, et)
}

func DeleteEmploymentType(c *gin.Context) {
	id := c.Param("id")
	var et models.EmploymentType
	if err := config.DB.Where("id = ?", id).First(&et).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employment type not found"})
		return
	}
	config.DB.Delete(&et)
	c.JSON(http.StatusOK, gin.H{"message": "Employment type deleted"})
}

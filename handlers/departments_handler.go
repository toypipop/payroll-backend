package handlers

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

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
		repository.ApplyScopes(currentUser, dataScope, "department.companies_id", "department.id"),
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
		repository.ApplyScopes(currentUser, dataScope, "department.companies_id", "department.id"),
	).Where("id = ?", id).Delete(&models.Department{})

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Department not found or unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Department deleted successfully"})
}

package settings

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

func GetLeaveTypeSettings(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var leaveTypes []models.LeaveTypeSetting
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "leave_type_setting.companies_id", "unused"),
	).Preload("Companies").Order("name").Find(&leaveTypes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch leave types"})
		return
	}
	c.JSON(http.StatusOK, leaveTypes)
}

func CreateLeaveTypeSetting(c *gin.Context) {
	var lt models.LeaveTypeSetting
	if err := c.ShouldBindJSON(&lt); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)
	companyID, ok := repository.ResolveCompanyID(currentUser, dataScope, lt.CompaniesID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for this company"})
		return
	}
	lt.CompaniesID = companyID

	if err := config.DB.Create(&lt).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create leave type"})
		return
	}
	c.JSON(http.StatusCreated, lt)
}

func UpdateLeaveTypeSetting(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var lt models.LeaveTypeSetting
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "leave_type_setting.companies_id", "unused"),
	).Where("id = ?", id).First(&lt).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Leave type not found or unauthorized"})
		return
	}

	var input struct {
		Name           string  `json:"name" binding:"required"`
		IsPaid         bool    `json:"is_paid"`
		MaxDaysPerYear float64 `json:"max_days_per_year"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	lt.Name = input.Name
	lt.IsPaid = input.IsPaid
	lt.MaxDaysPerYear = input.MaxDaysPerYear
	config.DB.Save(&lt)
	c.JSON(http.StatusOK, lt)
}

func DeleteLeaveTypeSetting(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	var lt models.LeaveTypeSetting
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "leave_type_setting.companies_id", "unused"),
	).Where("id = ?", id).First(&lt).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Leave type not found or unauthorized"})
		return
	}
	config.DB.Delete(&lt)
	c.JSON(http.StatusOK, gin.H{"message": "Leave type deleted"})
}

package settings

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

func GetDayOffs(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var dayOffs []models.DayOff
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "day_off.companies_id", "unused"),
	).Preload("Companies").Order("weekday ASC, name ASC").Find(&dayOffs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch day offs"})
		return
	}

	c.JSON(http.StatusOK, dayOffs)
}

// GetDayOffOptions returns a minimal id/weekday/companies_id list. Ungated by
// page permission (mirrors GetWorkLocationOptions) so other pages (e.g. the
// Time Log bulk-add form) can default weekday selection to a company's
// configured day-offs without needing DayOffPage permission.
func GetDayOffOptions(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	type option struct {
		ID          uint `json:"id"`
		Weekday     int  `json:"weekday"`
		CompaniesID uint `json:"companies_id"`
	}

	var dayOffs []models.DayOff
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "day_off.companies_id", "unused"),
	).Find(&dayOffs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch day offs"})
		return
	}

	options := make([]option, 0, len(dayOffs))
	for _, d := range dayOffs {
		options = append(options, option{ID: d.ID, Weekday: d.Weekday, CompaniesID: d.CompaniesID})
	}
	c.JSON(http.StatusOK, options)
}

func GetDayOffCompanies(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var companies []models.Companies
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "companies.id", "unused"),
	).Order("name ASC").Find(&companies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch companies"})
		return
	}

	c.JSON(http.StatusOK, companies)
}

func CreateDayOff(c *gin.Context) {
	var input struct {
		CompaniesID uint   `json:"companies_id"`
		Weekday     int    `json:"weekday" binding:"min=0,max=6"`
		Name        string `json:"name" binding:"required"`
		IsPaid      bool   `json:"is_paid"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	targetCompanyID, ok := repository.ResolveCompanyID(c, currentUser, dataScope, input.CompaniesID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for this company"})
		return
	}

	var company models.Companies
	if err := config.DB.Select("id").First(&company, targetCompanyID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid company"})
		return
	}

	dayOff := models.DayOff{
		CompaniesID: targetCompanyID,
		Weekday:     input.Weekday,
		Name:        input.Name,
		IsPaid:      input.IsPaid,
	}

	if err := config.DB.Create(&dayOff).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create day off"})
		return
	}

	c.JSON(http.StatusCreated, dayOff)
}

func UpdateDayOff(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var dayOff models.DayOff
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "day_off.companies_id", "unused"),
	).Where("id = ?", id).First(&dayOff).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Day off not found or unauthorized"})
		return
	}

	var input struct {
		CompaniesID uint   `json:"companies_id"`
		Weekday     int    `json:"weekday" binding:"min=0,max=6"`
		Name        string `json:"name" binding:"required"`
		IsPaid      bool   `json:"is_paid"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Company is immutable after creation — ignore companies_id from the request.
	dayOff.Weekday = input.Weekday
	dayOff.Name = input.Name
	dayOff.IsPaid = input.IsPaid
	config.DB.Save(&dayOff)
	c.JSON(http.StatusOK, dayOff)
}

func DeleteDayOff(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	var dayOff models.DayOff
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "day_off.companies_id", "unused"),
	).Where("id = ?", id).First(&dayOff).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Day off not found or unauthorized"})
		return
	}

	config.DB.Delete(&dayOff)
	c.JSON(http.StatusOK, gin.H{"message": "Day off deleted"})
}

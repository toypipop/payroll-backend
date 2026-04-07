package attendance

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

func GetAttendanceConfigs(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var companies []models.Companies
	err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "companies.id", "unused"),
	).Preload("AttendanceConfig").Find(&companies).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, companies)
}

func parseTimeString(timeStr string) *time.Time {
	if timeStr == "" {
		return nil
	}
	parsed, err := time.Parse("15:04", timeStr)
	if err != nil {
		return nil
	}
	now := time.Now()
	result := time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour(), parsed.Minute(), 0, 0, now.Location())
	return &result
}

func UpdateAttendanceConfig(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var input struct {
		CompaniesID               uint    `json:"companies_id"`
		LateRatePerMinute         float64 `json:"late_rate_per_minute"`
		AbsentRatePerDay          float64 `json:"absent_rate_per_day"`
		IsLateDeductionEnabled    bool    `json:"is_late_deduction_enabled"`
		IsAbsentDeductionEnabled  bool    `json:"is_absent_deduction_enabled"`
		WorkStartTime             string  `json:"work_start_time"`
		WorkEndTime               string  `json:"work_end_time"`
		OvertimeRatePerMinute     float64 `json:"overtime_rate_per_minute"`
		ExtraRatePerDay           float64 `json:"extra_rate_per_day"`
		IsOvertimeEarningsEnabled bool    `json:"is_overtime_earnings_enabled"`
		IsExtraEarningsEnabled    bool    `json:"is_extra_earnings_enabled"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var targetCompany models.Companies
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "companies.id", "unused"),
	).Where("id = ?", input.CompaniesID).First(&targetCompany).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for this company"})
		return
	}

	// Parse time strings to time.Time
	workStartTime := parseTimeString(input.WorkStartTime)
	workEndTime := parseTimeString(input.WorkEndTime)

	err := config.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "companies_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"late_rate_per_minute":         input.LateRatePerMinute,
			"absent_rate_per_day":          input.AbsentRatePerDay,
			"is_late_deduction_enabled":    input.IsLateDeductionEnabled,
			"is_absent_deduction_enabled":  input.IsAbsentDeductionEnabled,
			"work_start_time":              workStartTime,
			"work_end_time":                workEndTime,
			"overtime_rate_per_minute":     input.OvertimeRatePerMinute,
			"extra_rate_per_day":           input.ExtraRatePerDay,
			"is_overtime_earnings_enabled": input.IsOvertimeEarningsEnabled,
			"is_extra_earnings_enabled":    input.IsExtraEarningsEnabled,
			"updated_at":                   time.Now(),
		}),
	}).Create(&models.AttendanceConfig{
		CompaniesID:               input.CompaniesID,
		LateRatePerMinute:         input.LateRatePerMinute,
		AbsentRatePerDay:          input.AbsentRatePerDay,
		IsLateDeductionEnabled:    input.IsLateDeductionEnabled,
		IsAbsentDeductionEnabled:  input.IsAbsentDeductionEnabled,
		WorkStartTime:             workStartTime,
		WorkEndTime:               workEndTime,
		OvertimeRatePerMinute:     input.OvertimeRatePerMinute,
		ExtraRatePerDay:           input.ExtraRatePerDay,
		IsOvertimeEarningsEnabled: input.IsOvertimeEarningsEnabled,
		IsExtraEarningsEnabled:    input.IsExtraEarningsEnabled,
	}).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Configuration saved", "data": input})
}

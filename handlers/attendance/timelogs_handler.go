package attendance

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"
	"time"

	"github.com/gin-gonic/gin"
)

func GetTimeLogs(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	var logs []models.TimeLog
	err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "time_log.companies_id", "time_log.employees_id"),
	).Preload("Employees").Preload("Employees.Companies").Find(&logs).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, logs)
}

func CreateTimeLog(c *gin.Context) {
	var input models.TimeLog
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.EmployeesID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Employee is required"})
		return
	}
	if input.WorkLocationID == nil || *input.WorkLocationID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Work location is required"})
		return
	}

	// 1. Validate CheckIn and CheckOut sequence (English Comment)
	if input.CheckIn != nil && input.CheckOut != nil {
		if input.CheckOut.Before(*input.CheckIn) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Check-out time cannot be earlier than check-in time"})
			return
		}
	}

	// 2. Get Context and Security (English Comment)
	userID, dataScope := repository.GetContextAuth(c)
	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context invalid"})
		return
	}

	targetEmployee, ok := repository.ResolveEmployeeForCompany(c, currentUser, dataScope, input.EmployeesID, input.CompaniesID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Employee does not belong to selected company or access denied"})
		return
	}

	// 3. Force company ID from the selected employee for security and schedule lookup.
	input.CompaniesID = targetEmployee.CompaniesID
	input.Status = deriveTimeLogStatus(input.CompaniesID, input.CheckIn, input.CheckOut)

	// 4. Save to Database (English Comment)
	if err := config.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, input)
}

func UpdateTimeLog(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	var log models.TimeLog
	if err := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "time_log.companies_id", "time_log.employees_id"),
	).First(&log, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or access denied"})
		return
	}

	var input models.TimeLog
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.EmployeesID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Employee is required"})
		return
	}
	if input.WorkLocationID == nil || *input.WorkLocationID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Work location is required"})
		return
	}

	targetEmployee, ok := repository.ResolveEmployeeForCompany(c, currentUser, dataScope, input.EmployeesID, input.CompaniesID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Employee does not belong to selected company or access denied"})
		return
	}

	log.EmployeesID = targetEmployee.ID
	log.CompaniesID = targetEmployee.CompaniesID
	log.CheckIn = input.CheckIn
	log.CheckOut = input.CheckOut
	log.WorkLocationID = input.WorkLocationID
	log.LocationIn = input.LocationIn
	log.Remark = input.Remark

	if log.CheckIn != nil && log.CheckOut != nil && log.CheckOut.Before(*log.CheckIn) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Check-out time cannot be earlier than check-in time"})
		return
	}

	log.Status = deriveTimeLogStatus(log.CompaniesID, log.CheckIn, log.CheckOut)
	config.DB.Save(&log)
	c.JSON(http.StatusOK, log)
}

func DeleteTimeLog(c *gin.Context) {
	id := c.Param("id")
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID)

	result := config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "time_log.companies_id", "time_log.employees_id"),
	).Where("id = ?", id).Delete(&models.TimeLog{})

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

func deriveTimeLogStatus(companiesID uint, checkIn, checkOut *time.Time) models.AttendanceStatus {
	if checkIn == nil || checkOut == nil {
		return models.StatusAbsent
	}

	day := checkIn.Format("2006-01-02")

	var holidayCount int64
	config.DB.Model(&models.Holiday{}).Where("companies_id = ? AND date = ?", companiesID, day).Count(&holidayCount)
	if holidayCount > 0 {
		return models.StatusHoliday
	}

	var dayOffCount int64
	config.DB.Model(&models.DayOff{}).Where("companies_id = ? AND weekday = ?", companiesID, int(checkIn.Weekday())).Count(&dayOffCount)
	if dayOffCount > 0 {
		return models.StatusHoliday
	}

	workStartHour := 9
	workStartMin := 0
	var cfg models.AttendanceConfig
	if err := config.DB.Where("companies_id = ?", companiesID).First(&cfg).Error; err == nil && cfg.WorkStartTime != nil {
		workStartHour = cfg.WorkStartTime.Hour()
		workStartMin = cfg.WorkStartTime.Minute()
	}

	scheduledStart := time.Date(checkIn.Year(), checkIn.Month(), checkIn.Day(), workStartHour, workStartMin, 0, 0, checkIn.Location())
	if checkIn.After(scheduledStart) {
		return models.StatusLate
	}

	return models.StatusOnTime
}

package attendance

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/timeutil"
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

// BulkCreateTimeLog creates one TimeLog per matching weekday within a date
// range for a single employee, using the same check-in/check-out time of
// day for every generated log. Dates that already have a log for that
// employee are skipped rather than duplicated.
func BulkCreateTimeLog(c *gin.Context) {
	var input struct {
		EmployeesID    uint   `json:"employees_id" binding:"required"`
		CompaniesID    uint   `json:"companies_id" binding:"required"`
		WorkLocationID uint   `json:"work_location_id" binding:"required"`
		StartDate      string `json:"start_date" binding:"required"`
		EndDate        string `json:"end_date" binding:"required"`
		Weekdays       []int  `json:"weekdays" binding:"required"`
		CheckInTime    string `json:"check_in_time" binding:"required"`
		CheckOutTime   string `json:"check_out_time" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startDate, err := time.Parse("2006-01-02", input.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format. Expected YYYY-MM-DD"})
		return
	}
	endDate, err := time.Parse("2006-01-02", input.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format. Expected YYYY-MM-DD"})
		return
	}
	if endDate.Before(startDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end_date cannot be before start_date"})
		return
	}
	if endDate.Sub(startDate) > 366*24*time.Hour {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Date range cannot exceed 366 days"})
		return
	}

	checkInHour, checkInMin, err := parseHHMM(input.CheckInTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid check_in_time format. Expected HH:MM"})
		return
	}
	checkOutHour, checkOutMin, err := parseHHMM(input.CheckOutTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid check_out_time format. Expected HH:MM"})
		return
	}

	weekdaySet := make(map[int]bool, len(input.Weekdays))
	for _, w := range input.Weekdays {
		weekdaySet[w] = true
	}
	if len(weekdaySet) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one weekday must be selected"})
		return
	}

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

	createdCount := 0
	var skippedDates []string
	workLocationID := input.WorkLocationID

	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		if !weekdaySet[int(d.Weekday())] {
			continue
		}
		dateStr := d.Format("2006-01-02")

		var existingCount int64
		config.DB.Model(&models.TimeLog{}).
			Where("employees_id = ? AND check_in >= ? AND check_in < ?", targetEmployee.ID, d, d.AddDate(0, 0, 1)).
			Count(&existingCount)
		if existingCount > 0 {
			skippedDates = append(skippedDates, dateStr)
			continue
		}

		checkIn := time.Date(d.Year(), d.Month(), d.Day(), checkInHour, checkInMin, 0, 0, timeutil.BangkokLocation)
		checkOut := time.Date(d.Year(), d.Month(), d.Day(), checkOutHour, checkOutMin, 0, 0, timeutil.BangkokLocation)
		if !checkOut.After(checkIn) {
			checkOut = checkOut.AddDate(0, 0, 1)
		}

		log := models.TimeLog{
			CompaniesID:    targetEmployee.CompaniesID,
			EmployeesID:    targetEmployee.ID,
			CheckIn:        &checkIn,
			CheckOut:       &checkOut,
			WorkLocationID: &workLocationID,
		}
		log.Status = deriveTimeLogStatus(log.CompaniesID, log.CheckIn, log.CheckOut)

		if err := config.DB.Create(&log).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		createdCount++
	}

	c.JSON(http.StatusOK, gin.H{
		"created_count": createdCount,
		"skipped_count": len(skippedDates),
		"skipped_dates": skippedDates,
	})
}

func parseHHMM(value string) (int, int, error) {
	t, err := time.Parse("15:04", value)
	if err != nil {
		return 0, 0, err
	}
	return t.Hour(), t.Minute(), nil
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

	// Normalize to the business timezone before extracting date/weekday: a
	// *time.Time read back from Postgres carries whatever session timezone
	// the driver attached, so reading these directly can land on the wrong
	// calendar day/weekday.
	bkkCheckIn := timeutil.ToBangkok(*checkIn)
	day := bkkCheckIn.Format("2006-01-02")

	var holidayCount int64
	config.DB.Model(&models.Holiday{}).Where("companies_id = ? AND date = ?", companiesID, day).Count(&holidayCount)
	if holidayCount > 0 {
		return models.StatusHoliday
	}

	var dayOffCount int64
	config.DB.Model(&models.DayOff{}).Where("companies_id = ? AND weekday = ?", companiesID, int(bkkCheckIn.Weekday())).Count(&dayOffCount)
	if dayOffCount > 0 {
		return models.StatusHoliday
	}

	workStartHour := 9
	workStartMin := 0
	var cfg models.AttendanceConfig
	if err := config.DB.Where("companies_id = ?", companiesID).First(&cfg).Error; err == nil && cfg.WorkStartTime != nil {
		bkkStart := timeutil.ToBangkok(*cfg.WorkStartTime)
		workStartHour = bkkStart.Hour()
		workStartMin = bkkStart.Minute()
	}

	scheduledStart := time.Date(bkkCheckIn.Year(), bkkCheckIn.Month(), bkkCheckIn.Day(), workStartHour, workStartMin, 0, 0, timeutil.BangkokLocation)
	if checkIn.After(scheduledStart) {
		return models.StatusLate
	}

	return models.StatusOnTime
}

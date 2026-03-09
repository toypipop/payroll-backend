package handlers

import (
	"math"
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"
	"time"

	"github.com/gin-gonic/gin"
)

type AttendanceSummaryRow struct {
	EmployeesID   uint    `json:"employees_id"`
	EmployeesCode string  `json:"employees_code"`
	FullName      string  `json:"full_name"`
	TotalLateMin  float64 `json:"total_late_minutes"`
	TotalAbsent   int     `json:"total_absent_days"`
	LateAmount    float64 `json:"late_amount"`
	AbsentAmount  float64 `json:"absent_amount"`
	TotalAmount   float64 `json:"total_amount"`
}

func GetAttendanceSummary(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	fromStr := c.Query("from")
	toStr := c.Query("to")

	var fromDate, toDate time.Time
	var parseErr error

	if fromStr != "" {
		fromDate, parseErr = time.Parse("2006-01-02", fromStr)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid 'from' date. Use YYYY-MM-DD"})
			return
		}
	} else {
		now := time.Now()
		fromDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	}

	if toStr != "" {
		toDate, parseErr = time.Parse("2006-01-02", toStr)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid 'to' date. Use YYYY-MM-DD"})
			return
		}
		toDate = toDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	} else {
		toDate = time.Now()
	}

	// Load attendance config for work-start time and rates
	var attConfig models.AttendanceConfig
	config.DB.Where("companies_id = ?", currentUser.CompaniesID).First(&attConfig)

	workStartHour := attConfig.WorkStartHour
	workStartMin := attConfig.WorkStartMin
	if workStartHour == 0 && workStartMin == 0 {
		workStartHour = 9 // fallback default 09:00
	}
	lateRate := attConfig.LateRatePerMinute
	absentRate := attConfig.AbsentRatePerDay

	// Fetch employees within scope
	var employees []models.Employees
	config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "employees.companies_id", "employees.id"),
	).Find(&employees)

	if len(employees) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"from":    fromDate.Format("2006-01-02"),
			"to":      time.Date(toDate.Year(), toDate.Month(), toDate.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02"),
			"summary": []AttendanceSummaryRow{},
		})
		return
	}

	empIDs := make([]uint, len(employees))
	for i, e := range employees {
		empIDs[i] = e.ID
	}

	// Fetch timelogs: covers both check_in-based (late/on_time) and absent without check_in
	var timeLogs []models.TimeLog
	config.DB.Where(
		"employees_id IN ? AND ((check_in >= ? AND check_in <= ?) OR (status = ? AND check_in IS NULL AND created_at >= ? AND created_at <= ?))",
		empIDs, fromDate, toDate, models.StatusAbsent, fromDate, toDate,
	).Find(&timeLogs)

	type empSummary struct {
		lateMins   float64
		absentDays int
	}
	summaryMap := make(map[uint]*empSummary, len(employees))
	for _, e := range employees {
		summaryMap[e.ID] = &empSummary{}
	}

	for _, tl := range timeLogs {
		s, ok := summaryMap[tl.EmployeesID]
		if !ok {
			continue
		}
		switch tl.Status {
		case models.StatusAbsent:
			s.absentDays++
		case models.StatusLate:
			if tl.CheckIn != nil {
				scheduled := time.Date(
					tl.CheckIn.Year(), tl.CheckIn.Month(), tl.CheckIn.Day(),
					workStartHour, workStartMin, 0, 0, tl.CheckIn.Location(),
				)
				if tl.CheckIn.After(scheduled) {
					s.lateMins += tl.CheckIn.Sub(scheduled).Minutes()
				}
			}
		}
	}

	result := make([]AttendanceSummaryRow, 0, len(employees))
	for _, emp := range employees {
		s := summaryMap[emp.ID]
		lateAmt := math.Round(s.lateMins*lateRate*100) / 100
		absentAmt := math.Round(float64(s.absentDays)*absentRate*100) / 100
		result = append(result, AttendanceSummaryRow{
			EmployeesID:   emp.ID,
			EmployeesCode: emp.EmployeesCode,
			FullName:      emp.FirstName + " " + emp.LastName,
			TotalLateMin:  math.Round(s.lateMins*100) / 100,
			TotalAbsent:   s.absentDays,
			LateAmount:    lateAmt,
			AbsentAmount:  absentAmt,
			TotalAmount:   math.Round((lateAmt+absentAmt)*100) / 100,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"from":    fromDate.Format("2006-01-02"),
		"to":      time.Date(toDate.Year(), toDate.Month(), toDate.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02"),
		"summary": result,
	})
}

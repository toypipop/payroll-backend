package attendance

import (
	"fmt"
	"math"
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/timeutil"
	"time"

	"github.com/gin-gonic/gin"
)

type AttendanceSummaryRow struct {
	EmployeesID   uint    `json:"employees_id"`
	EmployeesCode string  `json:"employees_code"`
	FullName      string  `json:"full_name"`
	CompanyName   string  `json:"company_name"`
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

	if toDate.Before(fromDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "'to' date must be on or after 'from' date"})
		return
	}

	// Fetch employees within scope
	var employees []models.Employees
	config.DB.Scopes(
		repository.ApplyContextScopes(c, currentUser, dataScope, "employees.companies_id", "employees.id"),
	).Preload("Companies").Find(&employees)

	if len(employees) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"from":    fromDate.Format("2006-01-02"),
			"to":      time.Date(toDate.Year(), toDate.Month(), toDate.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02"),
			"summary": []AttendanceSummaryRow{},
		})
		return
	}

	empIDs := make([]uint, len(employees))
	companyIDSet := make(map[uint]bool)
	for i, e := range employees {
		empIDs[i] = e.ID
		companyIDSet[e.CompaniesID] = true
	}

	companyIDs := make([]uint, 0, len(companyIDSet))
	for id := range companyIDSet {
		companyIDs = append(companyIDs, id)
	}

	var attConfigs []models.AttendanceConfig
	config.DB.Where("companies_id IN ?", companyIDs).Find(&attConfigs)
	configByCompany := make(map[uint]models.AttendanceConfig, len(attConfigs))
	for _, cfg := range attConfigs {
		configByCompany[cfg.CompaniesID] = cfg
	}

	var holidays []models.Holiday
	config.DB.Where("companies_id IN ? AND date >= ? AND date <= ?", companyIDs, fromDate, toDate).Find(&holidays)
	holidayMap := make(map[string]bool, len(holidays))
	for _, h := range holidays {
		holidayMap[dayKey(h.CompaniesID, h.Date)] = true
	}

	var dayOffs []models.DayOff
	config.DB.Where("companies_id IN ?", companyIDs).Find(&dayOffs)
	dayOffWeekdayMap := make(map[uint]map[int]bool)
	for _, d := range dayOffs {
		if _, ok := dayOffWeekdayMap[d.CompaniesID]; !ok {
			dayOffWeekdayMap[d.CompaniesID] = map[int]bool{}
		}
		dayOffWeekdayMap[d.CompaniesID][d.Weekday] = true
	}

	var approvedLeaves []models.LeaveRequest
	config.DB.Where("employees_id IN ? AND status = ? AND start_date <= ? AND end_date >= ?", empIDs, models.LeaveStatusApproved, toDate, fromDate).Find(&approvedLeaves)
	leaveMap := make(map[string]bool)
	for _, lv := range approvedLeaves {
		for day := dayStart(lv.StartDate); !day.After(dayStart(lv.EndDate)); day = day.AddDate(0, 0, 1) {
			leaveMap[employeeDayKey(lv.EmployeesID, day)] = true
		}
	}

	var timeLogs []models.TimeLog
	config.DB.Where(
		"employees_id IN ? AND ((check_in >= ? AND check_in <= ?) OR (check_in IS NULL AND created_at >= ? AND created_at <= ?))",
		empIDs, fromDate, toDate, fromDate, toDate,
	).Find(&timeLogs)

	type empSummary struct {
		lateMins   float64
		absentDays int
	}
	type dayRecord struct {
		hasCheckPair    bool
		earliestCheckIn *time.Time
		hasLateStatus   bool
	}
	summaryMap := make(map[uint]*empSummary, len(employees))
	logsByEmployeeDay := make(map[string]*dayRecord)
	for _, e := range employees {
		summaryMap[e.ID] = &empSummary{}
	}

	for _, tl := range timeLogs {
		_, ok := summaryMap[tl.EmployeesID]
		if !ok {
			continue
		}

		logDate := tl.CreatedAt
		if tl.CheckIn != nil {
			logDate = *tl.CheckIn
		}
		key := employeeDayKey(tl.EmployeesID, logDate)
		rec := logsByEmployeeDay[key]
		if rec == nil {
			rec = &dayRecord{}
			logsByEmployeeDay[key] = rec
		}

		if tl.CheckIn != nil && tl.CheckOut != nil {
			rec.hasCheckPair = true
			if rec.earliestCheckIn == nil || tl.CheckIn.Before(*rec.earliestCheckIn) {
				t := *tl.CheckIn
				rec.earliestCheckIn = &t
			}
		}

		// Check if status is marked as late
		if tl.Status == models.StatusLate {
			rec.hasLateStatus = true
		}
	}

	for _, emp := range employees {
		s := summaryMap[emp.ID]
		cfg := configByCompany[emp.CompaniesID]

		workStartHour := 9
		workStartMin := 0
		if cfg.WorkStartTime != nil {
			bkkTime := timeutil.ToBangkok(*cfg.WorkStartTime)
			workStartHour = bkkTime.Hour()
			workStartMin = bkkTime.Minute()
		}

		for day := dayStart(fromDate); !day.After(dayStart(toDate)); day = day.AddDate(0, 0, 1) {
			isDayOff := dayOffWeekdayMap[emp.CompaniesID] != nil && dayOffWeekdayMap[emp.CompaniesID][int(day.Weekday())]
			if holidayMap[dayKey(emp.CompaniesID, day)] || isDayOff {
				continue
			}
			if leaveMap[employeeDayKey(emp.ID, day)] {
				continue
			}

			rec := logsByEmployeeDay[employeeDayKey(emp.ID, day)]
			if rec == nil || !rec.hasCheckPair {
				s.absentDays++
				continue
			}

			scheduled := time.Date(day.Year(), day.Month(), day.Day(), workStartHour, workStartMin, 0, 0, timeutil.BangkokLocation)
			if rec.earliestCheckIn != nil && rec.earliestCheckIn.After(scheduled) {
				s.lateMins += rec.earliestCheckIn.Sub(scheduled).Minutes()
			} else if rec.hasLateStatus {
				// If status is marked as late but time calculation shows on-time, use 1 minute as default
				s.lateMins += 1.0
			}
		}
	}

	result := make([]AttendanceSummaryRow, 0, len(employees))
	for _, emp := range employees {
		s := summaryMap[emp.ID]
		cfg := configByCompany[emp.CompaniesID]

		lateMins := s.lateMins
		if !cfg.IsLateDeductionEnabled {
			lateMins = 0
		}

		absentDays := s.absentDays
		if !cfg.IsAbsentDeductionEnabled {
			absentDays = 0
		}

		lateAmt := 0.0
		if cfg.IsLateDeductionEnabled {
			lateAmt = math.Round(lateMins*cfg.LateRatePerMinute*100) / 100
			if lateAmt > cfg.AbsentRatePerDay {
				lateAmt = cfg.AbsentRatePerDay
			}
		}

		absentAmt := 0.0
		if cfg.IsAbsentDeductionEnabled {
			absentAmt = math.Round(float64(absentDays)*cfg.AbsentRatePerDay*100) / 100
		}

		result = append(result, AttendanceSummaryRow{
			EmployeesID:   emp.ID,
			EmployeesCode: emp.EmployeesCode,
			FullName:      emp.FirstName + " " + emp.LastName,
			CompanyName:   emp.Companies.Name,
			TotalLateMin:  math.Round(lateMins*100) / 100,
			TotalAbsent:   absentDays,
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

// dayStart, dayKey, and employeeDayKey normalize to the business timezone
// (Asia/Bangkok) before extracting Y/M/D: a time.Time read back from
// Postgres carries whatever session timezone the driver attached, so
// reading Year()/Month()/Day() directly can land on the wrong calendar day.
func dayStart(t time.Time) time.Time {
	return timeutil.DayStart(t)
}

func dayKey(companiesID uint, day time.Time) string {
	return fmt.Sprintf("%d|%s", companiesID, timeutil.DateKey(day))
}

func employeeDayKey(employeeID uint, day time.Time) string {
	return fmt.Sprintf("%d|%s", employeeID, timeutil.DateKey(day))
}

package attendance

import (
	"math"
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"
	"time"

	"github.com/gin-gonic/gin"
)

type EarningSummaryRow struct {
	EmployeesID          uint    `json:"employees_id"`
	EmployeesCode        string  `json:"employees_code"`
	FullName             string  `json:"full_name"`
	CompanyName          string  `json:"company_name"`
	TotalOvertimeMinutes float64 `json:"total_overtime_minutes"`
	TotalExtraDays       int     `json:"total_extra_days"`
	OvertimeAmount       float64 `json:"overtime_amount"`
	ExtraAmount          float64 `json:"extra_amount"`
	TotalAmount          float64 `json:"total_amount"`
}

func GetEarningSummary(c *gin.Context) {
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

	var employees []models.Employees
	config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "employees.companies_id", "employees.id"),
	).Preload("Companies").Find(&employees)

	if len(employees) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"from":    fromDate.Format("2006-01-02"),
			"to":      time.Date(toDate.Year(), toDate.Month(), toDate.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02"),
			"summary": []EarningSummaryRow{},
		})
		return
	}

	empIDs := make([]uint, len(employees))
	empByID := make(map[uint]models.Employees, len(employees))
	companyIDSet := make(map[uint]bool)
	for i, e := range employees {
		empIDs[i] = e.ID
		empByID[e.ID] = e
		companyIDSet[e.CompaniesID] = true
	}

	companyIDs := make([]uint, 0, len(companyIDSet))
	for id := range companyIDSet {
		companyIDs = append(companyIDs, id)
	}

	var configs []models.AttendanceConfig
	config.DB.Where("companies_id IN ?", companyIDs).Find(&configs)
	configByCompany := make(map[uint]models.AttendanceConfig, len(configs))
	for _, cfg := range configs {
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

	var timeLogs []models.TimeLog
	config.DB.Where(
		"employees_id IN ? AND ((check_in >= ? AND check_in <= ?) OR (check_in IS NULL AND created_at >= ? AND created_at <= ?))",
		empIDs, fromDate, toDate, fromDate, toDate,
	).Find(&timeLogs)

	type empSummary struct {
		overtimeMins float64
		extraDates   map[string]bool
	}
	type dayRecord struct {
		hasCheckPair    bool
		earliestCheckIn *time.Time
		latestCheckOut  *time.Time
	}
	summaryMap := make(map[uint]*empSummary, len(employees))
	logsByEmployeeDay := make(map[string]*dayRecord)
	for _, e := range employees {
		summaryMap[e.ID] = &empSummary{extraDates: map[string]bool{}}
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
			if rec.latestCheckOut == nil || tl.CheckOut.After(*rec.latestCheckOut) {
				t := *tl.CheckOut
				rec.latestCheckOut = &t
			}
		}
	}

	for _, emp := range employees {
		s := summaryMap[emp.ID]
		cfg := configByCompany[emp.CompaniesID]

		startHour, startMin := 9, 0
		endHour, endMin := 18, 0
		if cfg.WorkStartTime != nil {
			utcTime := cfg.WorkStartTime.UTC()
			startHour, startMin = utcTime.Hour(), utcTime.Minute()
		}
		if cfg.WorkEndTime != nil {
			utcTime := cfg.WorkEndTime.UTC()
			endHour, endMin = utcTime.Hour(), utcTime.Minute()
		}

		for day := dayStart(fromDate); !day.After(dayStart(toDate)); day = day.AddDate(0, 0, 1) {
			rec := logsByEmployeeDay[employeeDayKey(emp.ID, day)]
			if rec == nil || !rec.hasCheckPair || rec.latestCheckOut == nil {
				continue
			}

			isDayOff := dayOffWeekdayMap[emp.CompaniesID] != nil && dayOffWeekdayMap[emp.CompaniesID][int(day.Weekday())]
			isExtraDay := holidayMap[dayKey(emp.CompaniesID, day)] || isDayOff
			if isExtraDay {
				s.extraDates[day.Format("2006-01-02")] = true
				continue
			}

			scheduledEnd := time.Date(day.Year(), day.Month(), day.Day(), endHour, endMin, 0, 0, time.UTC)
			if endHour < startHour || (endHour == startHour && endMin <= startMin) {
				scheduledEnd = scheduledEnd.AddDate(0, 0, 1)
			}

			if rec.latestCheckOut.After(scheduledEnd) {
				s.overtimeMins += rec.latestCheckOut.Sub(scheduledEnd).Minutes()
			}
		}
	}

	result := make([]EarningSummaryRow, 0, len(employees))
	for _, emp := range employees {
		s := summaryMap[emp.ID]
		cfg := configByCompany[emp.CompaniesID]

		overtimeMins := s.overtimeMins
		if !cfg.IsOvertimeEarningsEnabled {
			overtimeMins = 0
		}

		totalExtraDays := len(s.extraDates)
		if !cfg.IsExtraEarningsEnabled {
			totalExtraDays = 0
		}

		overtimeAmount := 0.0
		if cfg.IsOvertimeEarningsEnabled {
			overtimeAmount = math.Round(overtimeMins*cfg.OvertimeRatePerMinute*100) / 100
		}

		extraAmount := 0.0
		if cfg.IsExtraEarningsEnabled {
			extraAmount = math.Round(float64(totalExtraDays)*cfg.ExtraRatePerDay*100) / 100
		}

		result = append(result, EarningSummaryRow{
			EmployeesID:          emp.ID,
			EmployeesCode:        emp.EmployeesCode,
			FullName:             emp.FirstName + " " + emp.LastName,
			CompanyName:          emp.Companies.Name,
			TotalOvertimeMinutes: math.Round(overtimeMins*100) / 100,
			TotalExtraDays:       totalExtraDays,
			OvertimeAmount:       overtimeAmount,
			ExtraAmount:          extraAmount,
			TotalAmount:          math.Round((overtimeAmount+extraAmount)*100) / 100,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"from":    fromDate.Format("2006-01-02"),
		"to":      time.Date(toDate.Year(), toDate.Month(), toDate.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02"),
		"summary": result,
	})
}

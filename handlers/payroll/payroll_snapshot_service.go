package payroll

import (
	"math"
	"payroll-backend/config"
	"payroll-backend/models"
	"strings"
	"time"

	"gorm.io/gorm"
)

// calculateTaxByMethod computes income tax based on the configured tax calculation method
func calculateTaxByMethod(taxMethod string, taxableBase float64, taxDeductible float64) float64 {
	method := strings.ToLower(strings.TrimSpace(taxMethod))

	switch {
	case method == "none" || method == "no tax":
		return 0.0
	case method == "fixed" || method == "fixed rate":
		return math.Round(math.Max(0, taxableBase-taxDeductible)*0.05*100) / 100
	default:
		// Default to Progressive for "Progressive", "Progressive Rate", or any unknown method
		annualTaxableBase := (taxableBase - taxDeductible) * 12
		workExpenseDeduction := math.Min(annualTaxableBase*0.5, 100000)
		personalAllowance := 60000.0
		annualNetIncome := annualTaxableBase - workExpenseDeduction - personalAllowance
		return math.Round(calculateThaiProgressiveTax(annualNetIncome)/12*100) / 100
	}
}

// runPayrollSnapshot recalculates and persists payroll transactions/details for one cycle.
// CRITICAL: Fetches ONLY the LATEST ACTIVE config (non-deleted, newest created)
// This ensures calculations always use current parameters, never stale/old versions
func runPayrollSnapshot(companiesID uint, cycle models.PayrollCycle) (int, int, error) {
	// LATEST ACTIVE VERSION: Fetch the newest NON-DELETED config
	// Ensures stale/old configs are never used in calculation
	var payrollConfig models.PayrollConfig
	if err := config.DB.
		Where("companies_id = ? AND deleted_at IS NULL", companiesID).
		Order("created_at DESC").
		First(&payrollConfig).Error; err != nil {
		return 0, 0, err
	}

	var employees []models.Employees
	if err := config.DB.
		Joins("JOIN roles ON roles.id = employees.roles_id").
		Where("employees.companies_id = ? AND employees.is_active = ? AND roles.is_super_admin = ?", companiesID, true, false).
		Find(&employees).Error; err != nil {
		return 0, 0, err
	}

	var attendanceConfig models.AttendanceConfig
	if err := config.DB.Where("companies_id = ?", companiesID).Order("id DESC").First(&attendanceConfig).Error; err != nil {
		// No config found — default to enabled so payroll still runs
		attendanceConfig.IsLateDeductionEnabled = true
		attendanceConfig.IsAbsentDeductionEnabled = true
	}

	// Pre-load holidays and day-offs for extra day detection
	var holidays []models.Holiday
	config.DB.Where("companies_id = ? AND date >= ? AND date <= ?", companiesID, cycle.StartDate, cycle.EndDate).Find(&holidays)
	holidaySet := make(map[string]bool, len(holidays))
	for _, h := range holidays {
		holidaySet[h.Date.Format("2006-01-02")] = true
	}

	var dayOffs []models.DayOff
	config.DB.Where("companies_id = ?", companiesID).Find(&dayOffs)
	dayOffWeekdays := make(map[int]bool, len(dayOffs))
	for _, d := range dayOffs {
		dayOffWeekdays[d.Weekday] = true
	}

	processedCount := 0
	skippedCount := 0
	var lastErr error

	for _, emp := range employees {
		var salaryRecord models.SalariesStructure
		if err := config.DB.
			Where("employees_id = ? AND effective_date <= ?", emp.ID, cycle.EndDate).
			Order("effective_date DESC, created_at DESC").
			First(&salaryRecord).Error; err != nil {
			skippedCount++
			continue
		}
		baseSalary := salaryRecord.BaseSalaries

		taxableBase := baseSalary
		ssoBase := baseSalary

		var empEarnings []models.EmployeeEarning
		config.DB.Preload("EarningType").
			Where("employees_id = ?", emp.ID).
			Find(&empEarnings)

		additionalEarning := 0.0
		type earningItem struct {
			name          string
			amount        float64
			earningTypeID *uint
		}
		var earningItems []earningItem
		for _, ee := range empEarnings {
			additionalEarning += ee.Amount
			if ee.EarningType.IsTaxable {
				taxableBase += ee.Amount
			}
			if ee.EarningType.IsSso {
				ssoBase += ee.Amount
			}
			etID := ee.EarningTypeID
			earningItems = append(earningItems, earningItem{name: ee.EarningType.Name, amount: ee.Amount, earningTypeID: &etID})
		}

		var cycleEarnings []models.PayrollCycleEarning
		config.DB.Preload("EarningType").
			Where("employees_id = ? AND cycle_id = ?", emp.ID, cycle.ID).
			Find(&cycleEarnings)

		for _, ce := range cycleEarnings {
			additionalEarning += ce.Amount
			if ce.EarningType.IsTaxable {
				taxableBase += ce.Amount
			}
			if ce.EarningType.IsSso {
				ssoBase += ce.Amount
			}
			etID := ce.EarningTypeID
			earningItems = append(earningItems, earningItem{name: ce.EarningType.Name, amount: ce.Amount, earningTypeID: &etID})
		}

		// Use check_in for both bounds: a workday belongs to the date the employee clocked in.
		// EndDate is stored as midnight UTC, so check_out <= EndDate drops the entire last day.
		endOfCycle := cycle.EndDate.Truncate(24 * time.Hour).AddDate(0, 0, 1)
		var timeLogs []models.TimeLog
		config.DB.Where(
			"employees_id = ? AND check_in >= ? AND check_in < ? AND check_in IS NOT NULL AND check_out IS NOT NULL",
			emp.ID, cycle.StartDate, endOfCycle,
		).Find(&timeLogs)

		// Resolve work times from config
		endHour, endMin := 18, 0
		if attendanceConfig.WorkEndTime != nil {
			utcEnd := attendanceConfig.WorkEndTime.UTC()
			endHour, endMin = utcEnd.Hour(), utcEnd.Minute()
		}
		startHour, startMin := 9, 0
		if attendanceConfig.WorkStartTime != nil {
			utcStart := attendanceConfig.WorkStartTime.UTC()
			startHour, startMin = utcStart.Hour(), utcStart.Minute()
		}

		// Build day → check-pair map while computing OT and extra days
		type logDayRecord struct {
			hasCheckPair    bool
			earliestCheckIn time.Time
		}
		logsByDay := make(map[string]*logDayRecord)
		totalOTMins := 0.0
		extraDays := 0
		for _, log := range timeLogs {
			if log.CheckIn == nil || log.CheckOut == nil {
				continue
			}
			day := log.CheckIn.UTC().Truncate(24 * time.Hour)
			dateStr := day.Format("2006-01-02")
			isExtraDay := holidaySet[dateStr] || dayOffWeekdays[int(day.Weekday())]

			if isExtraDay {
				extraDays++
				continue
			}

			// Track earliest check-in per day for late detection
			rec := logsByDay[dateStr]
			if rec == nil {
				rec = &logDayRecord{earliestCheckIn: log.CheckIn.UTC()}
				logsByDay[dateStr] = rec
			}
			rec.hasCheckPair = true
			if log.CheckIn.UTC().Before(rec.earliestCheckIn) {
				rec.earliestCheckIn = log.CheckIn.UTC()
			}

			// OT: minutes worked beyond scheduled end
			scheduledEnd := time.Date(day.Year(), day.Month(), day.Day(), endHour, endMin, 0, 0, time.UTC)
			if endHour < startHour || (endHour == startHour && endMin <= startMin) {
				scheduledEnd = scheduledEnd.AddDate(0, 0, 1)
			}
			if log.CheckOut.UTC().After(scheduledEnd) {
				totalOTMins += log.CheckOut.UTC().Sub(scheduledEnd).Minutes()
			}
		}

		otAmount := 0.0
		if attendanceConfig.IsOvertimeEarningsEnabled {
			otAmount = math.Round(totalOTMins*attendanceConfig.OvertimeRatePerMinute*100) / 100
		}
		taxableBase += otAmount
		ssoBase += otAmount

		extraAmount := 0.0
		if attendanceConfig.IsExtraEarningsEnabled {
			extraAmount = math.Round(float64(extraDays)*attendanceConfig.ExtraRatePerDay*100) / 100
		}
		taxableBase += extraAmount
		ssoBase += extraAmount

		var empDeductions []models.EmployeeDeduction
		config.DB.Preload("DeductionType").Where("employees_id = ?", emp.ID).Find(&empDeductions)

		additionalDeduction := 0.0
		taxDeductible := 0.0
		ssoDeductible := 0.0
		type deductionItem struct {
			name            string
			amount          float64
			deductionTypeID *uint
		}
		var deductionItems []deductionItem
		for _, ed := range empDeductions {
			additionalDeduction += ed.Amount
			if ed.DeductionType.IsTaxDeductible {
				taxDeductible += ed.Amount
			}
			if ed.DeductionType.IsSsoDeductible {
				ssoDeductible += ed.Amount
			}
			dtID := ed.DeductionTypeID
			deductionItems = append(deductionItems, deductionItem{name: ed.DeductionType.Name, amount: ed.Amount, deductionTypeID: &dtID})
		}

		var cycleDeductions []models.PayrollCycleDeduction
		config.DB.Preload("DeductionType").
			Where("employees_id = ? AND cycle_id = ?", emp.ID, cycle.ID).
			Find(&cycleDeductions)

		for _, cd := range cycleDeductions {
			additionalDeduction += cd.Amount
			if cd.DeductionType.IsTaxDeductible {
				taxDeductible += cd.Amount
			}
			if cd.DeductionType.IsSsoDeductible {
				ssoDeductible += cd.Amount
			}
			dtID := cd.DeductionTypeID
			deductionItems = append(deductionItems, deductionItem{name: cd.DeductionType.Name, amount: cd.Amount, deductionTypeID: &dtID})
		}

		// Load ALL approved leaves: build skip-day set + compute unpaid days
		var approvedLeaves []models.LeaveRequest
		config.DB.Where(
			"employees_id = ? AND status = ? AND start_date <= ? AND end_date >= ?",
			emp.ID, models.LeaveStatusApproved, cycle.EndDate, cycle.StartDate,
		).Find(&approvedLeaves)
		leaveSet := make(map[string]bool)
		totalUnpaidDays := 0.0
		for _, lv := range approvedLeaves {
			if lv.LeaveType == models.LeaveTypeUnpaid {
				totalUnpaidDays += lv.TotalDays
			}
			for d := lv.StartDate.Truncate(24 * time.Hour); !d.After(lv.EndDate.Truncate(24 * time.Hour)); d = d.AddDate(0, 0, 1) {
				leaveSet[d.Format("2006-01-02")] = true
			}
		}
		unpaidLeaveDeduction := math.Round(totalUnpaidDays*(baseSalary/30)*100) / 100

		// Day-by-day absent/late detection — same logic as deduction summary
		totalLateMins := 0.0
		totalAbsentDays := 0
		for d := cycle.StartDate.Truncate(24 * time.Hour); !d.After(cycle.EndDate.Truncate(24 * time.Hour)); d = d.AddDate(0, 0, 1) {
			dateStr := d.Format("2006-01-02")
			if holidaySet[dateStr] || dayOffWeekdays[int(d.Weekday())] {
				continue
			}
			if leaveSet[dateStr] {
				continue
			}
			rec := logsByDay[dateStr]
			if rec == nil || !rec.hasCheckPair {
				totalAbsentDays++
				continue
			}
			scheduled := time.Date(d.Year(), d.Month(), d.Day(), startHour, startMin, 0, 0, time.UTC)
			if rec.earliestCheckIn.After(scheduled) {
				totalLateMins += rec.earliestCheckIn.Sub(scheduled).Minutes()
			}
		}

		lateDeduction := 0.0
		absentDeduction := 0.0
		if attendanceConfig.IsLateDeductionEnabled {
			lateDeduction = math.Round(totalLateMins*attendanceConfig.LateRatePerMinute*100) / 100
			if lateDeduction > attendanceConfig.AbsentRatePerDay {
				lateDeduction = attendanceConfig.AbsentRatePerDay
			}
		}
		if attendanceConfig.IsAbsentDeductionEnabled {
			absentDeduction = math.Round(float64(totalAbsentDays)*attendanceConfig.AbsentRatePerDay*100) / 100
		}

		// Thai law: tax and SSO apply on actual earned income — subtract late/absent first
		taxableBase = math.Max(0, taxableBase-lateDeduction-absentDeduction)
		ssoBase = math.Max(0, ssoBase-ssoDeductible-lateDeduction-absentDeduction)
		ssoAmount := 0.0
		if ssoBase >= payrollConfig.SsoMinSalary {
			computed := ssoBase * payrollConfig.SsoRate / 100
			if computed > payrollConfig.SsoMaxAmount {
				computed = payrollConfig.SsoMaxAmount
			}
			ssoAmount = math.Round(computed*100) / 100
		}

		monthlyTax := calculateTaxByMethod(payrollConfig.TaxCalculation, taxableBase, taxDeductible)

		totalEarning := math.Round((baseSalary+additionalEarning+otAmount+extraAmount)*100) / 100
		totalDeduction := math.Round((unpaidLeaveDeduction+additionalDeduction+lateDeduction+absentDeduction)*100) / 100
		netPay := math.Round((totalEarning-totalDeduction-ssoAmount-monthlyTax)*100) / 100

		dbTx := config.DB.Begin()
		if dbTx.Error != nil {
			lastErr = dbTx.Error
			continue
		}

		var transaction models.PayrollTransaction
		isNew := false
		result := dbTx.Where("cycle_id = ? AND employees_id = ?", cycle.ID, emp.ID).First(&transaction)
		if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
			dbTx.Rollback()
			lastErr = result.Error
			continue
		}
		if result.Error == gorm.ErrRecordNotFound {
			isNew = true
		}

		transaction.CycleID = cycle.ID
		transaction.EmployeesID = emp.ID
		transaction.SalariesAtTime = baseSalary
		transaction.TotalEarning = totalEarning
		transaction.TotalDeduction = totalDeduction
		transaction.SsoAmount = ssoAmount
		transaction.TaxAmount = monthlyTax
		transaction.NetPay = netPay

		if isNew {
			if err := dbTx.Create(&transaction).Error; err != nil {
				dbTx.Rollback()
				lastErr = err
				continue
			}
		} else {
			if err := dbTx.Save(&transaction).Error; err != nil {
				dbTx.Rollback()
				lastErr = err
				continue
			}
			if err := dbTx.Where("transaction_id = ?", transaction.ID).Delete(&models.PayrollTransactionDetail{}).Error; err != nil {
				dbTx.Rollback()
				lastErr = err
				continue
			}
		}

		var details []models.PayrollTransactionDetail
		details = append(details, models.PayrollTransactionDetail{TransactionID: transaction.ID, ItemName: "Base Salary", Amount: baseSalary, Type: "earning"})
		for _, ei := range earningItems {
			details = append(details, models.PayrollTransactionDetail{TransactionID: transaction.ID, ItemName: ei.name, Amount: ei.amount, Type: "earning", EarningTypeID: ei.earningTypeID})
		}
		if otAmount > 0 {
			details = append(details, models.PayrollTransactionDetail{TransactionID: transaction.ID, ItemName: "Overtime", Amount: otAmount, Type: "earning"})
		}
		if extraAmount > 0 {
			details = append(details, models.PayrollTransactionDetail{TransactionID: transaction.ID, ItemName: "Extra Day", Amount: extraAmount, Type: "earning"})
		}
		for _, di := range deductionItems {
			details = append(details, models.PayrollTransactionDetail{TransactionID: transaction.ID, ItemName: di.name, Amount: di.amount, Type: "deduction", DeductionTypeID: di.deductionTypeID})
		}
		if unpaidLeaveDeduction > 0 {
			details = append(details, models.PayrollTransactionDetail{TransactionID: transaction.ID, ItemName: "Unpaid Leave", Amount: unpaidLeaveDeduction, Type: "deduction"})
		}
		if lateDeduction > 0 {
			details = append(details, models.PayrollTransactionDetail{TransactionID: transaction.ID, ItemName: "Late Amount", Amount: lateDeduction, Type: "deduction"})
		}
		if absentDeduction > 0 {
			details = append(details, models.PayrollTransactionDetail{TransactionID: transaction.ID, ItemName: "Absent Amount", Amount: absentDeduction, Type: "deduction"})
		}
		if ssoAmount > 0 {
			details = append(details, models.PayrollTransactionDetail{TransactionID: transaction.ID, ItemName: "Social Security (SSO)", Amount: ssoAmount, Type: "deduction"})
		}
		if monthlyTax > 0 {
			details = append(details, models.PayrollTransactionDetail{TransactionID: transaction.ID, ItemName: "Income Tax", Amount: monthlyTax, Type: "deduction"})
		}

		if err := dbTx.Create(&details).Error; err != nil {
			dbTx.Rollback()
			lastErr = err
			continue
		}

		dbTx.Commit()
		processedCount++
	}

	if lastErr != nil {
		return processedCount, skippedCount, lastErr
	}
	return processedCount, skippedCount, nil
}

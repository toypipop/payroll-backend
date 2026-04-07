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
		// Fixed rate: 5% on taxable base (configurable via payroll config if needed)
		return math.Round(taxableBase*0.05*100) / 100
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

	attendanceConfig := models.AttendanceConfig{
		IsLateDeductionEnabled:   true,
		IsAbsentDeductionEnabled: true,
	}
	config.DB.Where("companies_id = ?", companiesID).First(&attendanceConfig)

	processedCount := 0
	skippedCount := 0
	var lastErr error

	for _, emp := range employees {
		var salaryRecord models.SalariesStructure
		if err := config.DB.
			Where("employees_id = ? AND effective_date <= ? AND is_enabled = ?", emp.ID, cycle.EndDate, true).
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
			Joins("JOIN earning_type ON earning_type.id = employee_earning.earning_type_id").
			Where("employee_earning.employees_id = ? AND earning_type.is_permanent = ?", emp.ID, true).
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

		var timeLogs []models.TimeLog
		config.DB.Where(
			"employees_id = ? AND check_in >= ? AND check_out <= ? AND check_in IS NOT NULL AND check_out IS NOT NULL",
			emp.ID, cycle.StartDate, cycle.EndDate,
		).Find(&timeLogs)

		totalOTHours := 0.0
		for _, log := range timeLogs {
			if log.CheckIn == nil || log.CheckOut == nil {
				continue
			}
			workedHours := log.CheckOut.Sub(*log.CheckIn).Hours()
			if ot := workedHours - 8.0; ot > 0 {
				totalOTHours += ot
			}
		}
		hourlyRate := baseSalary / (30 * 8)
		otAmount := math.Round(totalOTHours*hourlyRate*1.5*100) / 100
		taxableBase += otAmount
		ssoBase += otAmount

		var empDeductions []models.EmployeeDeduction
		config.DB.Preload("DeductionType").Where("employees_id = ?", emp.ID).Find(&empDeductions)

		additionalDeduction := 0.0
		taxDeductible := 0.0
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
			dtID := cd.DeductionTypeID
			deductionItems = append(deductionItems, deductionItem{name: cd.DeductionType.Name, amount: cd.Amount, deductionTypeID: &dtID})
		}

		var unpaidLeaves []models.LeaveRequest
		config.DB.Where(
			"employees_id = ? AND leave_type = ? AND status = ? AND start_date <= ? AND end_date >= ?",
			emp.ID, models.LeaveTypeUnpaid, models.LeaveStatusApproved, cycle.EndDate, cycle.StartDate,
		).Find(&unpaidLeaves)

		totalUnpaidDays := 0.0
		for _, leave := range unpaidLeaves {
			totalUnpaidDays += leave.TotalDays
		}
		unpaidLeaveDeduction := math.Round(totalUnpaidDays*(baseSalary/30)*100) / 100

		var attendanceLogs []models.TimeLog
		config.DB.Where(
			"employees_id = ? AND ((check_in >= ? AND check_in <= ?) OR (status = ? AND check_in IS NULL AND created_at >= ? AND created_at <= ?))",
			emp.ID, cycle.StartDate, cycle.EndDate, models.StatusAbsent, cycle.StartDate, cycle.EndDate,
		).Find(&attendanceLogs)

		totalLateMins := 0.0
		totalAbsentDays := 0
		workStartHour := 9
		workStartMin := 0
		if attendanceConfig.WorkStartTime != nil {
			workStartHour = attendanceConfig.WorkStartTime.Hour()
			workStartMin = attendanceConfig.WorkStartTime.Minute()
		}

		for _, log := range attendanceLogs {
			switch log.Status {
			case models.StatusAbsent:
				totalAbsentDays++
			case models.StatusLate:
				if log.CheckIn != nil {
					scheduled := time.Date(log.CheckIn.Year(), log.CheckIn.Month(), log.CheckIn.Day(), workStartHour, workStartMin, 0, 0, log.CheckIn.Location())
					if log.CheckIn.After(scheduled) {
						totalLateMins += log.CheckIn.Sub(scheduled).Minutes()
					}
				}
			}
		}

		lateDeduction := 0.0
		absentDeduction := 0.0
		if attendanceConfig.IsLateDeductionEnabled {
			lateDeduction = math.Round(totalLateMins*attendanceConfig.LateRatePerMinute*100) / 100
		}
		if attendanceConfig.IsAbsentDeductionEnabled {
			absentDeduction = math.Round(float64(totalAbsentDays)*attendanceConfig.AbsentRatePerDay*100) / 100
		}

		ssoAmount := 0.0
		if ssoBase >= payrollConfig.SsoMinSalary {
			computed := ssoBase * payrollConfig.SsoRate / 100
			if computed > payrollConfig.SsoMaxAmount {
				computed = payrollConfig.SsoMaxAmount
			}
			ssoAmount = math.Round(computed*100) / 100
		}

		monthlyTax := calculateTaxByMethod(payrollConfig.TaxCalculation, taxableBase, taxDeductible)

		totalEarning := math.Round((baseSalary+additionalEarning+otAmount)*100) / 100
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

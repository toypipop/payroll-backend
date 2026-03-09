package handlers

import (
	"math"
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// calculateThaiProgressiveTax computes annual income tax using Thailand's progressive brackets
func calculateThaiProgressiveTax(annualNetIncome float64) float64 {
	if annualNetIncome <= 0 {
		return 0
	}

	type bracket struct {
		max  float64
		rate float64
	}

	brackets := []bracket{
		{150000, 0.00},
		{300000, 0.05},
		{500000, 0.10},
		{750000, 0.15},
		{1000000, 0.20},
		{2000000, 0.25},
		{5000000, 0.30},
		{math.MaxFloat64, 0.35},
	}

	tax := 0.0
	prev := 0.0
	for _, b := range brackets {
		if annualNetIncome <= prev {
			break
		}
		taxable := math.Min(annualNetIncome, b.max) - prev
		tax += taxable * b.rate
		prev = b.max
	}
	return tax
}

// CalculatePayroll runs the core payroll engine for all active employees in a given cycle
func CalculatePayroll(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	cycleID := c.Param("cycle_id")

	// 1. Fetch & validate cycle - must belong to user's company and not be closed
	var cycle models.PayrollCycle
	if err := config.DB.Scopes(repository.ScopeCompanies(currentUser.CompaniesID)).
		First(&cycle, cycleID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Payroll cycle not found"})
		return
	}
	if cycle.IsClosed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot recalculate a closed cycle"})
		return
	}

	// 2. Get PayrollConfig for SSO and Tax settings
	var payrollConfig models.PayrollConfig
	if err := config.DB.Where("companies_id = ?", currentUser.CompaniesID).
		First(&payrollConfig).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payroll config not found. Please configure payroll settings first."})
		return
	}

	// 3. Get all active employees (exclude super-admin)
	var employees []models.Employees
	if err := config.DB.
		Joins("JOIN roles ON roles.id = employees.roles_id").
		Where("employees.companies_id = ? AND employees.is_active = ? AND roles.is_super_admin = ?",
			currentUser.CompaniesID, true, false).
		Find(&employees).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch employees"})
		return
	}

	processedCount := 0
	skippedCount := 0
	var lastErr error

	for _, emp := range employees {
		// 4a. Snapshot: get the most recent effective salary on or before cycle end date
		var salaryRecord models.SalariesStructure
		if err := config.DB.
			Where("employees_id = ? AND effective_date <= ? AND is_enabled = ?",
				emp.ID, cycle.EndDate, true).
			Order("effective_date DESC, created_at DESC").
			First(&salaryRecord).Error; err != nil {
			// No valid salary record — skip this employee
			skippedCount++
			continue
		}
		baseSalary := salaryRecord.BaseSalaries

		// 4b. Fixed earnings (is_permanent=true) from EmployeeEarning — applied every cycle
		// taxableBase = income used for progressive tax; ssoBase = income used for SSO
		taxableBase := baseSalary // base salary is always taxable
		ssoBase := baseSalary     // base salary always counts for SSO

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
			earningItems = append(earningItems, earningItem{
				name: ee.EarningType.Name, amount: ee.Amount, earningTypeID: &etID,
			})
		}

		// 4b2. Variable earnings (is_permanent=false) from PayrollCycleEarning — per-cycle only
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
			earningItems = append(earningItems, earningItem{
				name: ce.EarningType.Name, amount: ce.Amount, earningTypeID: &etID,
			})
		}

		// 4c. OT calculation from TimeLogs within cycle period
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
		// OT is taxable and counts toward SSO basis
		taxableBase += otAmount
		ssoBase += otAmount

		// 4d. Additional deductions from EmployeeDeduction assignments
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
			deductionItems = append(deductionItems, deductionItem{
				name: ed.DeductionType.Name, amount: ed.Amount, deductionTypeID: &dtID,
			})
		}

		// 4e. Unpaid leave deduction — only Approved Unpaid leaves overlapping cycle
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

		// 4f. SSO — based on all SSO-eligible earnings, capped at max amount
		ssoAmount := 0.0
		if ssoBase >= payrollConfig.SsoMinSalary {
			computed := ssoBase * payrollConfig.SsoRate / 100
			if computed > payrollConfig.SsoMaxAmount {
				computed = payrollConfig.SsoMaxAmount
			}
			ssoAmount = math.Round(computed*100) / 100
		}

		// 4g. Progressive tax — taxable earnings minus tax-deductible deductions
		annualTaxableBase := (taxableBase - taxDeductible) * 12
		workExpenseDeduction := math.Min(annualTaxableBase*0.5, 100000)
		personalAllowance := 60000.0
		annualNetIncome := annualTaxableBase - workExpenseDeduction - personalAllowance
		monthlyTax := math.Round(calculateThaiProgressiveTax(annualNetIncome)/12*100) / 100

		// 4h. Aggregate totals
		totalEarning := math.Round((baseSalary+additionalEarning+otAmount)*100) / 100
		totalDeduction := math.Round((unpaidLeaveDeduction+additionalDeduction)*100) / 100
		netPay := math.Round((totalEarning-totalDeduction-ssoAmount-monthlyTax)*100) / 100

		// 4h. Upsert PayrollTransaction within a DB transaction
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
			// Soft-delete old details before re-inserting fresh snapshot
			if err := dbTx.Where("transaction_id = ?", transaction.ID).
				Delete(&models.PayrollTransactionDetail{}).Error; err != nil {
				dbTx.Rollback()
				lastErr = err
				continue
			}
		}

		// 4i. Build granular detail lines for payslip
		var details []models.PayrollTransactionDetail
		// Base salary always first
		details = append(details, models.PayrollTransactionDetail{
			TransactionID: transaction.ID, ItemName: "Base Salary", Amount: baseSalary, Type: "earning",
		})
		// Additional earnings (Position Allowance, Bonus, etc.)
		for _, ei := range earningItems {
			details = append(details, models.PayrollTransactionDetail{
				TransactionID: transaction.ID, ItemName: ei.name, Amount: ei.amount,
				Type: "earning", EarningTypeID: ei.earningTypeID,
			})
		}
		// OT
		if otAmount > 0 {
			details = append(details, models.PayrollTransactionDetail{
				TransactionID: transaction.ID, ItemName: "Overtime",
				Amount: otAmount, Type: "earning",
			})
		}
		// Additional deductions (Provident Fund, Loan Repayment, etc.)
		for _, di := range deductionItems {
			details = append(details, models.PayrollTransactionDetail{
				TransactionID: transaction.ID, ItemName: di.name, Amount: di.amount,
				Type: "deduction", DeductionTypeID: di.deductionTypeID,
			})
		}
		// Unpaid leave
		if unpaidLeaveDeduction > 0 {
			details = append(details, models.PayrollTransactionDetail{
				TransactionID: transaction.ID, ItemName: "Unpaid Leave",
				Amount: unpaidLeaveDeduction, Type: "deduction",
			})
		}
		// SSO
		if ssoAmount > 0 {
			details = append(details, models.PayrollTransactionDetail{
				TransactionID: transaction.ID, ItemName: "Social Security (SSO)",
				Amount: ssoAmount, Type: "deduction",
			})
		}
		// Income tax
		if monthlyTax > 0 {
			details = append(details, models.PayrollTransactionDetail{
				TransactionID: transaction.ID, ItemName: "Income Tax",
				Amount: monthlyTax, Type: "deduction",
			})
		}

		if err := dbTx.Create(&details).Error; err != nil {
			dbTx.Rollback()
			lastErr = err
			continue
		}

		dbTx.Commit()
		processedCount++
	}

	status := http.StatusOK
	if lastErr != nil {
		status = http.StatusInternalServerError
	}

	c.JSON(status, gin.H{
		"message":         "Payroll calculation completed",
		"cycle_id":        cycle.ID,
		"processed_count": processedCount,
		"skipped_count":   skippedCount,
		"error": func() string {
			if lastErr != nil {
				return lastErr.Error()
			}
			return ""
		}(),
	})
}

// GetPayrollTransactions returns a summary table of all transactions for a given cycle
func GetPayrollTransactions(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	cycleID := c.Param("cycle_id")

	// Verify the cycle belongs to the user's company
	var cycle models.PayrollCycle
	if err := config.DB.Scopes(repository.ScopeCompanies(currentUser.CompaniesID)).
		First(&cycle, cycleID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cycle not found"})
		return
	}

	var transactions []models.PayrollTransaction
	if err := config.DB.Where("cycle_id = ?", cycle.ID).
		Preload("Employees").
		Preload("Employees.Department").
		Preload("Employees.Position").
		Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"cycle": cycle, "transactions": transactions})
}

// GetPayrollSlip returns transaction + all detail lines for a single employee payslip
func GetPayrollSlip(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	transactionID := c.Param("transaction_id")

	var transaction models.PayrollTransaction
	if err := config.DB.
		Preload("Employees").
		Preload("Employees.Department").
		Preload("Employees.Position").
		Preload("Details").
		First(&transaction, transactionID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	}

	// Verify the transaction's cycle belongs to the user's company (authorization check)
	var cycle models.PayrollCycle
	if err := config.DB.Scopes(repository.ScopeCompanies(currentUser.CompaniesID)).
		First(&cycle, transaction.CycleID).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"cycle": cycle, "slip": transaction})
}

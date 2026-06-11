package main

import (
	"log"
	"math"
	"payroll-backend/config"
	"payroll-backend/migrations"
	"payroll-backend/models"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func ptr(u uint) *uint {
	return &u
}

func upsertEmployeeCompanyAccess(employeeID, companyID, roleID uint, isDefault bool) {
	mapping := models.EmployeeCompanyAccess{
		EmployeeID: employeeID,
		CompanyID:  companyID,
		RoleID:     roleID,
		IsDefault:  isDefault,
		IsActive:   true,
	}
	config.DB.Where(models.EmployeeCompanyAccess{
		EmployeeID: employeeID,
		CompanyID:  companyID,
		RoleID:     roleID,
	}).Assign(mapping).FirstOrCreate(&mapping)
}

func syncAppModuleIDSequence() {
	config.DB.Exec(`
		SELECT setval(
			pg_get_serial_sequence('app_module', 'id'),
			COALESCE((SELECT MAX(id) FROM app_module), 1),
			(SELECT COUNT(*) > 0 FROM app_module)
		)
	`)
}

func main() {
	config.LoadDotEnv()
	config.ConnectDatabase()

	if err := migrations.RunOnce(config.DB, "20260525_drop_legacy_indexes", migrations.DropLegacyIndexes); err != nil {
		log.Fatalf("Legacy index cleanup failed: %v", err)
	}
	if err := migrations.RunOnce(config.DB, "20260525_rename_employee_company_access_table", migrations.RenameEmployeeCompanyAccessTable); err != nil {
		log.Fatalf("Employee-company-access table rename failed: %v", err)
	}
	if err := migrations.RunOnce(config.DB, "20260525_backfill_salaries_structure_companies", migrations.BackfillSalariesStructureCompanies); err != nil {
		log.Fatalf("Salaries company backfill failed: %v", err)
	}

	if err := migrations.AutoMigrateSchema(config.DB); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	if err := migrations.RunOnce(config.DB, "20260525_remove_legacy_elevated_role", migrations.RemoveLegacyElevatedRole); err != nil {
		log.Fatalf("Legacy elevated role cleanup failed: %v", err)
	}
	if err := migrations.RunOnce(config.DB, "20260525_backfill_employee_company_access", migrations.BackfillEmployeeCompanyAccess); err != nil {
		log.Fatalf("Employee-company-access backfill failed: %v", err)
	}

	// Initialize companies
	sysComp := models.Companies{Name: "System Administration", TaxID: "0000000000000"}
	config.DB.Where(models.Companies{TaxID: "0000000000000"}).Assign(sysComp).FirstOrCreate(&sysComp)

	compA := models.Companies{Name: "Tech Corp", TaxID: "1111111111111"}
	config.DB.Where(models.Companies{TaxID: "1111111111111"}).Assign(compA).FirstOrCreate(&compA)

	compB := models.Companies{Name: "Biz Group", TaxID: "2222222222222"}
	config.DB.Where(models.Companies{TaxID: "2222222222222"}).Assign(compB).FirstOrCreate(&compB)

	// Initialize payroll configs
	configSys := models.PayrollConfig{
		CompaniesID:    sysComp.ID,
		SsoRate:        5.0,
		SsoMaxAmount:   750,
		SsoMinSalary:   1650,
		TaxCalculation: "Progressive",
	}
	config.DB.Where(models.PayrollConfig{CompaniesID: sysComp.ID}).Assign(configSys).FirstOrCreate(&configSys)

	configA := models.PayrollConfig{
		CompaniesID:    compA.ID,
		SsoRate:        5.0,
		SsoMaxAmount:   750,
		SsoMinSalary:   1650,
		TaxCalculation: "Progressive",
	}
	config.DB.Where(models.PayrollConfig{CompaniesID: compA.ID}).Assign(configA).FirstOrCreate(&configA)

	configB := models.PayrollConfig{
		CompaniesID:    compB.ID,
		SsoRate:        5.0,
		SsoMaxAmount:   750,
		SsoMinSalary:   1650,
		TaxCalculation: "Progressive",
	}
	config.DB.Where(models.PayrollConfig{CompaniesID: compB.ID}).Assign(configB).FirstOrCreate(&configB)

	// Initialize attendance configs
	parseTime := func(h, m int) *time.Time {
		t := time.Date(2026, 1, 1, h, m, 0, 0, time.UTC)
		return &t
	}

	attConfigSys := models.AttendanceConfig{
		CompaniesID:               sysComp.ID,
		LateRatePerMinute:         5.0,
		AbsentRatePerDay:          300.0,
		WorkStartTime:             parseTime(9, 0),
		WorkEndTime:               parseTime(17, 0),
		OvertimeRatePerMinute:     5.0,
		ExtraRatePerDay:           300.0,
		IsOvertimeEarningsEnabled: true,
		IsExtraEarningsEnabled:    true,
	}
	config.DB.Unscoped().Delete(&models.AttendanceConfig{}, "companies_id = ?", sysComp.ID)
	config.DB.Create(&attConfigSys)

	attConfigA := models.AttendanceConfig{
		CompaniesID:               compA.ID,
		LateRatePerMinute:         5.0,
		AbsentRatePerDay:          300.0,
		WorkStartTime:             parseTime(9, 0),
		WorkEndTime:               parseTime(17, 0),
		OvertimeRatePerMinute:     5.0,
		ExtraRatePerDay:           300.0,
		IsOvertimeEarningsEnabled: true,
		IsExtraEarningsEnabled:    true,
	}
	config.DB.Unscoped().Delete(&models.AttendanceConfig{}, "companies_id = ?", compA.ID)
	config.DB.Create(&attConfigA)

	attConfigB := models.AttendanceConfig{
		CompaniesID:               compB.ID,
		LateRatePerMinute:         5.0,
		AbsentRatePerDay:          300.0,
		WorkStartTime:             parseTime(9, 0),
		WorkEndTime:               parseTime(17, 0),
		OvertimeRatePerMinute:     5.0,
		ExtraRatePerDay:           300.0,
		IsOvertimeEarningsEnabled: true,
		IsExtraEarningsEnabled:    true,
	}
	config.DB.Unscoped().Delete(&models.AttendanceConfig{}, "companies_id = ?", compB.ID)
	config.DB.Create(&attConfigB)

	// Initialize global banks
	banks := []models.Bank{
		{BankCode: "BBL", BankName: "Bangkok Bank", SwiftCode: "BKKBTHBK"},
		{BankCode: "KBANK", BankName: "Kasikorn Bank", SwiftCode: "KASITHBK"},
		{BankCode: "SCB", BankName: "Siam Commercial Bank", SwiftCode: "SICOTHBK"},
		{BankCode: "KTB", BankName: "Krungthai Bank", SwiftCode: "KRTHTHBK"},
		{BankCode: "BAY", BankName: "Bank of Ayudhya (Krungsri)", SwiftCode: "AYUDTHBK"},
		{BankCode: "TTB", BankName: "TTB Bank", SwiftCode: "TMBKTHBK"},
		{BankCode: "TISCO", BankName: "TISCO Bank", SwiftCode: "TFPCTHB1"},
		{BankCode: "UOB", BankName: "UOB Thailand", SwiftCode: "UOVBTHBK"},
		{BankCode: "GSB", BankName: "Government Savings Bank", SwiftCode: "GOSABANK"},
		{BankCode: "BAAC", BankName: "Bank for Agriculture & Agricultural Cooperatives", SwiftCode: "BAABTHBK"},
		{BankCode: "KKP", BankName: "Kiatnakin Phatra Bank", SwiftCode: "KKPBTHBK"},
		{BankCode: "CIMB", BankName: "CIMB Thai Bank", SwiftCode: "UBOBTHBK"},
		{BankCode: "LHB", BankName: "Land and Houses Bank", SwiftCode: "LAHRTHB1"},
	}
	for _, b := range banks {
		config.DB.Where(models.Bank{BankCode: b.BankCode}).Assign(b).FirstOrCreate(&b)
	}

	// Fetch bank IDs for employee seeding
	var bankBBL, bankKBANK, bankSCB, bankKTB, bankBAY, bankTTB models.Bank
	config.DB.Where("bank_code = ?", "BBL").First(&bankBBL)
	config.DB.Where("bank_code = ?", "KBANK").First(&bankKBANK)
	config.DB.Where("bank_code = ?", "SCB").First(&bankSCB)
	config.DB.Where("bank_code = ?", "KTB").First(&bankKTB)
	config.DB.Where("bank_code = ?", "BAY").First(&bankBAY)
	config.DB.Where("bank_code = ?", "TTB").First(&bankTTB)

	// Initialize employment types
	empTypeNames := []struct {
		Name        string
		Description string
	}{
		{"Full-time", "Permanent full-time employee"},
		{"Part-time", "Part-time employee working reduced hours"},
		{"Contract", "Fixed-term contract employee"},
		{"Probation", "Employee on probation period"},
		{"Intern", "Internship / trainee position"},
	}
	for _, comp := range []models.Companies{sysComp, compA, compB} {
		for _, et := range empTypeNames {
			rec := models.EmploymentType{CompaniesID: comp.ID, Name: et.Name, Description: et.Description}
			config.DB.Where(models.EmploymentType{CompaniesID: comp.ID, Name: et.Name}).Assign(rec).FirstOrCreate(&rec)
		}
	}

	// Fetch Full-time employment type per company for employee seeding
	var fullTimeEmpTypeSys, fullTimeEmpTypeA, fullTimeEmpTypeB models.EmploymentType
	config.DB.Where("companies_id = ? AND name = ?", sysComp.ID, "Full-time").First(&fullTimeEmpTypeSys)
	config.DB.Where("companies_id = ? AND name = ?", compA.ID, "Full-time").First(&fullTimeEmpTypeA)
	config.DB.Where("companies_id = ? AND name = ?", compB.ID, "Full-time").First(&fullTimeEmpTypeB)

	// Initialize categories
	earningCats := []models.EarningCategory{
		{Name: "Regular Pay"},
		{Name: "Allowance"},
		{Name: "Bonus"},
		{Name: "Overtime"},
		{Name: "Reimbursement"},
	}
	for _, ec := range earningCats {
		config.DB.Where(models.EarningCategory{Name: ec.Name}).Assign(ec).FirstOrCreate(&ec)
	}

	// --- Deduction Categories (Global) ---
	deductionCats := []models.DeductionCategory{
		{Name: "Statutory"},
		{Name: "Voluntary"},
		{Name: "Loan"},
	}
	for _, dc := range deductionCats {
		config.DB.Where(models.DeductionCategory{Name: dc.Name}).Assign(dc).FirstOrCreate(&dc)
	}

	// --- Per-Company Master Data ---
	for _, comp := range []models.Companies{sysComp, compA, compB} {
		// Work Locations
		locations := []models.WorkLocation{
			{CompaniesID: comp.ID, Name: "Office (HQ)"},
			{CompaniesID: comp.ID, Name: "Work From Home"},
			{CompaniesID: comp.ID, Name: "Client Site"},
		}
		for _, loc := range locations {
			config.DB.Where(models.WorkLocation{CompaniesID: comp.ID, Name: loc.Name}).Assign(loc).FirstOrCreate(&loc)
		}

		// Leave Types
		leaveTypes := []models.LeaveTypeSetting{
			{CompaniesID: comp.ID, Name: "Annual Leave", IsPaid: true, MaxDaysPerYear: 15},
			{CompaniesID: comp.ID, Name: "Sick Leave", IsPaid: true, MaxDaysPerYear: 30},
			{CompaniesID: comp.ID, Name: "Personal Leave", IsPaid: true, MaxDaysPerYear: 3},
			{CompaniesID: comp.ID, Name: "Unpaid Leave", IsPaid: false, MaxDaysPerYear: 0},
			{CompaniesID: comp.ID, Name: "Maternity Leave", IsPaid: true, MaxDaysPerYear: 98},
		}
		for _, lt := range leaveTypes {
			config.DB.Where(models.LeaveTypeSetting{CompaniesID: comp.ID, Name: lt.Name}).Assign(lt).FirstOrCreate(&lt)
		}

		// Initialize 2026 Thai holidays
		holidays := []models.Holiday{
			{CompaniesID: comp.ID, Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Name: "New Year's Day", IsPaid: true},
			{CompaniesID: comp.ID, Date: time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC), Name: "Makha Bucha Day", IsPaid: true},
			{CompaniesID: comp.ID, Date: time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC), Name: "Chakri Memorial Day", IsPaid: true},
			{CompaniesID: comp.ID, Date: time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC), Name: "Songkran Festival", IsPaid: true},
			{CompaniesID: comp.ID, Date: time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC), Name: "Songkran Festival", IsPaid: true},
			{CompaniesID: comp.ID, Date: time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC), Name: "Songkran Festival", IsPaid: true},
			{CompaniesID: comp.ID, Date: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), Name: "Labour Day", IsPaid: true},
			{CompaniesID: comp.ID, Date: time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC), Name: "Coronation Day", IsPaid: true},
			{CompaniesID: comp.ID, Date: time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC), Name: "Visakha Bucha Day", IsPaid: true},
			{CompaniesID: comp.ID, Date: time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC), Name: "King's Birthday", IsPaid: true},
			{CompaniesID: comp.ID, Date: time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC), Name: "Queen Mother's Birthday", IsPaid: true},
			{CompaniesID: comp.ID, Date: time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC), Name: "King Rama IX Memorial Day", IsPaid: true},
			{CompaniesID: comp.ID, Date: time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC), Name: "Chulalongkorn Day", IsPaid: true},
			{CompaniesID: comp.ID, Date: time.Date(2026, 12, 5, 0, 0, 0, 0, time.UTC), Name: "King Rama IX Birthday", IsPaid: true},
			{CompaniesID: comp.ID, Date: time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC), Name: "Constitution Day", IsPaid: true},
			{CompaniesID: comp.ID, Date: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), Name: "New Year's Eve", IsPaid: true},
		}
		for _, h := range holidays {
			dateStr := h.Date.Format("2006-01-02")
			config.DB.Where("companies_id = ? AND date = ?", comp.ID, dateStr).Assign(h).FirstOrCreate(&h)
		}

		dayOffs := []models.DayOff{
			{CompaniesID: comp.ID, Weekday: int(time.Sunday), Name: "Sunday Off", IsPaid: true},
			{CompaniesID: comp.ID, Weekday: int(time.Saturday), Name: "Saturday Off", IsPaid: true},
		}
		for _, d := range dayOffs {
			config.DB.Where("companies_id = ? AND weekday = ?", comp.ID, d.Weekday).Assign(d).FirstOrCreate(&d)
		}
	}

	// --- Earning Types ---
	// Base Salary is NOT listed here — it is managed via the Salaries module (SalariesStructure)
	earnings := []models.EarningType{
		{CompaniesID: sysComp.ID, Name: "Overtime (OT)", IsTaxable: true, IsSso: false, IsPermanent: false},
		{CompaniesID: compA.ID, Name: "Position Allowance", IsTaxable: true, IsSso: true, IsPermanent: true},
		{CompaniesID: compA.ID, Name: "Overtime (OT)", IsTaxable: true, IsSso: false, IsPermanent: false},
		{CompaniesID: compA.ID, Name: "Bonus", IsTaxable: true, IsSso: false, IsPermanent: false},
		{CompaniesID: compB.ID, Name: "Position Allowance", IsTaxable: true, IsSso: true, IsPermanent: true},
		{CompaniesID: compB.ID, Name: "Overtime (OT)", IsTaxable: true, IsSso: false, IsPermanent: false},
		{CompaniesID: compB.ID, Name: "Bonus", IsTaxable: true, IsSso: false, IsPermanent: false},
	}
	for _, e := range earnings {
		config.DB.Where(models.EarningType{CompaniesID: e.CompaniesID, Name: e.Name}).Assign(e).FirstOrCreate(&e)
	}

	// --- Deduction Types ---
	deductions := []models.DeductionType{
		{CompaniesID: compA.ID, Name: "Loan Repayment", IsTaxDeductible: false, IsSsoDeductible: false},
		{CompaniesID: compA.ID, Name: "Provident Fund", IsTaxDeductible: true, IsSsoDeductible: false},
		{CompaniesID: compB.ID, Name: "Loan Repayment", IsTaxDeductible: false, IsSsoDeductible: false},
		{CompaniesID: compB.ID, Name: "Provident Fund", IsTaxDeductible: true, IsSsoDeductible: false},
		{CompaniesID: sysComp.ID, Name: "Loan Repayment", IsTaxDeductible: false, IsSsoDeductible: false},
		{CompaniesID: sysComp.ID, Name: "Provident Fund", IsTaxDeductible: true, IsSsoDeductible: false},
	}
	for _, d := range deductions {
		config.DB.Where(models.DeductionType{CompaniesID: d.CompaniesID, Name: d.Name}).Assign(d).FirstOrCreate(&d)
	}

	// --- Roles ---
	sysRole := models.Roles{RolesName: "SYSTEM_ADMIN", Description: "System Admin Access", CompaniesID: sysComp.ID}
	config.DB.Where(models.Roles{RolesName: "SYSTEM_ADMIN", CompaniesID: sysComp.ID}).Assign(sysRole).FirstOrCreate(&sysRole)

	sysStaffRole := models.Roles{RolesName: "STAFF", Description: "System Staff", CompaniesID: sysComp.ID}
	config.DB.Where(models.Roles{RolesName: "STAFF", CompaniesID: sysComp.ID}).Assign(sysStaffRole).FirstOrCreate(&sysStaffRole)

	managerRoleA := models.Roles{RolesName: "MANAGER", Description: "Sees whole companies", CompaniesID: compA.ID}
	config.DB.Where(models.Roles{RolesName: "MANAGER", CompaniesID: compA.ID}).Assign(managerRoleA).FirstOrCreate(&managerRoleA)
	staffRoleA := models.Roles{RolesName: "STAFF", Description: "Sees only department", CompaniesID: compA.ID}
	config.DB.Where(models.Roles{RolesName: "STAFF", CompaniesID: compA.ID}).Assign(staffRoleA).FirstOrCreate(&staffRoleA)

	managerRoleB := models.Roles{RolesName: "MANAGER", Description: "Sees whole companies", CompaniesID: compB.ID}
	config.DB.Where(models.Roles{RolesName: "MANAGER", CompaniesID: compB.ID}).Assign(managerRoleB).FirstOrCreate(&managerRoleB)
	staffRoleB := models.Roles{RolesName: "STAFF", Description: "Sees only department", CompaniesID: compB.ID}
	config.DB.Where(models.Roles{RolesName: "STAFF", CompaniesID: compB.ID}).Assign(staffRoleB).FirstOrCreate(&staffRoleB)

	// --- Departments & Positions ---
	sysDept := models.Department{DeptName: "System IT", CompaniesID: sysComp.ID}
	config.DB.Where(models.Department{DeptName: "System IT", CompaniesID: sysComp.ID}).Assign(sysDept).FirstOrCreate(&sysDept)
	deptEngA := models.Department{DeptName: "Engineering", CompaniesID: compA.ID}
	config.DB.Where(models.Department{DeptName: "Engineering", CompaniesID: compA.ID}).Assign(deptEngA).FirstOrCreate(&deptEngA)
	deptHrA := models.Department{DeptName: "HR", CompaniesID: compA.ID}
	config.DB.Where(models.Department{DeptName: "HR", CompaniesID: compA.ID}).Assign(deptHrA).FirstOrCreate(&deptHrA)
	deptEngB := models.Department{DeptName: "Engineering", CompaniesID: compB.ID}
	config.DB.Where(models.Department{DeptName: "Engineering", CompaniesID: compB.ID}).Assign(deptEngB).FirstOrCreate(&deptEngB)

	sysPos := models.Position{PosName: "System Admin", CompaniesID: sysComp.ID}
	config.DB.Where(models.Position{PosName: "System Admin", CompaniesID: sysComp.ID}).Assign(sysPos).FirstOrCreate(&sysPos)
	posManagerA := models.Position{PosName: "Manager", CompaniesID: compA.ID}
	config.DB.Where(models.Position{PosName: "Manager", CompaniesID: compA.ID}).Assign(posManagerA).FirstOrCreate(&posManagerA)
	posStaffA := models.Position{PosName: "Engineer", CompaniesID: compA.ID}
	config.DB.Where(models.Position{PosName: "Engineer", CompaniesID: compA.ID}).Assign(posStaffA).FirstOrCreate(&posStaffA)
	posManagerB := models.Position{PosName: "Manager", CompaniesID: compB.ID}
	config.DB.Where(models.Position{PosName: "Manager", CompaniesID: compB.ID}).Assign(posManagerB).FirstOrCreate(&posManagerB)
	posStaffB := models.Position{PosName: "Engineer", CompaniesID: compB.ID}
	config.DB.Where(models.Position{PosName: "Engineer", CompaniesID: compB.ID}).Assign(posStaffB).FirstOrCreate(&posStaffB)
	// --- Modules ---
	deprecatedModuleKeys := []models.ModuleKey{
		"EarningCategoryPage",
	}
	config.DB.Model(&models.AppModule{}).
		Where("module_key IN ?", deprecatedModuleKeys).
		Update("is_active", false)

	modules := []models.AppModule{
		// --- Level 1: Main Categories ---
		{ID: 1, ModuleName: "Dashboard", ModuleKey: "DashboardPage", RoutePath: "/dashboard", Icon: "Dashboard", Order: 1, IsCategory: false},
		{ID: 2, ModuleName: "Employees", ModuleKey: "ModuleEmployees", RoutePath: "", Icon: "People", Order: 2, IsCategory: true},
		{ID: 3, ModuleName: "Attendance", ModuleKey: "ModuleAttendance", RoutePath: "", Icon: "Event", Order: 3, IsCategory: true},
		{ID: 4, ModuleName: "Payroll Processing", ModuleKey: "ModulePayroll", RoutePath: "", Icon: "AccountBalanceWallet", Order: 4, IsCategory: true},
		{ID: 5, ModuleName: "Payroll Config", ModuleKey: "ModulePayrollConfig", RoutePath: "", Icon: "SettingsSuggest", Order: 5, IsCategory: true},
		{ID: 6, ModuleName: "Settings", ModuleKey: "ModuleSettings", RoutePath: "", Icon: "Settings", Order: 6, IsCategory: true},
		{ID: 7, ModuleName: "Reports", ModuleKey: "ModuleReports", RoutePath: "", Icon: "FactCheck", Order: 7, IsCategory: true},
		{ID: 8, ModuleName: "Help Center", ModuleKey: "HelpPage", RoutePath: "/help", Icon: "HelpOutline", Order: 8, IsCategory: false},

		// Sub-Menus
		// Level 2: Sub-Menus under Employees
		{ModuleName: "Employee List", ModuleKey: "EmployeesListPage", RoutePath: "/employees", Icon: "Badge", Order: 1, ParentID: ptr(2)},
		{ModuleName: "Salaries", ModuleKey: "SalariesStructurePage", RoutePath: "/salaries", Icon: "Payments", Order: 2, ParentID: ptr(2)},
		{ModuleName: "Employee Earnings", ModuleKey: "EmployeeEarningsPage", RoutePath: "/employee-earnings", Icon: "AddCard", Order: 3, ParentID: ptr(2)},
		{ModuleName: "Employee Deductions", ModuleKey: "EmployeeDeductionsPage", RoutePath: "/employee-deductions", Icon: "PriceCheck", Order: 4, ParentID: ptr(2)},

		// Level 2: Sub-Menus under Attendance
		{ModuleName: "Timelogs", ModuleKey: "TimelogsListPage", RoutePath: "/timelogs", Icon: "Schedule", Order: 1, ParentID: ptr(3)},
		{ModuleName: "Leave Requests", ModuleKey: "LeaveRequestsPage", RoutePath: "/leave-requests", Icon: "EventNote", Order: 2, ParentID: ptr(3)},
		{ModuleName: "Leave Approved", ModuleKey: "LeaveApprovedPage", RoutePath: "/leave-approved", Icon: "FactCheck", Order: 3, ParentID: ptr(3)},
		{ModuleName: "Earning Summary", ModuleKey: "EarningSummaryPage", RoutePath: "/earning-summary", Icon: "TrendingUp", Order: 4, ParentID: ptr(3)},
		{ModuleName: "Deduction Summary", ModuleKey: "DeductionSummaryPage", RoutePath: "/deduction-summary", Icon: "Summarize", Order: 5, ParentID: ptr(3)},

		// Level 2: Sub-Menus under Payroll Processing
		{ModuleName: "Payroll Cycles", ModuleKey: "PayrollCyclesPage", RoutePath: "/payroll-cycles", Icon: "Sync", Order: 1, ParentID: ptr(4)},
		{ModuleName: "Run Payroll", ModuleKey: "RunPayrollPage", RoutePath: "/run-payroll", Icon: "PlayCircle", Order: 2, ParentID: ptr(4)},
		{ModuleName: "Cycle Earnings", ModuleKey: "CycleEarningsPage", RoutePath: "/cycle-earnings", Icon: "AddCard", Order: 3, ParentID: ptr(4)},
		{ModuleName: "Cycle Deductions", ModuleKey: "CycleDeductionsPage", RoutePath: "/cycle-deductions", Icon: "PriceCheck", Order: 4, ParentID: ptr(4)},
		{ModuleName: "Transaction", ModuleKey: "TransactionsPage", RoutePath: "/transactions", Icon: "ReceiptLong", Order: 5, ParentID: ptr(4)},

		// Level 2: Sub-Menus under Payroll Config
		{ModuleName: "Global Settings", ModuleKey: "PayrollGlobalSettingsPage", RoutePath: "/payroll-global", Icon: "SettingsSuggest", Order: 1, ParentID: ptr(5)},
		{ModuleName: "Earning Types", ModuleKey: "EarningTypesPage", RoutePath: "/earning-types", Icon: "AddCard", Order: 2, ParentID: ptr(5)},
		{ModuleName: "Deduction Types", ModuleKey: "DeductionTypesPage", RoutePath: "/deduction-types", Icon: "PriceCheck", Order: 3, ParentID: ptr(5)},

		// Level 2: Sub-Menus under Settings
		{ModuleName: "Companies", ModuleKey: "CompaniesPage", RoutePath: "/companies", Icon: "Business", Order: 1, ParentID: ptr(6)},
		{ModuleName: "Department", ModuleKey: "DepartmentSettings", RoutePath: "/departments", Icon: "CorporateFare", Order: 2, ParentID: ptr(6)},
		{ModuleName: "Position", ModuleKey: "PositionSettings", RoutePath: "/positions", Icon: "WorkOutline", Order: 3, ParentID: ptr(6)},
		{ModuleName: "Employment Types", ModuleKey: "EmploymentTypePage", RoutePath: "/employment-types", Icon: "WorkHistory", Order: 4, ParentID: ptr(6)},
		{ModuleName: "Roles", ModuleKey: "RolesPage", RoutePath: "/roles", Icon: "ManageAccounts", Order: 5, ParentID: ptr(6)},
		{ModuleName: "Permission", ModuleKey: "PermissionSettings", RoutePath: "/permissions", Icon: "LockPerson", Order: 6, ParentID: ptr(6)},
		{ModuleName: "Employee Company Access", ModuleKey: "EmployeeCompanyAccessPage", RoutePath: "/employee-company-access", Icon: "AdminPanelSettings", Order: 7, ParentID: ptr(6)},
		{ModuleName: "Attendance Settings", ModuleKey: "AttendanceSettingsPage", RoutePath: "/attendance-settings", Icon: "Tune", Order: 8, ParentID: ptr(6)},
		{ModuleName: "Bank Master", ModuleKey: "MasterBankPage", RoutePath: "/master-banks", Icon: "AccountBalance", Order: 9, ParentID: ptr(6)},
		{ModuleName: "Holiday Calendar", ModuleKey: "HolidayCalendarPage", RoutePath: "/holidays", Icon: "CalendarMonth", Order: 10, ParentID: ptr(6)},
		{ModuleName: "Day Off", ModuleKey: "DayOffPage", RoutePath: "/day-offs", Icon: "EventAvailable", Order: 11, ParentID: ptr(6)},
		{ModuleName: "Work Locations", ModuleKey: "WorkLocationPage", RoutePath: "/work-locations", Icon: "LocationOn", Order: 12, ParentID: ptr(6)},
		{ModuleName: "Leave Types", ModuleKey: "MasterLeaveTypePage", RoutePath: "/leave-types", Icon: "EventAvailable", Order: 13, ParentID: ptr(6)},

		// Level 2: Sub-Menus under Reports
		{ModuleName: "Payslips", ModuleKey: "PayslipsReportPage", RoutePath: "/payslips", Icon: "Description", Order: 1, ParentID: ptr(7)},
		{ModuleName: "Bank Transfer", ModuleKey: "BankTransferReportPage", RoutePath: "/reports/bank-transfer", Icon: "AccountBalance", Order: 2, ParentID: ptr(7)},
		{ModuleName: "Tax Forms", ModuleKey: "TaxFormsReportPage", RoutePath: "/reports/tax-forms", Icon: "Receipt", Order: 3, ParentID: ptr(7)},
		{ModuleName: "Social Security (SSO)", ModuleKey: "SSOReportPage", RoutePath: "/reports/sso", Icon: "HealthAndSafety", Order: 4, ParentID: ptr(7)},
		{ModuleName: "Payroll Analysis", ModuleKey: "PayrollAnalysisPage", RoutePath: "/reports/payroll-analysis", Icon: "Analytics", Order: 5, ParentID: ptr(7)},
	}

	for _, m := range modules {
		m.IsActive = true
		var existingModule models.AppModule
		config.DB.Where(models.AppModule{ModuleKey: m.ModuleKey}).Assign(m).FirstOrCreate(&existingModule)
		if m.ID != 0 {
			syncAppModuleIDSequence()
		}

		mID := existingModule.ID

		// SYSTEM_ADMIN (sysComp) gets full access through role/module/access mappings.
		upsert := func(roleID, compID uint, scope models.DataScope, full bool) {
			acc := models.RolesModuleAccess{
				RolesID: roleID, ModuleID: mID, CompaniesID: compID,
				CanView: true, CanCreate: full, CanEdit: full, CanDelete: full,
				DataScope: scope,
			}
			config.DB.Where(models.RolesModuleAccess{RolesID: roleID, ModuleID: mID}).Assign(acc).FirstOrCreate(&acc)
		}

		upsert(sysRole.ID, sysComp.ID, models.ScopeAll, true)            // SYSTEM_ADMIN: all mapped companies
		upsert(sysStaffRole.ID, sysComp.ID, models.ScopeCompanies, true) // sysStaff: full within company

		// compA roles
		upsert(managerRoleA.ID, compA.ID, models.ScopeCompanies, true) // MANAGER: full within company
		upsert(staffRoleA.ID, compA.ID, models.ScopeDepartment, false) // STAFF: view only, dept scope

		// compB roles
		upsert(managerRoleB.ID, compB.ID, models.ScopeCompanies, true)
		upsert(staffRoleB.ID, compB.ID, models.ScopeDepartment, false)
	}

	// --- Employees Seeding (Using your existing logic) ---
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("admin@admin.com"), 10)
	hashStr := string(hashedPassword)

	admin := models.Employees{Email: "admin@admin.com"}
	config.DB.Unscoped().Where(models.Employees{Email: admin.Email}).
		Assign(models.Employees{
			CompaniesID:   sysComp.ID,
			RolesID:       sysRole.ID,
			EmployeesCode: "SYS-001",
			FirstName:     "System", LastName: "Admin",
			Password: hashStr, DeptID: sysDept.ID, PosID: sysPos.ID,
		}).FirstOrCreate(&admin)
	upsertEmployeeCompanyAccess(admin.ID, sysComp.ID, sysRole.ID, true)
	upsertEmployeeCompanyAccess(admin.ID, compA.ID, sysRole.ID, false)
	upsertEmployeeCompanyAccess(admin.ID, compB.ID, sysRole.ID, false)

	type empSeed struct {
		emp         models.Employees
		salary      float64
		bankID      *uint
		bankName    string
		accountName string
		accountNo   string
	}

	usersToSeed := []empSeed{
		{models.Employees{CompaniesID: sysComp.ID, RolesID: sysStaffRole.ID, DeptID: sysDept.ID, PosID: sysPos.ID, EmploymentTypeID: &fullTimeEmpTypeSys.ID, EmployeesCode: "SYS-002", FirstName: "Test", LastName: "Staff", Email: "staff@admin.com", Password: hashStr, IsActive: true}, 0, &bankTTB.ID, bankTTB.BankName, "Test Staff", "6789012345"},
		{models.Employees{CompaniesID: compA.ID, RolesID: managerRoleA.ID, DeptID: deptEngA.ID, PosID: posManagerA.ID, EmploymentTypeID: &fullTimeEmpTypeA.ID, EmployeesCode: "EMP-A-01", FirstName: "Alice", LastName: "Johnson", Email: "alice@techa.com", Password: hashStr, IsActive: true}, 45250.50, &bankKBANK.ID, bankKBANK.BankName, "Alice Johnson", "1234567890"},
		{models.Employees{CompaniesID: compA.ID, RolesID: staffRoleA.ID, DeptID: deptEngA.ID, PosID: posStaffA.ID, EmploymentTypeID: &fullTimeEmpTypeA.ID, EmployeesCode: "EMP-A-02", FirstName: "Bob", LastName: "Smith", Email: "bob@techa.com", Password: hashStr, IsActive: true}, 30750.00, &bankSCB.ID, bankSCB.BankName, "Bob Smith", "2345678901"},
		{models.Employees{CompaniesID: compA.ID, RolesID: staffRoleA.ID, DeptID: deptHrA.ID, PosID: posStaffA.ID, EmploymentTypeID: &fullTimeEmpTypeA.ID, EmployeesCode: "EMP-A-03", FirstName: "Carol", LastName: "Davis", Email: "carol@techa.com", Password: hashStr, IsActive: true}, 28500.75, &bankBBL.ID, bankBBL.BankName, "Carol Davis", "3456789012"},
		{models.Employees{CompaniesID: compB.ID, RolesID: managerRoleB.ID, DeptID: deptEngB.ID, PosID: posManagerB.ID, EmploymentTypeID: &fullTimeEmpTypeB.ID, EmployeesCode: "EMP-B-01", FirstName: "David", LastName: "Wilson", Email: "david@bizgroup.com", Password: hashStr, IsActive: true}, 50125.00, &bankKTB.ID, bankKTB.BankName, "David Wilson", "4567890123"},
		{models.Employees{CompaniesID: compB.ID, RolesID: staffRoleB.ID, DeptID: deptEngB.ID, PosID: posStaffB.ID, EmploymentTypeID: &fullTimeEmpTypeB.ID, EmployeesCode: "EMP-B-02", FirstName: "Eve", LastName: "Brown", Email: "eve@bizgroup.com", Password: hashStr, IsActive: true}, 32300.25, &bankBAY.ID, bankBAY.BankName, "Eve Brown", "5678901234"},
		{models.Employees{CompaniesID: compB.ID, RolesID: staffRoleB.ID, DeptID: deptEngB.ID, PosID: posStaffB.ID, EmploymentTypeID: &fullTimeEmpTypeB.ID, EmployeesCode: "EMP-B-03", FirstName: "Frank", LastName: "Garcia", Email: "frank@bizgroup.com", Password: hashStr, IsActive: true}, 35450.00, &bankBBL.ID, bankBBL.BankName, "Frank Garcia", "6789012345"},
	}

	effDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for _, s := range usersToSeed {
		var existingEmp models.Employees
		config.DB.Unscoped().Where(models.Employees{Email: s.emp.Email}).Assign(s.emp).FirstOrCreate(&existingEmp)
		upsertEmployeeCompanyAccess(existingEmp.ID, s.emp.CompaniesID, s.emp.RolesID, true)

		// Update bank fields on employee
		config.DB.Model(&existingEmp).Updates(map[string]interface{}{
			"bank_id":      s.bankID,
			"bank_name":    s.bankName,
			"account_name": s.accountName,
			"account_no":   s.accountNo,
		})

		if s.salary > 0 {
			salary := models.SalariesStructure{CompaniesID: existingEmp.CompaniesID, EmployeesID: existingEmp.ID, BaseSalaries: s.salary, EffectiveDate: effDate}
			config.DB.Where(models.SalariesStructure{EmployeesID: existingEmp.ID}).Assign(salary).FirstOrCreate(&salary)
		} else {
			var existing models.SalariesStructure
			config.DB.Unscoped().Where("employees_id = ? AND deleted_at = 0", existingEmp.ID).Delete(&existing)
		}
	}

	// Ensure all existing employees are set to Full-time employment type
	config.DB.Model(&models.Employees{}).Where("employment_type_id IS NULL AND companies_id = ?", sysComp.ID).Update("employment_type_id", fullTimeEmpTypeSys.ID)
	config.DB.Model(&models.Employees{}).Where("employment_type_id IS NULL AND companies_id = ?", compA.ID).Update("employment_type_id", fullTimeEmpTypeA.ID)
	config.DB.Model(&models.Employees{}).Where("employment_type_id IS NULL AND companies_id = ?", compB.ID).Update("employment_type_id", fullTimeEmpTypeB.ID)

	// --- TimeLogs for March & April 2026 (payroll cycle test data with identical data) ---
	// Fetch employees by email so we can reference their IDs
	var alice, bob, carol, david, eve, frank, staff models.Employees
	config.DB.Unscoped().Where("email = ?", "alice@techa.com").First(&alice)
	config.DB.Unscoped().Where("email = ?", "bob@techa.com").First(&bob)
	config.DB.Unscoped().Where("email = ?", "carol@techa.com").First(&carol)
	config.DB.Unscoped().Where("email = ?", "david@bizgroup.com").First(&david)
	config.DB.Unscoped().Where("email = ?", "eve@bizgroup.com").First(&eve)
	config.DB.Unscoped().Where("email = ?", "frank@bizgroup.com").First(&frank)
	config.DB.Unscoped().Where("email = ?", "staff@admin.com").First(&staff)

	tp := func(t time.Time) *time.Time { return &t }
	d := func(month time.Month, day, checkInHour, checkInMin, checkOutHour, checkOutMin int) (ci, co *time.Time) {
		ci = tp(time.Date(2026, month, day, checkInHour, checkInMin, 0, 0, time.UTC))
		co = tp(time.Date(2026, month, day, checkOutHour, checkOutMin, 0, 0, time.UTC))
		return
	}

	type tlSeed struct {
		empID    uint
		compID   uint
		checkIn  *time.Time
		checkOut *time.Time
		legacy   models.AttendanceStatus
		remark   string
	}

	// Alice (Tech Corp MANAGER): perfect attendance — on time every day, heavy OT
	// Bob   (Tech Corp STAFF):  some OT, late every Thu
	// Carol (Tech Corp STAFF):  mostly late check-ins, no OT
	// David (Biz Group MANAGER): normal + OT on Tue/Thu/Sat
	// Eve   (Biz Group STAFF):   mixed late + OT
	// staff@admin.com:           clean, on time, no OT
	//
	// === FEBRUARY 2026 ===
	// Day-offs: Sunday (0) + Monday (1)
	// Working days (Tue–Sat): 3,4,5,6,7 / 10,11,12,13,14 / 17,18,19,20,21 / 24,25,26,27,28

	// Alice — all 20 working days, perfect attendance (9-5, no OT, no late)
	ciA4, coA4 := d(time.February, 4, 9, 0, 17, 0)    // Wed w1  normal
	ciA5, coA5 := d(time.February, 5, 9, 0, 17, 0)    // Thu w1  normal
	ciA6, coA6 := d(time.February, 6, 9, 0, 17, 0)    // Fri w1  normal
	ciA7, coA7 := d(time.February, 7, 9, 0, 17, 0)    // Sat w1  normal
	ciA10, coA10 := d(time.February, 10, 9, 0, 17, 0) // Tue w2  normal
	ciA11, coA11 := d(time.February, 11, 9, 0, 17, 0) // Wed w2  normal
	ciA12, coA12 := d(time.February, 12, 9, 0, 17, 0) // Thu w2  normal
	ciA13, coA13 := d(time.February, 13, 9, 0, 17, 0) // Fri w2  normal
	ciA14, coA14 := d(time.February, 14, 9, 0, 17, 0) // Sat w2  normal
	ciA17, coA17 := d(time.February, 17, 9, 0, 17, 0) // Tue w3  normal
	ciA18, coA18 := d(time.February, 18, 9, 0, 17, 0) // Wed w3  normal
	ciA19, coA19 := d(time.February, 19, 9, 0, 17, 0) // Thu w3  normal
	ciA20, coA20 := d(time.February, 20, 9, 0, 17, 0) // Fri w3  normal
	ciA21, coA21 := d(time.February, 21, 9, 0, 17, 0) // Sat w3  normal
	ciA24, coA24 := d(time.February, 24, 9, 0, 17, 0) // Tue w4  normal
	ciA25, coA25 := d(time.February, 25, 9, 0, 17, 0) // Wed w4  normal
	ciA26, coA26 := d(time.February, 26, 9, 0, 17, 0) // Thu w4  normal
	ciA27, coA27 := d(time.February, 27, 9, 0, 17, 0) // Fri w4  normal
	ciA28, coA28 := d(time.February, 28, 9, 0, 17, 0) // Sat w4  normal

	// Bob — all 20 working days, late every Thu (5,12,19,26) only, no OT
	ciB4, coB4 := d(time.February, 4, 9, 0, 17, 0)      // Wed w1  normal
	ciB5, coB5 := d(time.February, 5, 10, 30, 17, 0)    // Thu w1  late 6.5h
	ciB6, coB6 := d(time.February, 6, 9, 0, 17, 0)      // Fri w1  normal
	ciB7, coB7 := d(time.February, 7, 9, 0, 17, 0)      // Sat w1  normal
	ciB10, coB10 := d(time.February, 10, 9, 0, 17, 0)   // Tue w2  normal
	ciB11, coB11 := d(time.February, 11, 9, 0, 17, 0)   // Wed w2  normal
	ciB12, coB12 := d(time.February, 12, 10, 30, 17, 0) // Thu w2  late 6.5h
	ciB13, coB13 := d(time.February, 13, 9, 0, 17, 0)   // Fri w2  normal
	ciB14, coB14 := d(time.February, 14, 9, 0, 17, 0)   // Sat w2  normal
	ciB17, coB17 := d(time.February, 17, 9, 0, 17, 0)   // Tue w3  normal
	ciB18, coB18 := d(time.February, 18, 9, 0, 17, 0)   // Wed w3  normal
	ciB19, coB19 := d(time.February, 19, 10, 30, 17, 0) // Thu w3  late 6.5h
	ciB20, coB20 := d(time.February, 20, 9, 0, 17, 0)   // Fri w3  normal
	ciB21, coB21 := d(time.February, 21, 9, 0, 17, 0)   // Sat w3  normal
	ciB24, coB24 := d(time.February, 24, 9, 0, 17, 0)   // Tue w4  normal
	ciB25, coB25 := d(time.February, 25, 9, 0, 17, 0)   // Wed w4  normal
	ciB26, coB26 := d(time.February, 26, 10, 30, 17, 0) // Thu w4  late 6.5h
	ciB27, coB27 := d(time.February, 27, 9, 0, 17, 0)   // Fri w4  normal
	ciB28, coB28 := d(time.February, 28, 9, 0, 17, 0)   // Sat w4  normal

	// Carol — 20 working days: 4 absent + some on-time + some late
	var ciC4, coC4 *time.Time                           // Wed w1  ABSENT
	ciC5, coC5 := d(time.February, 5, 9, 0, 17, 0)      // Thu w1  on time 8h
	ciC6, coC6 := d(time.February, 6, 10, 15, 17, 0)    // Fri w1  late arrival
	var ciC7, coC7 *time.Time                           // Sat w1  ABSENT
	ciC10, coC10 := d(time.February, 10, 10, 0, 17, 0)  // Tue w2  late arrival
	ciC11, coC11 := d(time.February, 11, 10, 30, 17, 0) // Wed w2  late arrival
	ciC12, coC12 := d(time.February, 12, 9, 0, 17, 0)   // Thu w2  on time 8h
	var ciC13, coC13 *time.Time                         // Fri w2  ABSENT
	ciC14, coC14 := d(time.February, 14, 10, 0, 17, 0)  // Sat w2  late arrival
	ciC17, coC17 := d(time.February, 17, 9, 0, 17, 0)   // Tue w3  on time 8h
	ciC18, coC18 := d(time.February, 18, 10, 0, 17, 0)  // Wed w3  late arrival
	ciC19, coC19 := d(time.February, 19, 10, 30, 17, 0) // Thu w3  late arrival
	ciC20, coC20 := d(time.February, 20, 9, 0, 17, 0)   // Fri w3  on time 8h
	ciC21, coC21 := d(time.February, 21, 10, 15, 17, 0) // Sat w3  late arrival
	var ciC24, coC24 *time.Time                         // Tue w4  ABSENT
	ciC25, coC25 := d(time.February, 25, 9, 0, 17, 0)   // Wed w4  on time 8h
	ciC26, coC26 := d(time.February, 26, 10, 30, 17, 0) // Thu w4  late arrival
	ciC27, coC27 := d(time.February, 27, 10, 0, 17, 0)  // Fri w4  late arrival
	ciC28, coC28 := d(time.February, 28, 9, 0, 17, 0)   // Sat w4  on time 8h

	// David — all 20 working days, normal hours (9-5, no OT)
	ciD4, coD4 := d(time.February, 4, 9, 0, 17, 0)    // Wed w1  normal
	ciD5, coD5 := d(time.February, 5, 9, 0, 17, 0)    // Thu w1  normal
	ciD6, coD6 := d(time.February, 6, 9, 0, 17, 0)    // Fri w1  normal
	ciD7, coD7 := d(time.February, 7, 9, 0, 17, 0)    // Sat w1  normal
	ciD10, coD10 := d(time.February, 10, 9, 0, 17, 0) // Tue w2  normal
	ciD11, coD11 := d(time.February, 11, 9, 0, 17, 0) // Wed w2  normal
	ciD12, coD12 := d(time.February, 12, 9, 0, 17, 0) // Thu w2  normal
	ciD13, coD13 := d(time.February, 13, 9, 0, 17, 0) // Fri w2  normal
	ciD14, coD14 := d(time.February, 14, 9, 0, 17, 0) // Sat w2  normal
	ciD17, coD17 := d(time.February, 17, 9, 0, 17, 0) // Tue w3  normal
	ciD18, coD18 := d(time.February, 18, 9, 0, 17, 0) // Wed w3  normal
	ciD19, coD19 := d(time.February, 19, 9, 0, 17, 0) // Thu w3  normal
	ciD20, coD20 := d(time.February, 20, 9, 0, 17, 0) // Fri w3  normal
	ciD21, coD21 := d(time.February, 21, 9, 0, 17, 0) // Sat w3  normal
	ciD24, coD24 := d(time.February, 24, 9, 0, 17, 0) // Tue w4  normal
	ciD25, coD25 := d(time.February, 25, 9, 0, 17, 0) // Wed w4  normal
	ciD26, coD26 := d(time.February, 26, 9, 0, 17, 0) // Thu w4  normal
	ciD27, coD27 := d(time.February, 27, 9, 0, 17, 0) // Fri w4  normal
	ciD28, coD28 := d(time.February, 28, 9, 0, 17, 0) // Sat w4  normal

	// Eve — OT some days (roughly 6 days with OT spread throughout)
	ciE4, coE4 := d(time.February, 4, 9, 0, 18, 0)     // Wed w1  1h OT
	ciE5, coE5 := d(time.February, 5, 9, 0, 17, 0)     // Thu w1  normal
	ciE6, coE6 := d(time.February, 6, 9, 0, 18, 30)    // Fri w1  1.5h OT
	ciE7, coE7 := d(time.February, 7, 9, 0, 17, 0)     // Sat w1  normal
	ciE10, coE10 := d(time.February, 10, 9, 0, 17, 0)  // Tue w2  normal
	ciE11, coE11 := d(time.February, 11, 9, 0, 18, 30) // Wed w2  1.5h OT
	ciE12, coE12 := d(time.February, 12, 9, 0, 17, 0)  // Thu w2  normal
	ciE13, coE13 := d(time.February, 13, 9, 0, 18, 0)  // Fri w2  1h OT
	ciE14, coE14 := d(time.February, 14, 9, 0, 17, 0)  // Sat w2  normal
	ciE17, coE17 := d(time.February, 17, 9, 0, 17, 0)  // Tue w3  normal
	ciE18, coE18 := d(time.February, 18, 9, 0, 18, 30) // Wed w3  1.5h OT
	ciE19, coE19 := d(time.February, 19, 9, 0, 17, 0)  // Thu w3  normal
	ciE20, coE20 := d(time.February, 20, 9, 0, 18, 0)  // Fri w3  1h OT
	ciE21, coE21 := d(time.February, 21, 9, 0, 17, 0)  // Sat w3  normal
	ciE24, coE24 := d(time.February, 24, 9, 0, 17, 0)  // Tue w4  normal
	ciE25, coE25 := d(time.February, 25, 9, 0, 18, 0)  // Wed w4  1h OT
	ciE26, coE26 := d(time.February, 26, 9, 0, 17, 0)  // Thu w4  normal
	ciE27, coE27 := d(time.February, 27, 9, 0, 17, 0)  // Fri w4  normal
	ciE28, coE28 := d(time.February, 28, 9, 0, 17, 0)  // Sat w4  normal

	// Frank — normal 9-5 most days, extra work on holidays (2-4 days)
	ciFk4, coFk4 := d(time.February, 4, 9, 0, 17, 0)    // Wed w1  normal
	ciFk5, coFk5 := d(time.February, 5, 9, 0, 17, 0)    // Thu w1  normal
	ciFk6, coFk6 := d(time.February, 6, 9, 0, 17, 0)    // Fri w1  normal
	ciFk7, coFk7 := d(time.February, 7, 9, 0, 17, 0)    // Sat w1  normal
	ciFk10, coFk10 := d(time.February, 10, 9, 0, 17, 0) // Tue w2  normal
	ciFk11, coFk11 := d(time.February, 11, 9, 0, 17, 0) // Wed w2  normal
	ciFk12, coFk12 := d(time.February, 12, 9, 0, 17, 0) // Thu w2  normal
	ciFk13, coFk13 := d(time.February, 13, 9, 0, 17, 0) // Fri w2  normal
	ciFk14, coFk14 := d(time.February, 14, 9, 0, 17, 0) // Sat w2  normal
	ciFk17, coFk17 := d(time.February, 17, 9, 0, 17, 0) // Tue w3  normal
	ciFk18, coFk18 := d(time.February, 18, 9, 0, 17, 0) // Wed w3  normal
	ciFk19, coFk19 := d(time.February, 19, 9, 0, 17, 0) // Thu w3  normal
	ciFk20, coFk20 := d(time.February, 20, 9, 0, 17, 0) // Fri w3  normal
	ciFk21, coFk21 := d(time.February, 21, 9, 0, 17, 0) // Sat w3  normal
	ciFk24, coFk24 := d(time.February, 24, 9, 0, 17, 0) // Tue w4  normal
	ciFk25, coFk25 := d(time.February, 25, 9, 0, 17, 0) // Wed w4  normal
	ciFk26, coFk26 := d(time.February, 26, 9, 0, 17, 0) // Thu w4  normal
	ciFk27, coFk27 := d(time.February, 27, 9, 0, 17, 0) // Fri w4  normal
	ciFk28, coFk28 := d(time.February, 28, 9, 0, 17, 0) // Sat w4  normal

	// Staff (sysComp) — all 20 working days, clean on time, no OT
	ciS4, coS4 := d(time.February, 4, 9, 0, 17, 0)
	ciS5, coS5 := d(time.February, 5, 9, 0, 17, 0)
	ciS6, coS6 := d(time.February, 6, 9, 0, 17, 0)
	ciS7, coS7 := d(time.February, 7, 9, 0, 17, 0)
	ciS10, coS10 := d(time.February, 10, 9, 0, 17, 0)
	ciS11, coS11 := d(time.February, 11, 9, 0, 17, 0)
	ciS12, coS12 := d(time.February, 12, 9, 0, 17, 0)
	ciS13, coS13 := d(time.February, 13, 9, 0, 17, 0)
	ciS14, coS14 := d(time.February, 14, 9, 0, 17, 0)
	ciS17, coS17 := d(time.February, 17, 9, 0, 17, 0)
	ciS18, coS18 := d(time.February, 18, 9, 0, 17, 0)
	ciS19, coS19 := d(time.February, 19, 9, 0, 17, 0)
	ciS20, coS20 := d(time.February, 20, 9, 0, 17, 0)
	ciS21, coS21 := d(time.February, 21, 9, 0, 17, 0)
	ciS24, coS24 := d(time.February, 24, 9, 0, 17, 0)
	ciS25, coS25 := d(time.February, 25, 9, 0, 17, 0)
	ciS26, coS26 := d(time.February, 26, 9, 0, 17, 0)
	ciS27, coS27 := d(time.February, 27, 9, 0, 17, 0)
	ciS28, coS28 := d(time.February, 28, 9, 0, 17, 0)

	// Extra weekend work in February — Frank only
	ciFkFebExtra1, coFkFebExtra1 := d(time.February, 1, 10, 0, 17, 0) // Sun Feb 1 — Frank holiday work
	ciFkFebExtra2, coFkFebExtra2 := d(time.February, 15, 9, 0, 17, 0) // Sun Feb 15 — Frank holiday work
	ciFkFebExtra3, coFkFebExtra3 := d(time.February, 22, 9, 0, 17, 0) // Sun Feb 22 — Frank holiday work

	timelogs := []tlSeed{
		// === FEBRUARY 2026 ===
		// Alice — perfect attendance, OT every working day (0 absent, 0 late)
		{alice.ID, compA.ID, ciA4, coA4, models.StatusOnTime, "Sprint deadline"},
		{alice.ID, compA.ID, ciA5, coA5, models.StatusOnTime, ""},
		{alice.ID, compA.ID, ciA6, coA6, models.StatusOnTime, "Client meeting"},
		{alice.ID, compA.ID, ciA7, coA7, models.StatusOnTime, ""},
		{alice.ID, compA.ID, ciA10, coA10, models.StatusOnTime, "Sprint planning"},
		{alice.ID, compA.ID, ciA11, coA11, models.StatusOnTime, "Sprint review"},
		{alice.ID, compA.ID, ciA12, coA12, models.StatusOnTime, ""},
		{alice.ID, compA.ID, ciA13, coA13, models.StatusOnTime, "Backlog grooming"},
		{alice.ID, compA.ID, ciA14, coA14, models.StatusOnTime, ""},
		{alice.ID, compA.ID, ciA17, coA17, models.StatusOnTime, "Quarter close"},
		{alice.ID, compA.ID, ciA18, coA18, models.StatusOnTime, ""},
		{alice.ID, compA.ID, ciA19, coA19, models.StatusOnTime, ""},
		{alice.ID, compA.ID, ciA20, coA20, models.StatusOnTime, "Standup"},
		{alice.ID, compA.ID, ciA21, coA21, models.StatusOnTime, ""},
		{alice.ID, compA.ID, ciA24, coA24, models.StatusOnTime, ""},
		{alice.ID, compA.ID, ciA25, coA25, models.StatusOnTime, "Release prep"},
		{alice.ID, compA.ID, ciA26, coA26, models.StatusOnTime, ""},
		{alice.ID, compA.ID, ciA27, coA27, models.StatusOnTime, "Month end"},
		{alice.ID, compA.ID, ciA28, coA28, models.StatusOnTime, ""},
		// Bob — some OT, late every Thu
		{bob.ID, compA.ID, ciB4, coB4, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB5, coB5, models.StatusLate, "Traffic"},
		{bob.ID, compA.ID, ciB6, coB6, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB7, coB7, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB10, coB10, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB11, coB11, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB12, coB12, models.StatusLate, "Traffic"},
		{bob.ID, compA.ID, ciB13, coB13, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB14, coB14, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB17, coB17, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB18, coB18, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB19, coB19, models.StatusLate, "Traffic"},
		{bob.ID, compA.ID, ciB20, coB20, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB21, coB21, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB24, coB24, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB25, coB25, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB26, coB26, models.StatusLate, "Traffic"},
		{bob.ID, compA.ID, ciB27, coB27, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB28, coB28, models.StatusOnTime, ""},
		// Carol — 4 absent, rest on-time
		{carol.ID, compA.ID, ciC4, coC4, models.StatusAbsent, "Unexcused absence"},
		{carol.ID, compA.ID, ciC5, coC5, models.StatusOnTime, ""},
		{carol.ID, compA.ID, ciC6, coC6, models.StatusOnTime, ""},
		{carol.ID, compA.ID, ciC7, coC7, models.StatusAbsent, "Unexcused absence"},
		{carol.ID, compA.ID, ciC10, coC10, models.StatusOnTime, ""},
		{carol.ID, compA.ID, ciC11, coC11, models.StatusOnTime, ""},
		{carol.ID, compA.ID, ciC12, coC12, models.StatusOnTime, ""},
		{carol.ID, compA.ID, ciC13, coC13, models.StatusAbsent, "Unexcused absence"},
		{carol.ID, compA.ID, ciC14, coC14, models.StatusOnTime, ""},
		{carol.ID, compA.ID, ciC17, coC17, models.StatusOnTime, ""},
		{carol.ID, compA.ID, ciC18, coC18, models.StatusOnTime, ""},
		{carol.ID, compA.ID, ciC19, coC19, models.StatusOnTime, ""},
		{carol.ID, compA.ID, ciC20, coC20, models.StatusOnTime, ""},
		{carol.ID, compA.ID, ciC21, coC21, models.StatusOnTime, ""},
		{carol.ID, compA.ID, ciC24, coC24, models.StatusAbsent, "Unexcused absence"},
		{carol.ID, compA.ID, ciC25, coC25, models.StatusOnTime, ""},
		{carol.ID, compA.ID, ciC26, coC26, models.StatusOnTime, ""},
		{carol.ID, compA.ID, ciC27, coC27, models.StatusOnTime, ""},
		{carol.ID, compA.ID, ciC28, coC28, models.StatusOnTime, ""},
		// David — OT several days
		{david.ID, compB.ID, ciD4, coD4, models.StatusOnTime, "Presentation prep"},
		{david.ID, compB.ID, ciD5, coD5, models.StatusOnTime, ""},
		{david.ID, compB.ID, ciD6, coD6, models.StatusOnTime, "Quarter close"},
		{david.ID, compB.ID, ciD7, coD7, models.StatusOnTime, ""},
		{david.ID, compB.ID, ciD10, coD10, models.StatusOnTime, ""},
		{david.ID, compB.ID, ciD11, coD11, models.StatusOnTime, ""},
		{david.ID, compB.ID, ciD12, coD12, models.StatusOnTime, "Financial review"},
		{david.ID, compB.ID, ciD13, coD13, models.StatusOnTime, ""},
		{david.ID, compB.ID, ciD14, coD14, models.StatusOnTime, "Month end"},
		{david.ID, compB.ID, ciD17, coD17, models.StatusOnTime, ""},
		{david.ID, compB.ID, ciD18, coD18, models.StatusOnTime, "Strategy meeting"},
		{david.ID, compB.ID, ciD19, coD19, models.StatusOnTime, ""},
		{david.ID, compB.ID, ciD20, coD20, models.StatusOnTime, "Audit prep"},
		{david.ID, compB.ID, ciD21, coD21, models.StatusOnTime, ""},
		{david.ID, compB.ID, ciD24, coD24, models.StatusOnTime, ""},
		{david.ID, compB.ID, ciD25, coD25, models.StatusOnTime, ""},
		{david.ID, compB.ID, ciD26, coD26, models.StatusOnTime, "Close books"},
		{david.ID, compB.ID, ciD27, coD27, models.StatusOnTime, ""},
		{david.ID, compB.ID, ciD28, coD28, models.StatusOnTime, ""},
		// Eve — clean, on time
		{eve.ID, compB.ID, ciE4, coE4, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE5, coE5, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE6, coE6, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE7, coE7, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE10, coE10, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE11, coE11, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE12, coE12, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE13, coE13, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE14, coE14, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE17, coE17, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE18, coE18, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE19, coE19, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE20, coE20, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE21, coE21, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE24, coE24, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE25, coE25, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE26, coE26, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE27, coE27, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE28, coE28, models.StatusOnTime, ""},
		// Frank — clean, on time
		{frank.ID, compB.ID, ciFk4, coFk4, models.StatusOnTime, ""},
		{frank.ID, compB.ID, ciFk5, coFk5, models.StatusOnTime, ""},
		{frank.ID, compB.ID, ciFk6, coFk6, models.StatusOnTime, ""},
		{frank.ID, compB.ID, ciFk7, coFk7, models.StatusOnTime, ""},
		{frank.ID, compB.ID, ciFk10, coFk10, models.StatusOnTime, ""},
		{frank.ID, compB.ID, ciFk11, coFk11, models.StatusOnTime, ""},
		{frank.ID, compB.ID, ciFk12, coFk12, models.StatusOnTime, ""},
		{frank.ID, compB.ID, ciFk13, coFk13, models.StatusOnTime, ""},
		{frank.ID, compB.ID, ciFk14, coFk14, models.StatusOnTime, ""},
		{frank.ID, compB.ID, ciFk17, coFk17, models.StatusOnTime, ""},
		{frank.ID, compB.ID, ciFk18, coFk18, models.StatusOnTime, ""},
		{frank.ID, compB.ID, ciFk19, coFk19, models.StatusOnTime, ""},
		{frank.ID, compB.ID, ciFk20, coFk20, models.StatusOnTime, ""},
		{frank.ID, compB.ID, ciFk21, coFk21, models.StatusOnTime, ""},
		{frank.ID, compB.ID, ciFk24, coFk24, models.StatusOnTime, ""},
		{frank.ID, compB.ID, ciFk25, coFk25, models.StatusOnTime, ""},
		{frank.ID, compB.ID, ciFk26, coFk26, models.StatusOnTime, ""},
		{frank.ID, compB.ID, ciFk27, coFk27, models.StatusOnTime, ""},
		{frank.ID, compB.ID, ciFk28, coFk28, models.StatusOnTime, ""},
		// staff@admin.com — clean, on time, no OT
		{staff.ID, sysComp.ID, ciS4, coS4, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS5, coS5, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS6, coS6, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS7, coS7, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS10, coS10, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS11, coS11, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS12, coS12, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS13, coS13, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS14, coS14, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS17, coS17, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS18, coS18, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS19, coS19, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS20, coS20, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS21, coS21, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS24, coS24, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS25, coS25, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS26, coS26, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS27, coS27, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS28, coS28, models.StatusOnTime, ""},
		// Extra weekend work — Frank only
		{frank.ID, compB.ID, ciFkFebExtra1, coFkFebExtra1, models.StatusHoliday, "Holiday work"},
		{frank.ID, compB.ID, ciFkFebExtra2, coFkFebExtra2, models.StatusHoliday, "Holiday work"},
		{frank.ID, compB.ID, ciFkFebExtra3, coFkFebExtra3, models.StatusHoliday, "Holiday work"},
	}

	for _, tl := range timelogs {
		if tl.empID == 0 {
			continue // skip if employee wasn't seeded
		}
		log := models.TimeLog{
			CompaniesID: tl.compID,
			EmployeesID: tl.empID,
			CheckIn:     tl.checkIn,
			CheckOut:    tl.checkOut,
			Status:      models.StatusOnTime,
			Remark:      tl.remark,
		}
		if tl.checkIn == nil || tl.checkOut == nil {
			log.Status = models.StatusAbsent
		} else {
			day := tl.checkIn.Format("2006-01-02")
			var holidayCount int64
			config.DB.Model(&models.Holiday{}).Where("companies_id = ? AND date = ?", tl.compID, day).Count(&holidayCount)
			var dayOffCount int64
			config.DB.Model(&models.DayOff{}).Where("companies_id = ? AND weekday = ?", tl.compID, int(tl.checkIn.Weekday())).Count(&dayOffCount)
			if holidayCount > 0 || dayOffCount > 0 {
				log.Status = models.StatusHoliday
			} else {
				// Use status from tlSeed if it's late, absent, or leave
				if tl.legacy == models.StatusLate || tl.legacy == models.StatusAbsent || tl.legacy == models.StatusLeave {
					log.Status = tl.legacy
				}
			}
		}
		// Use check_in as unique key to avoid duplicate rows on re-run
		config.DB.Where(models.TimeLog{EmployeesID: tl.empID, CheckIn: tl.checkIn}).
			Assign(log).FirstOrCreate(&log)
	}

	// --- Employee Earning Assignments ---
	// Fetch earning types by name + company
	var etPosA, etPosB models.EarningType
	config.DB.Where("companies_id = ? AND name = ?", compA.ID, "Position Allowance").First(&etPosA)
	config.DB.Where("companies_id = ? AND name = ?", compB.ID, "Position Allowance").First(&etPosB)

	type eeSeed struct {
		emp           models.Employees
		earningTypeID uint
		amount        float64
		companiesID   uint
	}
	eeSeeds := []eeSeed{
		{alice, etPosA.ID, 5000, compA.ID}, // Alice: Position Allowance ฿5,000
		{david, etPosB.ID, 6000, compB.ID}, // David: Position Allowance ฿6,000
	}
	for _, s := range eeSeeds {
		if s.emp.ID == 0 || s.earningTypeID == 0 {
			continue
		}
		ee := models.EmployeeEarning{
			CompaniesID: s.companiesID, EmployeesID: s.emp.ID,
			EarningTypeID: s.earningTypeID, Amount: s.amount,
		}
		config.DB.Where(models.EmployeeEarning{EmployeesID: s.emp.ID, EarningTypeID: s.earningTypeID}).
			Assign(ee).FirstOrCreate(&ee)
	}

	// --- Employee Deduction Assignments ---
	var dtPfA, dtPfB, dtPfSys, dtLoanA, dtLoanB models.DeductionType
	config.DB.Where("companies_id = ? AND name = ?", compA.ID, "Provident Fund").First(&dtPfA)
	config.DB.Where("companies_id = ? AND name = ?", compB.ID, "Provident Fund").First(&dtPfB)
	config.DB.Where("companies_id = ? AND name = ?", sysComp.ID, "Provident Fund").First(&dtPfSys)
	config.DB.Where("companies_id = ? AND name = ?", compA.ID, "Loan Repayment").First(&dtLoanA)
	config.DB.Where("companies_id = ? AND name = ?", compB.ID, "Loan Repayment").First(&dtLoanB)

	type edSeed struct {
		emp             models.Employees
		deductionTypeID uint
		amount          float64
		companiesID     uint
	}
	edSeeds := []edSeed{
		{alice, dtPfA.ID, 1800, compA.ID},     // Alice: Provident Fund ฿1,800
		{bob, dtPfA.ID, 1200, compA.ID},       // Bob: Provident Fund ฿1,200
		{bob, dtLoanA.ID, 3000, compA.ID},     // Bob: Loan Repayment ฿3,000
		{carol, dtPfA.ID, 1120, compA.ID},     // Carol: Provident Fund ฿1,120
		{david, dtPfB.ID, 2000, compB.ID},     // David: Provident Fund ฿2,000
		{eve, dtPfB.ID, 1280, compB.ID},       // Eve: Provident Fund ฿1,280
		{eve, dtLoanB.ID, 2000, compB.ID},     // Eve: Loan Repayment ฿2,000
		{frank, dtPfB.ID, 1400, compB.ID},     // Frank: Provident Fund ฿1,400
		{staff, dtPfSys.ID, 1600, sysComp.ID}, // staff@admin.com: Provident Fund ฿1,600
	}
	for _, s := range edSeeds {
		if s.emp.ID == 0 || s.deductionTypeID == 0 {
			continue
		}
		ed := models.EmployeeDeduction{
			CompaniesID: s.companiesID, EmployeesID: s.emp.ID,
			DeductionTypeID: s.deductionTypeID, Amount: s.amount,
		}
		config.DB.Where(models.EmployeeDeduction{EmployeesID: s.emp.ID, DeductionTypeID: s.deductionTypeID}).
			Assign(ed).FirstOrCreate(&ed)
	}

	// --- Payroll Cycles: Jan / Feb / Mar 2026 for all companies ---
	type cycleSeed struct {
		cycle       models.PayrollCycle
		companiesID uint
	}
	cycleSeeds := []cycleSeed{
		// System company
		{models.PayrollCycle{CompaniesID: sysComp.ID, CycleName: "January 2026", StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC), IsClosed: true}, sysComp.ID},
		{models.PayrollCycle{CompaniesID: sysComp.ID, CycleName: "February 2026", StartDate: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 2, 28, 23, 59, 59, 0, time.UTC), IsClosed: true}, sysComp.ID},
		{models.PayrollCycle{CompaniesID: sysComp.ID, CycleName: "March 2026", StartDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 3, 31, 23, 59, 59, 0, time.UTC), IsClosed: false}, sysComp.ID},
		{models.PayrollCycle{CompaniesID: sysComp.ID, CycleName: "April 2026", StartDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 4, 30, 23, 59, 59, 0, time.UTC), IsClosed: false}, sysComp.ID},
		// Tech Corp (compA)
		{models.PayrollCycle{CompaniesID: compA.ID, CycleName: "January 2026", StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC), IsClosed: true}, compA.ID},
		{models.PayrollCycle{CompaniesID: compA.ID, CycleName: "February 2026", StartDate: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 2, 28, 23, 59, 59, 0, time.UTC), IsClosed: true}, compA.ID},
		{models.PayrollCycle{CompaniesID: compA.ID, CycleName: "March 2026", StartDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 3, 31, 23, 59, 59, 0, time.UTC), IsClosed: false}, compA.ID},
		{models.PayrollCycle{CompaniesID: compA.ID, CycleName: "April 2026", StartDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 4, 30, 23, 59, 59, 0, time.UTC), IsClosed: false}, compA.ID},
		// Biz Group (compB)
		{models.PayrollCycle{CompaniesID: compB.ID, CycleName: "January 2026", StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC), IsClosed: true}, compB.ID},
		{models.PayrollCycle{CompaniesID: compB.ID, CycleName: "February 2026", StartDate: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 2, 28, 23, 59, 59, 0, time.UTC), IsClosed: true}, compB.ID},
		{models.PayrollCycle{CompaniesID: compB.ID, CycleName: "March 2026", StartDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 3, 31, 23, 59, 59, 0, time.UTC), IsClosed: false}, compB.ID},
		{models.PayrollCycle{CompaniesID: compB.ID, CycleName: "April 2026", StartDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 4, 30, 23, 59, 59, 0, time.UTC), IsClosed: false}, compB.ID},
	}
	for _, s := range cycleSeeds {
		config.DB.Where(models.PayrollCycle{CompaniesID: s.companiesID, CycleName: s.cycle.CycleName}).
			Assign(s.cycle).FirstOrCreate(&s.cycle)
	}

	// Re-fetch cycles needed for cycle earnings
	var cycleAJan, cycleAFeb, cycleBJan, cycleBFeb models.PayrollCycle
	config.DB.Where("companies_id = ? AND cycle_name = ?", compA.ID, "January 2026").First(&cycleAJan)
	config.DB.Where("companies_id = ? AND cycle_name = ?", compA.ID, "February 2026").First(&cycleAFeb)
	config.DB.Where("companies_id = ? AND cycle_name = ?", compB.ID, "January 2026").First(&cycleBJan)
	config.DB.Where("companies_id = ? AND cycle_name = ?", compB.ID, "February 2026").First(&cycleBFeb)

	// Re-fetch variable earning types
	var etBonusA, etBonusB models.EarningType
	config.DB.Where("companies_id = ? AND name = ?", compA.ID, "Bonus").First(&etBonusA)
	config.DB.Where("companies_id = ? AND name = ?", compB.ID, "Bonus").First(&etBonusB)

	// --- PayrollCycleEarning: one-time bonuses in Jan & Feb 2026 ---
	type ceSeed struct {
		cycleID       uint
		companiesID   uint
		emp           models.Employees
		earningTypeID uint
		amount        float64
	}
	ceSeeds := []ceSeed{
		// Tech Corp — Jan 2026 bonuses
		{cycleAJan.ID, compA.ID, alice, etBonusA.ID, 10000}, // Alice: ฿10,000 year-start bonus
		{cycleAJan.ID, compA.ID, bob, etBonusA.ID, 5000},    // Bob: ฿5,000
		{cycleAJan.ID, compA.ID, carol, etBonusA.ID, 5000},  // Carol: ฿5,000
		// Tech Corp — Feb 2026 bonuses
		{cycleAFeb.ID, compA.ID, alice, etBonusA.ID, 8000}, // Alice: performance bonus
		// Biz Group — Jan 2026 bonuses
		{cycleBJan.ID, compB.ID, david, etBonusB.ID, 12000}, // David: ฿12,000 year-start bonus
		{cycleBJan.ID, compB.ID, eve, etBonusB.ID, 6000},    // Eve: ฿6,000
		// Biz Group — Feb 2026 bonuses
		{cycleBFeb.ID, compB.ID, david, etBonusB.ID, 7000}, // David: performance bonus
		{cycleBFeb.ID, compB.ID, eve, etBonusB.ID, 3500},   // Eve: performance bonus
	}
	for _, s := range ceSeeds {
		if s.cycleID == 0 || s.emp.ID == 0 || s.earningTypeID == 0 {
			continue
		}
		ce := models.PayrollCycleEarning{
			CompaniesID:   s.companiesID,
			CycleID:       s.cycleID,
			EmployeesID:   s.emp.ID,
			EarningTypeID: s.earningTypeID,
			Amount:        s.amount,
		}
		config.DB.Where(models.PayrollCycleEarning{CycleID: s.cycleID, EmployeesID: s.emp.ID, EarningTypeID: s.earningTypeID}).
			Assign(ce).FirstOrCreate(&ce)
	}

	// Additional Feb 2026 cycle variable earnings (sysComp staff + missing compA/compB employees)
	var cycleSysFeb0 models.PayrollCycle
	config.DB.Where("companies_id = ? AND cycle_name = ?", sysComp.ID, "February 2026").First(&cycleSysFeb0)

	var etOtSys, etOtA, etOtB models.EarningType
	config.DB.Where("companies_id = ? AND name = ?", sysComp.ID, "Overtime (OT)").First(&etOtSys)
	config.DB.Where("companies_id = ? AND name = ?", compA.ID, "Overtime (OT)").First(&etOtA)
	config.DB.Where("companies_id = ? AND name = ?", compB.ID, "Overtime (OT)").First(&etOtB)

	ceSeeds2 := []ceSeed{
		// System Company — Feb 2026 overtime
		{cycleSysFeb0.ID, sysComp.ID, staff, etOtSys.ID, 3200},
		// Tech Corp — Feb 2026 overtime (bob & carol)
		{cycleAFeb.ID, compA.ID, bob, etOtA.ID, 4500},
		{cycleAFeb.ID, compA.ID, carol, etOtA.ID, 2500},
		// Biz Group — Feb 2026 bonus (frank)
		{cycleBFeb.ID, compB.ID, frank, etBonusB.ID, 5000},
	}
	for _, s := range ceSeeds2 {
		if s.cycleID == 0 || s.emp.ID == 0 || s.earningTypeID == 0 {
			continue
		}
		ce := models.PayrollCycleEarning{
			CompaniesID:   s.companiesID,
			CycleID:       s.cycleID,
			EmployeesID:   s.emp.ID,
			EarningTypeID: s.earningTypeID,
			Amount:        s.amount,
		}
		config.DB.Where(models.PayrollCycleEarning{CycleID: s.cycleID, EmployeesID: s.emp.ID, EarningTypeID: s.earningTypeID}).
			Assign(ce).FirstOrCreate(&ce)
	}

	// --- PayrollCycleDeduction: one-time variable deductions for Feb 2026 (all companies) ---
	var dtLoanSys0, dtLoanA0, dtLoanB0 models.DeductionType
	config.DB.Where("companies_id = ? AND name = ?", sysComp.ID, "Loan Repayment").First(&dtLoanSys0)
	config.DB.Where("companies_id = ? AND name = ?", compA.ID, "Loan Repayment").First(&dtLoanA0)
	config.DB.Where("companies_id = ? AND name = ?", compB.ID, "Loan Repayment").First(&dtLoanB0)

	type cdSeed struct {
		cycleID         uint
		companiesID     uint
		emp             models.Employees
		deductionTypeID uint
		amount          float64
	}
	cdSeeds := []cdSeed{
		// System Company — Feb 2026
		{cycleSysFeb0.ID, sysComp.ID, staff, dtLoanSys0.ID, 2000},
		// Tech Corp — Feb 2026
		{cycleAFeb.ID, compA.ID, alice, dtLoanA0.ID, 5000},
		{cycleAFeb.ID, compA.ID, bob, dtLoanA0.ID, 2000},
		{cycleAFeb.ID, compA.ID, carol, dtLoanA0.ID, 1500},
		// Biz Group — Feb 2026
		{cycleBFeb.ID, compB.ID, david, dtLoanB0.ID, 3000},
		{cycleBFeb.ID, compB.ID, eve, dtLoanB0.ID, 1500},
		{cycleBFeb.ID, compB.ID, frank, dtLoanB0.ID, 2500},
	}
	for _, s := range cdSeeds {
		if s.cycleID == 0 || s.emp.ID == 0 || s.deductionTypeID == 0 {
			continue
		}
		cd := models.PayrollCycleDeduction{
			CompaniesID:     s.companiesID,
			CycleID:         s.cycleID,
			EmployeesID:     s.emp.ID,
			DeductionTypeID: s.deductionTypeID,
			Amount:          s.amount,
		}
		config.DB.Where(models.PayrollCycleDeduction{CycleID: s.cycleID, EmployeesID: s.emp.ID, DeductionTypeID: s.deductionTypeID}).
			Assign(cd).FirstOrCreate(&cd)
	}

	// Mock Jan 2026 payroll data for testing

	// Re-fetch Jan & Feb 2026 cycles for system company
	var cycleSysJan, cycleSysFeb models.PayrollCycle
	config.DB.Where("companies_id = ? AND cycle_name = ?", sysComp.ID, "January 2026").First(&cycleSysJan)
	config.DB.Where("companies_id = ? AND cycle_name = ?", sysComp.ID, "February 2026").First(&cycleSysFeb)

	// Re-fetch earning / deduction type IDs for detail cross-references
	var etPosA2, etPosB2, etBonusA2, etBonusB2 models.EarningType
	var dtPfA2, dtPfB2, dtPfSys2, dtLoanA2, dtLoanB2 models.DeductionType
	config.DB.Where("companies_id = ? AND name = ?", compA.ID, "Position Allowance").First(&etPosA2)
	config.DB.Where("companies_id = ? AND name = ?", compB.ID, "Position Allowance").First(&etPosB2)
	config.DB.Where("companies_id = ? AND name = ?", compA.ID, "Bonus").First(&etBonusA2)
	config.DB.Where("companies_id = ? AND name = ?", compB.ID, "Bonus").First(&etBonusB2)
	config.DB.Where("companies_id = ? AND name = ?", compA.ID, "Provident Fund").First(&dtPfA2)
	config.DB.Where("companies_id = ? AND name = ?", compB.ID, "Provident Fund").First(&dtPfB2)
	config.DB.Where("companies_id = ? AND name = ?", sysComp.ID, "Provident Fund").First(&dtPfSys2)
	config.DB.Where("companies_id = ? AND name = ?", compA.ID, "Loan Repayment").First(&dtLoanA2)
	config.DB.Where("companies_id = ? AND name = ?", compB.ID, "Loan Repayment").First(&dtLoanB2)

	type txSeed struct {
		cycleID        uint
		emp            models.Employees
		salaries       float64
		totalEarning   float64
		totalDeduction float64 // non-SSO, non-Tax deductions only
		ssoAmount      float64
		taxAmount      float64
		netPay         float64
		details        []models.PayrollTransactionDetail
	}

	uid := func(id uint) *uint { return &id }

	mockTxSeeds := []txSeed{
		// ── Tech Corp — January 2026 ─────────────────────────────────────
		{
			cycleID: cycleAJan.ID, emp: alice,
			salaries: 45000, totalEarning: 60000, totalDeduction: 1800,
			ssoAmount: 750, taxAmount: 2741.67, netPay: 54708.33,
			details: []models.PayrollTransactionDetail{
				{ItemName: "Position Allowance", Amount: 5000, Type: "earning", EarningTypeID: uid(etPosA2.ID)},
				{ItemName: "Bonus", Amount: 10000, Type: "earning", EarningTypeID: uid(etBonusA2.ID)},
				{ItemName: "Provident Fund", Amount: 1800, Type: "deduction", DeductionTypeID: uid(dtPfA2.ID)},
				{ItemName: "Social Security (SSO)", Amount: 750, Type: "deduction"},
				{ItemName: "Income Tax", Amount: 2741.67, Type: "deduction"},
			},
		},
		{
			cycleID: cycleAJan.ID, emp: bob,
			salaries: 30000, totalEarning: 35000, totalDeduction: 4200,
			ssoAmount: 750, taxAmount: 398.33, netPay: 29651.67,
			details: []models.PayrollTransactionDetail{
				{ItemName: "Bonus", Amount: 5000, Type: "earning", EarningTypeID: uid(etBonusA2.ID)},
				{ItemName: "Provident Fund", Amount: 1200, Type: "deduction", DeductionTypeID: uid(dtPfA2.ID)},
				{ItemName: "Loan Repayment", Amount: 3000, Type: "deduction", DeductionTypeID: uid(dtLoanA2.ID)},
				{ItemName: "Social Security (SSO)", Amount: 750, Type: "deduction"},
				{ItemName: "Income Tax", Amount: 398.33, Type: "deduction"},
			},
		},
		{
			cycleID: cycleAJan.ID, emp: carol,
			salaries: 28000, totalEarning: 33000, totalDeduction: 1120,
			ssoAmount: 750, taxAmount: 302.33, netPay: 30827.67,
			details: []models.PayrollTransactionDetail{
				{ItemName: "Bonus", Amount: 5000, Type: "earning", EarningTypeID: uid(etBonusA2.ID)},
				{ItemName: "Provident Fund", Amount: 1120, Type: "deduction", DeductionTypeID: uid(dtPfA2.ID)},
				{ItemName: "Social Security (SSO)", Amount: 750, Type: "deduction"},
				{ItemName: "Income Tax", Amount: 302.33, Type: "deduction"},
			},
		},
		// ── Biz Group — January 2026 ─────────────────────────────────────
		{
			cycleID: cycleBJan.ID, emp: david,
			salaries: 50000, totalEarning: 68000, totalDeduction: 2000,
			ssoAmount: 750, taxAmount: 3941.67, netPay: 61308.33,
			details: []models.PayrollTransactionDetail{
				{ItemName: "Position Allowance", Amount: 6000, Type: "earning", EarningTypeID: uid(etPosB2.ID)},
				{ItemName: "Bonus", Amount: 12000, Type: "earning", EarningTypeID: uid(etBonusB2.ID)},
				{ItemName: "Provident Fund", Amount: 2000, Type: "deduction", DeductionTypeID: uid(dtPfB2.ID)},
				{ItemName: "Social Security (SSO)", Amount: 750, Type: "deduction"},
				{ItemName: "Income Tax", Amount: 3941.67, Type: "deduction"},
			},
		},
		{
			cycleID: cycleBJan.ID, emp: eve,
			salaries: 32000, totalEarning: 38000, totalDeduction: 3280,
			ssoAmount: 750, taxAmount: 544.33, netPay: 33425.67,
			details: []models.PayrollTransactionDetail{
				{ItemName: "Bonus", Amount: 6000, Type: "earning", EarningTypeID: uid(etBonusB2.ID)},
				{ItemName: "Provident Fund", Amount: 1280, Type: "deduction", DeductionTypeID: uid(dtPfB2.ID)},
				{ItemName: "Loan Repayment", Amount: 2000, Type: "deduction", DeductionTypeID: uid(dtLoanB2.ID)},
				{ItemName: "Social Security (SSO)", Amount: 750, Type: "deduction"},
				{ItemName: "Income Tax", Amount: 544.33, Type: "deduction"},
			},
		},
		// ── System Company — January 2026 ────────────────────────────────
		{
			cycleID: cycleSysJan.ID, emp: staff,
			salaries: 40000, totalEarning: 40000, totalDeduction: 1600,
			ssoAmount: 750, taxAmount: 631.67, netPay: 37018.33,
			details: []models.PayrollTransactionDetail{
				{ItemName: "Provident Fund", Amount: 1600, Type: "deduction", DeductionTypeID: uid(dtPfSys2.ID)},
				{ItemName: "Social Security (SSO)", Amount: 750, Type: "deduction"},
				{ItemName: "Income Tax", Amount: 631.67, Type: "deduction"},
			},
		},
		// ── Tech Corp — February 2026 ─────────────────────────────────────
		{
			cycleID: cycleAFeb.ID, emp: alice,
			salaries: 45000, totalEarning: 58000, totalDeduction: 1800,
			ssoAmount: 750, taxAmount: 2471.67, netPay: 52978.33,
			details: []models.PayrollTransactionDetail{
				{ItemName: "Position Allowance", Amount: 5000, Type: "earning", EarningTypeID: uid(etPosA2.ID)},
				{ItemName: "Bonus", Amount: 8000, Type: "earning", EarningTypeID: uid(etBonusA2.ID)},
				{ItemName: "Provident Fund", Amount: 1800, Type: "deduction", DeductionTypeID: uid(dtPfA2.ID)},
				{ItemName: "Social Security (SSO)", Amount: 750, Type: "deduction"},
				{ItemName: "Income Tax", Amount: 2471.67, Type: "deduction"},
			},
		},
		{
			cycleID: cycleAFeb.ID, emp: bob,
			salaries: 30000, totalEarning: 30000, totalDeduction: 4200,
			ssoAmount: 750, taxAmount: 0, netPay: 25050,
			details: []models.PayrollTransactionDetail{
				{ItemName: "Provident Fund", Amount: 1200, Type: "deduction", DeductionTypeID: uid(dtPfA2.ID)},
				{ItemName: "Loan Repayment", Amount: 3000, Type: "deduction", DeductionTypeID: uid(dtLoanA2.ID)},
				{ItemName: "Social Security (SSO)", Amount: 750, Type: "deduction"},
			},
		},
		{
			cycleID: cycleAFeb.ID, emp: carol,
			salaries: 28000, totalEarning: 28000, totalDeduction: 1120,
			ssoAmount: 750, taxAmount: 52.33, netPay: 26077.67,
			details: []models.PayrollTransactionDetail{
				{ItemName: "Provident Fund", Amount: 1120, Type: "deduction", DeductionTypeID: uid(dtPfA2.ID)},
				{ItemName: "Social Security (SSO)", Amount: 750, Type: "deduction"},
				{ItemName: "Income Tax", Amount: 52.33, Type: "deduction"},
			},
		},
		// ── Biz Group — February 2026 ─────────────────────────────────────
		{
			cycleID: cycleBFeb.ID, emp: david,
			salaries: 50000, totalEarning: 63000, totalDeduction: 2000,
			ssoAmount: 750, taxAmount: 3191.67, netPay: 57058.33,
			details: []models.PayrollTransactionDetail{
				{ItemName: "Position Allowance", Amount: 6000, Type: "earning", EarningTypeID: uid(etPosB2.ID)},
				{ItemName: "Bonus", Amount: 7000, Type: "earning", EarningTypeID: uid(etBonusB2.ID)},
				{ItemName: "Provident Fund", Amount: 2000, Type: "deduction", DeductionTypeID: uid(dtPfB2.ID)},
				{ItemName: "Social Security (SSO)", Amount: 750, Type: "deduction"},
				{ItemName: "Income Tax", Amount: 3191.67, Type: "deduction"},
			},
		},
		{
			cycleID: cycleBFeb.ID, emp: eve,
			salaries: 32000, totalEarning: 35500, totalDeduction: 3280,
			ssoAmount: 750, taxAmount: 319.33, netPay: 31150.67,
			details: []models.PayrollTransactionDetail{
				{ItemName: "Bonus", Amount: 3500, Type: "earning", EarningTypeID: uid(etBonusB2.ID)},
				{ItemName: "Provident Fund", Amount: 1280, Type: "deduction", DeductionTypeID: uid(dtPfB2.ID)},
				{ItemName: "Loan Repayment", Amount: 2000, Type: "deduction", DeductionTypeID: uid(dtLoanB2.ID)},
				{ItemName: "Social Security (SSO)", Amount: 750, Type: "deduction"},
				{ItemName: "Income Tax", Amount: 319.33, Type: "deduction"},
			},
		},
		// ── System Company — February 2026 ───────────────────────────────
		{
			cycleID: cycleSysFeb.ID, emp: staff,
			salaries: 40000, totalEarning: 40000, totalDeduction: 1600,
			ssoAmount: 750, taxAmount: 631.67, netPay: 37018.33,
			details: []models.PayrollTransactionDetail{
				{ItemName: "Provident Fund", Amount: 1600, Type: "deduction", DeductionTypeID: uid(dtPfSys2.ID)},
				{ItemName: "Social Security (SSO)", Amount: 750, Type: "deduction"},
				{ItemName: "Income Tax", Amount: 631.67, Type: "deduction"},
			},
		},
	}

	for _, s := range mockTxSeeds {
		if s.cycleID == 0 || s.emp.ID == 0 {
			continue
		}
		var tx models.PayrollTransaction
		result := config.DB.Where("cycle_id = ? AND employees_id = ?", s.cycleID, s.emp.ID).Limit(1).Find(&tx)
		if result.Error != nil {
			log.Printf("Failed to query mock transaction for emp %d: %v", s.emp.ID, result.Error)
			continue
		}
		if result.RowsAffected == 0 {
			// Create the transaction
			tx = models.PayrollTransaction{
				CycleID:        s.cycleID,
				EmployeesID:    s.emp.ID,
				SalariesAtTime: s.salaries,
				TotalEarning:   s.totalEarning,
				TotalDeduction: s.totalDeduction,
				SsoAmount:      s.ssoAmount,
				TaxAmount:      s.taxAmount,
				NetPay:         s.netPay,
			}
			if err := config.DB.Create(&tx).Error; err != nil {
				log.Printf("Failed to create mock transaction for emp %d: %v", s.emp.ID, err)
				continue
			}
		}
		// Seed details only when none exist yet
		var detailCount int64
		config.DB.Model(&models.PayrollTransactionDetail{}).Where("transaction_id = ?", tx.ID).Count(&detailCount)
		if detailCount == 0 {
			for i := range s.details {
				s.details[i].TransactionID = tx.ID
			}
			config.DB.Create(&s.details)
		}
	}

	// --- Realistic 2027 Mock Baseline ---
	// Passwords intentionally match emails for local demo/dev convenience only.
	devUsers := []struct {
		emp     *models.Employees
		email   string
		company uint
		role    uint
		salary  float64
		bankID  *uint
		bank    string
		account string
	}{
		{&admin, "admin@admin.com", sysComp.ID, sysRole.ID, 0, &bankTTB.ID, bankTTB.BankName, "Payroll Pro Admin"},
		{&staff, "staff@admin.com", sysComp.ID, sysStaffRole.ID, 42000, &bankTTB.ID, bankTTB.BankName, "Test Staff"},
		{&alice, "alice@techa.com", compA.ID, managerRoleA.ID, 78000, &bankKBANK.ID, bankKBANK.BankName, "Alice Johnson"},
		{&bob, "bob@techa.com", compA.ID, staffRoleA.ID, 52000, &bankSCB.ID, bankSCB.BankName, "Bob Smith"},
		{&carol, "carol@techa.com", compA.ID, staffRoleA.ID, 32500, &bankBBL.ID, bankBBL.BankName, "Carol Davis"},
		{&david, "david@bizgroup.com", compB.ID, managerRoleB.ID, 86000, &bankKTB.ID, bankKTB.BankName, "David Wilson"},
		{&eve, "eve@bizgroup.com", compB.ID, staffRoleB.ID, 47000, &bankBAY.ID, bankBAY.BankName, "Eve Brown"},
		{&frank, "frank@bizgroup.com", compB.ID, staffRoleB.ID, 29500, &bankBBL.ID, bankBBL.BankName, "Frank Garcia"},
	}

	for idx, u := range devUsers {
		hashed, _ := bcrypt.GenerateFromPassword([]byte(u.email), 10)
		config.DB.Model(&models.Employees{}).Where("email = ?", u.email).Updates(map[string]interface{}{
			"password":     string(hashed),
			"is_active":    true,
			"roles_id":     u.role,
			"companies_id": u.company,
			"bank_id":      u.bankID,
			"bank_name":    u.bank,
			"account_name": u.account,
			"account_no":   "2027" + time.Date(2027, time.January, idx+1, 0, 0, 0, 0, time.UTC).Format("0102") + "000",
		})
		config.DB.Unscoped().Where("email = ?", u.email).First(u.emp)
		if u.salary > 0 && u.emp.ID != 0 {
			salary := models.SalariesStructure{
				CompaniesID:   u.company,
				EmployeesID:   u.emp.ID,
				BaseSalaries:  u.salary,
				EffectiveDate: time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
			}
			config.DB.Where(models.SalariesStructure{EmployeesID: u.emp.ID, EffectiveDate: salary.EffectiveDate}).
				Assign(salary).FirstOrCreate(&salary)
			upsertEmployeeCompanyAccess(u.emp.ID, u.company, u.role, true)
		}
	}

	companies := []models.Companies{sysComp, compA, compB}
	companyIDs := []uint{sysComp.ID, compA.ID, compB.ID}
	start2027 := time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)
	end2027 := time.Date(2027, time.June, 30, 23, 59, 59, 0, time.UTC)

	// Remove the older 2026 demo payroll/time-log rows from this seed so local data focuses on Jan-Jun 2027.
	config.DB.Exec(`DELETE FROM payroll_transaction_detail AS ptd
		USING payroll_transaction AS pt, payroll_cycle AS pc
		WHERE pt.id = ptd.transaction_id
			AND pc.id = pt.cycle_id
			AND pc.companies_id IN ? AND pc.start_date >= ? AND pc.start_date < ?`, companyIDs, time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), start2027)
	config.DB.Exec(`DELETE FROM payroll_transaction AS pt
		USING payroll_cycle AS pc
		WHERE pc.id = pt.cycle_id
			AND pc.companies_id IN ? AND pc.start_date >= ? AND pc.start_date < ?`, companyIDs, time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), start2027)
	config.DB.Exec(`DELETE FROM payroll_cycle_earning AS pce
		USING payroll_cycle AS pc
		WHERE pc.id = pce.cycle_id
			AND pc.companies_id IN ? AND pc.start_date >= ? AND pc.start_date < ?`, companyIDs, time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), start2027)
	config.DB.Exec(`DELETE FROM payroll_cycle_deduction AS pcd
		USING payroll_cycle AS pc
		WHERE pc.id = pcd.cycle_id
			AND pc.companies_id IN ? AND pc.start_date >= ? AND pc.start_date < ?`, companyIDs, time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), start2027)
	config.DB.Unscoped().Where("companies_id IN ? AND start_date >= ? AND start_date < ?", companyIDs, time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), start2027).Delete(&models.PayrollCycle{})
	config.DB.Unscoped().Where("companies_id IN ? AND ((check_in >= ? AND check_in <= ?) OR (created_at >= ? AND created_at <= ?))", companyIDs, time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), end2027, time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), end2027).Delete(&models.TimeLog{})

	holidaySeeds2027 := []models.Holiday{
		{Date: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), Name: "New Year's Day", IsPaid: true},
		{Date: time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC), Name: "Makha Bucha Day", IsPaid: true},
		{Date: time.Date(2027, 4, 6, 0, 0, 0, 0, time.UTC), Name: "Chakri Memorial Day", IsPaid: true},
		{Date: time.Date(2027, 4, 13, 0, 0, 0, 0, time.UTC), Name: "Songkran Festival", IsPaid: true},
		{Date: time.Date(2027, 4, 14, 0, 0, 0, 0, time.UTC), Name: "Songkran Festival", IsPaid: true},
		{Date: time.Date(2027, 4, 15, 0, 0, 0, 0, time.UTC), Name: "Songkran Festival", IsPaid: true},
		{Date: time.Date(2027, 5, 1, 0, 0, 0, 0, time.UTC), Name: "Labour Day", IsPaid: true},
		{Date: time.Date(2027, 5, 5, 0, 0, 0, 0, time.UTC), Name: "Coronation Day", IsPaid: true},
		{Date: time.Date(2027, 5, 20, 0, 0, 0, 0, time.UTC), Name: "Visakha Bucha Day", IsPaid: true},
		{Date: time.Date(2027, 6, 3, 0, 0, 0, 0, time.UTC), Name: "Queen Suthida's Birthday", IsPaid: true},
	}
	for _, comp := range companies {
		for _, h := range holidaySeeds2027 {
			h.CompaniesID = comp.ID
			config.DB.Where("companies_id = ? AND date = ?", comp.ID, h.Date.Format("2006-01-02")).Assign(h).FirstOrCreate(&h)
		}
	}

	monthName := func(m time.Month) string {
		return [...]string{"", "January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}[m]
	}
	monthEnd := func(year int, month time.Month) time.Time {
		return time.Date(year, month+1, 0, 23, 59, 59, 0, time.UTC)
	}
	cycleByCompanyMonth := map[uint]map[time.Month]models.PayrollCycle{}
	for _, comp := range companies {
		cycleByCompanyMonth[comp.ID] = map[time.Month]models.PayrollCycle{}
		for m := time.January; m <= time.June; m++ {
			cycle := models.PayrollCycle{
				CompaniesID: comp.ID,
				CycleName:   monthName(m) + " 2027",
				StartDate:   time.Date(2027, m, 1, 0, 0, 0, 0, time.UTC),
				EndDate:     monthEnd(2027, m),
				IsClosed:    m <= time.May,
			}
			config.DB.Where(models.PayrollCycle{CompaniesID: comp.ID, CycleName: cycle.CycleName}).Assign(cycle).FirstOrCreate(&cycle)
			cycleByCompanyMonth[comp.ID][m] = cycle
		}
	}

	type profile struct {
		emp          models.Employees
		companyID    uint
		salary       float64
		allowance    float64
		provident    float64
		loan         float64
		pattern      string
		approvedByID *uint
	}
	approverA, approverB, approverSys := alice.ID, david.ID, admin.ID
	profiles := []profile{
		{staff, sysComp.ID, 42000, 1500, 1680, 0, "steady", &approverSys},
		{alice, compA.ID, 78000, 8000, 3900, 0, "manager", &approverA},
		{bob, compA.ID, 52000, 2500, 2600, 3000, "late_ot", &approverA},
		{carol, compA.ID, 32500, 1200, 1300, 0, "mixed_absent", &approverA},
		{david, compB.ID, 86000, 9000, 4300, 0, "manager", &approverB},
		{eve, compB.ID, 47000, 1800, 2350, 1800, "steady_ot", &approverB},
		{frank, compB.ID, 29500, 900, 1180, 2200, "mixed_absent", &approverB},
	}

	type leavePlan struct {
		emp       models.Employees
		companyID uint
		leaveType models.LeaveType
		name      string
		start     time.Time
		end       time.Time
		days      float64
		status    models.LeaveStatus
		reason    string
		approver  *uint
	}
	leaveTypeID := func(companyID uint, name string) *uint {
		var lt models.LeaveTypeSetting
		if err := config.DB.Where("companies_id = ? AND name = ?", companyID, name).First(&lt).Error; err != nil {
			return nil
		}
		return &lt.ID
	}
	leavePlans := []leavePlan{
		{bob, compA.ID, models.LeaveTypeAnnual, "Annual Leave", time.Date(2027, 2, 18, 0, 0, 0, 0, time.UTC), time.Date(2027, 2, 19, 0, 0, 0, 0, time.UTC), 2, models.LeaveStatusApproved, "Family trip booked in advance", &approverA},
		{carol, compA.ID, models.LeaveTypeSick, "Sick Leave", time.Date(2027, 3, 9, 0, 0, 0, 0, time.UTC), time.Date(2027, 3, 10, 0, 0, 0, 0, time.UTC), 2, models.LeaveStatusApproved, "Medical certificate attached", &approverA},
		{eve, compB.ID, models.LeaveTypePersonal, "Personal Leave", time.Date(2027, 4, 9, 0, 0, 0, 0, time.UTC), time.Date(2027, 4, 9, 0, 0, 0, 0, time.UTC), 1, models.LeaveStatusApproved, "Government document appointment", &approverB},
		{frank, compB.ID, models.LeaveTypeAnnual, "Annual Leave", time.Date(2027, 5, 17, 0, 0, 0, 0, time.UTC), time.Date(2027, 5, 18, 0, 0, 0, 0, time.UTC), 2, models.LeaveStatusApproved, "Planned vacation", &approverB},
		{staff, sysComp.ID, models.LeaveTypeSick, "Sick Leave", time.Date(2027, 6, 8, 0, 0, 0, 0, time.UTC), time.Date(2027, 6, 8, 0, 0, 0, 0, time.UTC), 1, models.LeaveStatusPending, "Flu symptoms", nil},
	}
	approvedLeaveDays := map[string]bool{}
	for _, lp := range leavePlans {
		if lp.emp.ID == 0 {
			continue
		}
		leave := models.LeaveRequest{
			CompaniesID: lp.companyID, EmployeesID: lp.emp.ID, LeaveTypeID: leaveTypeID(lp.companyID, lp.name),
			LeaveType: lp.leaveType, StartDate: lp.start, EndDate: lp.end, TotalDays: lp.days,
			Reason: lp.reason, Status: lp.status, ApprovedBy: lp.approver,
		}
		config.DB.Where("employees_id = ? AND start_date = ? AND leave_type = ?", lp.emp.ID, lp.start, lp.leaveType).
			Assign(leave).FirstOrCreate(&leave)
		if lp.status == models.LeaveStatusApproved {
			for day := lp.start; !day.After(lp.end); day = day.AddDate(0, 0, 1) {
				approvedLeaveDays[lp.emp.Email+"|"+day.Format("2006-01-02")] = true
			}
		}
	}

	for _, p := range profiles {
		if p.emp.ID == 0 {
			continue
		}
		for _, lb := range []struct {
			t    models.LeaveType
			max  float64
			used float64
		}{
			{models.LeaveTypeAnnual, 15, 0},
			{models.LeaveTypeSick, 30, 0},
			{models.LeaveTypePersonal, 3, 0},
			{models.LeaveTypeUnpaid, 0, 0},
		} {
			for _, lp := range leavePlans {
				if lp.emp.ID == p.emp.ID && lp.leaveType == lb.t && lp.status == models.LeaveStatusApproved {
					lb.used += lp.days
				}
			}
			balance := models.LeaveBalance{
				CompaniesID: p.companyID, EmployeesID: p.emp.ID, Year: 2027, LeaveType: lb.t,
				MaxQuota: lb.max, UsedQuota: lb.used, Remaining: math.Max(lb.max-lb.used, 0),
			}
			config.DB.Where(models.LeaveBalance{CompaniesID: p.companyID, EmployeesID: p.emp.ID, Year: 2027, LeaveType: lb.t}).
				Assign(balance).FirstOrCreate(&balance)
		}
	}

	var etOtSys2027, etOtA2027, etOtB2027, etBonusA2027, etBonusB2027, etPosA2027, etPosB2027 models.EarningType
	var dtLoanSys2027, dtLoanA2027, dtLoanB2027, dtPfSys2027, dtPfA2027, dtPfB2027 models.DeductionType
	config.DB.Where("companies_id = ? AND name = ?", sysComp.ID, "Overtime (OT)").First(&etOtSys2027)
	config.DB.Where("companies_id = ? AND name = ?", compA.ID, "Overtime (OT)").First(&etOtA2027)
	config.DB.Where("companies_id = ? AND name = ?", compB.ID, "Overtime (OT)").First(&etOtB2027)
	config.DB.Where("companies_id = ? AND name = ?", compA.ID, "Bonus").First(&etBonusA2027)
	config.DB.Where("companies_id = ? AND name = ?", compB.ID, "Bonus").First(&etBonusB2027)
	config.DB.Where("companies_id = ? AND name = ?", compA.ID, "Position Allowance").First(&etPosA2027)
	config.DB.Where("companies_id = ? AND name = ?", compB.ID, "Position Allowance").First(&etPosB2027)
	config.DB.Where("companies_id = ? AND name = ?", sysComp.ID, "Loan Repayment").First(&dtLoanSys2027)
	config.DB.Where("companies_id = ? AND name = ?", compA.ID, "Loan Repayment").First(&dtLoanA2027)
	config.DB.Where("companies_id = ? AND name = ?", compB.ID, "Loan Repayment").First(&dtLoanB2027)
	config.DB.Where("companies_id = ? AND name = ?", sysComp.ID, "Provident Fund").First(&dtPfSys2027)
	config.DB.Where("companies_id = ? AND name = ?", compA.ID, "Provident Fund").First(&dtPfA2027)
	config.DB.Where("companies_id = ? AND name = ?", compB.ID, "Provident Fund").First(&dtPfB2027)

	earningTypeForCompany := func(companyID uint, kind string) uint {
		if companyID == sysComp.ID {
			if kind == "ot" {
				return etOtSys2027.ID
			}
			return 0
		}
		if companyID == compA.ID {
			if kind == "bonus" {
				return etBonusA2027.ID
			}
			if kind == "position" {
				return etPosA2027.ID
			}
			return etOtA2027.ID
		}
		if kind == "bonus" {
			return etBonusB2027.ID
		}
		if kind == "position" {
			return etPosB2027.ID
		}
		return etOtB2027.ID
	}
	deductionTypeForCompany := func(companyID uint, kind string) uint {
		if companyID == sysComp.ID {
			if kind == "loan" {
				return dtLoanSys2027.ID
			}
			return dtPfSys2027.ID
		}
		if companyID == compA.ID {
			if kind == "loan" {
				return dtLoanA2027.ID
			}
			return dtPfA2027.ID
		}
		if kind == "loan" {
			return dtLoanB2027.ID
		}
		return dtPfB2027.ID
	}

	for _, p := range profiles {
		if p.emp.ID == 0 {
			continue
		}
		if p.allowance > 0 {
			etID := earningTypeForCompany(p.companyID, "position")
			if etID != 0 {
				rec := models.EmployeeEarning{CompaniesID: p.companyID, EmployeesID: p.emp.ID, EarningTypeID: etID, Amount: p.allowance}
				config.DB.Where(models.EmployeeEarning{EmployeesID: p.emp.ID, EarningTypeID: etID}).Assign(rec).FirstOrCreate(&rec)
			}
		}
		if p.provident > 0 {
			dtID := deductionTypeForCompany(p.companyID, "provident")
			rec := models.EmployeeDeduction{CompaniesID: p.companyID, EmployeesID: p.emp.ID, DeductionTypeID: dtID, Amount: p.provident}
			config.DB.Where(models.EmployeeDeduction{EmployeesID: p.emp.ID, DeductionTypeID: dtID}).Assign(rec).FirstOrCreate(&rec)
		}
		if p.loan > 0 {
			dtID := deductionTypeForCompany(p.companyID, "loan")
			rec := models.EmployeeDeduction{CompaniesID: p.companyID, EmployeesID: p.emp.ID, DeductionTypeID: dtID, Amount: p.loan}
			config.DB.Where(models.EmployeeDeduction{EmployeesID: p.emp.ID, DeductionTypeID: dtID}).Assign(rec).FirstOrCreate(&rec)
		}
	}

	type attendanceTotals struct {
		lateMinutes int
		absentDays  int
		leaveDays   int
		otMinutes   int
		holidayDays int
	}
	totals := map[string]map[time.Month]attendanceTotals{}
	for _, p := range profiles {
		totals[p.emp.Email] = map[time.Month]attendanceTotals{}
	}
	locationByCompany := map[uint][]models.WorkLocation{}
	for _, comp := range companies {
		var locs []models.WorkLocation
		config.DB.Where("companies_id = ?", comp.ID).Order("name ASC").Find(&locs)
		locationByCompany[comp.ID] = locs
	}
	isHoliday := func(companyID uint, day time.Time) bool {
		var count int64
		config.DB.Model(&models.Holiday{}).Where("companies_id = ? AND date = ?", companyID, day.Format("2006-01-02")).Count(&count)
		return count > 0
	}
	addTotals := func(email string, month time.Month, delta attendanceTotals) {
		t := totals[email][month]
		t.lateMinutes += delta.lateMinutes
		t.absentDays += delta.absentDays
		t.leaveDays += delta.leaveDays
		t.otMinutes += delta.otMinutes
		t.holidayDays += delta.holidayDays
		totals[email][month] = t
	}
	workTimes := func(p profile, day time.Time) (models.AttendanceStatus, *time.Time, *time.Time, string, attendanceTotals) {
		key := p.emp.Email + "|" + day.Format("2006-01-02")
		if approvedLeaveDays[key] {
			return models.StatusLeave, nil, nil, "Approved leave", attendanceTotals{leaveDays: 1}
		}
		late := 0
		ot := 0
		absent := false
		remark := ""
		switch p.pattern {
		case "manager":
			if day.Day()%10 == 0 || day.Day()%21 == 0 {
				ot = 90
				remark = "Month-end review"
			}
		case "late_ot":
			if day.Day()%8 == 0 || day.Day()%19 == 0 {
				late = 25
				remark = "Traffic delay"
			}
			if day.Day()%6 == 0 {
				ot = 75
				remark = "Deployment support"
			}
		case "steady_ot":
			if day.Day()%7 == 0 || day.Day()%17 == 0 {
				ot = 60
				remark = "Customer support extension"
			}
			if day.Day()%23 == 0 {
				late = 15
				remark = "School drop-off"
			}
		case "mixed_absent":
			if day.Day()%16 == 0 {
				absent = true
				remark = "Unexcused absence"
			} else if day.Day()%5 == 0 || day.Day()%22 == 0 {
				late = 40
				remark = "Late arrival"
			}
			if day.Day()%13 == 0 {
				ot = 45
				remark = "Inventory count"
			}
		default:
			if day.Day()%18 == 0 {
				ot = 60
				remark = "System maintenance"
			}
		}
		if absent {
			return models.StatusAbsent, nil, nil, remark, attendanceTotals{absentDays: 1}
		}
		status := models.StatusOnTime
		if late > 0 {
			status = models.StatusLate
		}
		checkIn := time.Date(day.Year(), day.Month(), day.Day(), 9, late, 0, 0, time.UTC)
		checkOut := time.Date(day.Year(), day.Month(), day.Day(), 17, 0, 0, 0, time.UTC).Add(time.Duration(ot) * time.Minute)
		return status, &checkIn, &checkOut, remark, attendanceTotals{lateMinutes: late, otMinutes: ot}
	}

	for day := start2027; !day.After(end2027); day = day.AddDate(0, 0, 1) {
		isWeekend := day.Weekday() == time.Saturday || day.Weekday() == time.Sunday
		for _, p := range profiles {
			if p.emp.ID == 0 {
				continue
			}
			status, checkIn, checkOut, remark, delta := workTimes(p, day)
			if isWeekend || isHoliday(p.companyID, day) {
				if !(p.pattern == "manager" && day.Day()%14 == 0) && !(p.pattern == "steady_ot" && day.Day()%20 == 0) {
					continue
				}
				status = models.StatusHoliday
				checkInTime := time.Date(day.Year(), day.Month(), day.Day(), 10, 0, 0, 0, time.UTC)
				checkOutTime := time.Date(day.Year(), day.Month(), day.Day(), 16, 0, 0, 0, time.UTC)
				checkIn, checkOut = &checkInTime, &checkOutTime
				remark = "Approved holiday work"
				delta = attendanceTotals{holidayDays: 1, otMinutes: 360}
			}
			addTotals(p.emp.Email, day.Month(), delta)
			var locID *uint
			if locs := locationByCompany[p.companyID]; len(locs) > 0 && checkIn != nil {
				loc := locs[day.Day()%len(locs)]
				locID = &loc.ID
			}
			tl := models.TimeLog{
				CompaniesID: p.companyID, EmployeesID: p.emp.ID, CheckIn: checkIn, CheckOut: checkOut,
				Status: status, WorkLocationID: locID, Remark: remark,
			}
			if checkIn == nil {
				tl.CreatedAt = day
				tl.UpdatedAt = day
				config.DB.Where("employees_id = ? AND status = ? AND created_at >= ? AND created_at < ?", p.emp.ID, status, day, day.AddDate(0, 0, 1)).
					Assign(tl).FirstOrCreate(&tl)
			} else {
				config.DB.Where(models.TimeLog{EmployeesID: p.emp.ID, CheckIn: checkIn}).Assign(tl).FirstOrCreate(&tl)
			}
		}
	}

	roundMoney := func(v float64) float64 { return math.Round(v*100) / 100 }
	monthlyTax := func(taxable float64) float64 {
		annual := taxable*12 - 60000
		if annual <= 150000 {
			return 0
		}
		switch {
		case annual <= 300000:
			return roundMoney((annual - 150000) * 0.05 / 12)
		case annual <= 500000:
			return roundMoney((7500 + (annual-300000)*0.10) / 12)
		case annual <= 750000:
			return roundMoney((27500 + (annual-500000)*0.15) / 12)
		case annual <= 1000000:
			return roundMoney((65000 + (annual-750000)*0.20) / 12)
		default:
			return roundMoney((115000 + (annual-1000000)*0.25) / 12)
		}
	}
	seedCycleItem := func(companyID, cycleID, empID, typeID uint, amount float64, earning bool) {
		if cycleID == 0 || empID == 0 || typeID == 0 || amount <= 0 {
			return
		}
		if earning {
			rec := models.PayrollCycleEarning{CompaniesID: companyID, CycleID: cycleID, EmployeesID: empID, EarningTypeID: typeID, Amount: amount}
			config.DB.Where(models.PayrollCycleEarning{CycleID: cycleID, EmployeesID: empID, EarningTypeID: typeID}).Assign(rec).FirstOrCreate(&rec)
			return
		}
		rec := models.PayrollCycleDeduction{CompaniesID: companyID, CycleID: cycleID, EmployeesID: empID, DeductionTypeID: typeID, Amount: amount}
		config.DB.Where(models.PayrollCycleDeduction{CycleID: cycleID, EmployeesID: empID, DeductionTypeID: typeID}).Assign(rec).FirstOrCreate(&rec)
	}

	for _, p := range profiles {
		if p.emp.ID == 0 {
			continue
		}
		for m := time.January; m <= time.June; m++ {
			cycle := cycleByCompanyMonth[p.companyID][m]
			t := totals[p.emp.Email][m]
			bonus := 0.0
			if m == time.January && (p.pattern == "manager" || p.pattern == "steady") {
				bonus = 12000
			}
			if m == time.April && p.companyID != sysComp.ID {
				bonus += 5000
			}
			otAmount := roundMoney(float64(t.otMinutes) * 5)
			lateDeduction := roundMoney(float64(t.lateMinutes) * 5)
			absentDeduction := roundMoney(float64(t.absentDays) * 1100)
			holidayAmount := roundMoney(float64(t.holidayDays) * 700)
			loanExtra := 0.0
			if p.loan > 0 && (m == time.March || m == time.May) {
				loanExtra = 1500
			}
			seedCycleItem(p.companyID, cycle.ID, p.emp.ID, earningTypeForCompany(p.companyID, "ot"), otAmount+holidayAmount, true)
			seedCycleItem(p.companyID, cycle.ID, p.emp.ID, earningTypeForCompany(p.companyID, "bonus"), bonus, true)
			seedCycleItem(p.companyID, cycle.ID, p.emp.ID, deductionTypeForCompany(p.companyID, "loan"), loanExtra, false)

			totalEarning := roundMoney(p.salary + p.allowance + otAmount + holidayAmount + bonus)
			totalDeduction := roundMoney(p.provident + p.loan + loanExtra + lateDeduction + absentDeduction)
			sso := 0.0
			if p.salary >= 1650 {
				sso = math.Min(roundMoney(p.salary*0.05), 750)
			}
			tax := monthlyTax(totalEarning - p.provident)
			netPay := roundMoney(totalEarning - totalDeduction - sso - tax)

			tx := models.PayrollTransaction{
				CycleID: cycle.ID, EmployeesID: p.emp.ID, SalariesAtTime: p.salary,
				TotalEarning: totalEarning, TotalDeduction: totalDeduction, SsoAmount: sso, TaxAmount: tax, NetPay: netPay,
			}
			var existingTx models.PayrollTransaction
			result := config.DB.Where("cycle_id = ? AND employees_id = ?", cycle.ID, p.emp.ID).Limit(1).Find(&existingTx)
			if result.Error != nil {
				log.Printf("Failed to query 2027 transaction for emp %d: %v", p.emp.ID, result.Error)
				continue
			}
			if result.RowsAffected > 0 {
				tx.ID = existingTx.ID
				config.DB.Model(&existingTx).Updates(tx)
			} else {
				config.DB.Create(&tx)
			}
			config.DB.Unscoped().Where("transaction_id = ?", tx.ID).Delete(&models.PayrollTransactionDetail{})
			details := []models.PayrollTransactionDetail{
				{TransactionID: tx.ID, ItemName: "Base Salary", Amount: p.salary, Type: "earning"},
				{TransactionID: tx.ID, ItemName: "Social Security (SSO)", Amount: sso, Type: "deduction"},
				{TransactionID: tx.ID, ItemName: "Income Tax", Amount: tax, Type: "deduction"},
			}
			if p.allowance > 0 {
				etID := earningTypeForCompany(p.companyID, "position")
				details = append(details, models.PayrollTransactionDetail{TransactionID: tx.ID, ItemName: "Position Allowance", Amount: p.allowance, Type: "earning", EarningTypeID: &etID})
			}
			if otAmount+holidayAmount > 0 {
				etID := earningTypeForCompany(p.companyID, "ot")
				details = append(details, models.PayrollTransactionDetail{TransactionID: tx.ID, ItemName: "Overtime and Holiday Work", Amount: otAmount + holidayAmount, Type: "earning", EarningTypeID: &etID})
			}
			if bonus > 0 {
				etID := earningTypeForCompany(p.companyID, "bonus")
				details = append(details, models.PayrollTransactionDetail{TransactionID: tx.ID, ItemName: "Performance Bonus", Amount: bonus, Type: "earning", EarningTypeID: &etID})
			}
			if p.provident > 0 {
				dtID := deductionTypeForCompany(p.companyID, "provident")
				details = append(details, models.PayrollTransactionDetail{TransactionID: tx.ID, ItemName: "Provident Fund", Amount: p.provident, Type: "deduction", DeductionTypeID: &dtID})
			}
			if p.loan+loanExtra > 0 {
				dtID := deductionTypeForCompany(p.companyID, "loan")
				details = append(details, models.PayrollTransactionDetail{TransactionID: tx.ID, ItemName: "Loan Repayment", Amount: p.loan + loanExtra, Type: "deduction", DeductionTypeID: &dtID})
			}
			if lateDeduction > 0 {
				details = append(details, models.PayrollTransactionDetail{TransactionID: tx.ID, ItemName: "Late Deduction", Amount: lateDeduction, Type: "deduction"})
			}
			if absentDeduction > 0 {
				details = append(details, models.PayrollTransactionDetail{TransactionID: tx.ID, ItemName: "Absent Deduction", Amount: absentDeduction, Type: "deduction"})
			}
			config.DB.Create(&details)
		}
	}

	log.Println("Migration & Setup completed with realistic Jan-Jun 2027 mock data!")
}

package main

import (
	"log"
	"payroll-backend/config"
	"payroll-backend/models"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func ptr(u uint) *uint {
	return &u
}

func main() {
	config.ConnectDatabase()

	err := config.DB.AutoMigrate(
		&models.Roles{},
		&models.AppModule{},
		&models.RolesModuleAccess{},
		&models.Department{},
		&models.Position{},
		&models.Employees{},
		&models.SalariesStructure{},
		&models.PayrollCycle{},
		&models.PayrollTransaction{},
		&models.PayrollTransactionDetail{},
		&models.Companies{},
		&models.TimeLog{},
		&models.LeaveRequest{},
		&models.LeaveBalance{},
		&models.PayrollConfig{},
		&models.EarningType{},
		&models.DeductionType{},
		&models.EmployeeEarning{},
		&models.EmployeeDeduction{},
		&models.PayrollCycleEarning{},
		&models.PayrollCycleDeduction{},
		&models.AttendanceConfig{},
		&models.Bank{},
		&models.EmploymentType{},
		&models.Holiday{},
		&models.WorkLocation{},
		&models.LeaveTypeSetting{},
		&models.EarningCategory{},
		&models.DeductionCategory{},
	)
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
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
		WorkEndTime:               parseTime(18, 0),
		OvertimeRatePerMinute:     5.0,
		ExtraRatePerDay:           300.0,
		IsOvertimeEarningsEnabled: true,
		IsExtraEarningsEnabled:    true,
	}
	config.DB.Where(models.AttendanceConfig{CompaniesID: sysComp.ID}).Assign(attConfigSys).FirstOrCreate(&attConfigSys)

	attConfigA := models.AttendanceConfig{
		CompaniesID:               compA.ID,
		LateRatePerMinute:         5.0,
		AbsentRatePerDay:          300.0,
		WorkStartTime:             parseTime(9, 0),
		WorkEndTime:               parseTime(18, 0),
		OvertimeRatePerMinute:     5.0,
		ExtraRatePerDay:           300.0,
		IsOvertimeEarningsEnabled: true,
		IsExtraEarningsEnabled:    true,
	}
	config.DB.Where(models.AttendanceConfig{CompaniesID: compA.ID}).Assign(attConfigA).FirstOrCreate(&attConfigA)

	attConfigB := models.AttendanceConfig{
		CompaniesID:               compB.ID,
		LateRatePerMinute:         5.0,
		AbsentRatePerDay:          300.0,
		WorkStartTime:             parseTime(9, 0),
		WorkEndTime:               parseTime(18, 0),
		OvertimeRatePerMinute:     5.0,
		ExtraRatePerDay:           300.0,
		IsOvertimeEarningsEnabled: true,
		IsExtraEarningsEnabled:    true,
	}
	config.DB.Where(models.AttendanceConfig{CompaniesID: compB.ID}).Assign(attConfigB).FirstOrCreate(&attConfigB)

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
	empTypes := []models.EmploymentType{
		{Name: "Full-time", Description: "Permanent full-time employee"},
		{Name: "Part-time", Description: "Part-time employee working reduced hours"},
		{Name: "Contract", Description: "Fixed-term contract employee"},
		{Name: "Probation", Description: "Employee on probation period"},
		{Name: "Intern", Description: "Internship / trainee position"},
	}
	for _, et := range empTypes {
		config.DB.Where(models.EmploymentType{Name: et.Name}).Assign(et).FirstOrCreate(&et)
	}

	// Fetch Full-time employment type ID for seeding employees
	var fullTimeEmpType models.EmploymentType
	config.DB.Where("name = ?", "Full-time").First(&fullTimeEmpType)

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
	}

	// --- Earning Types ---
	// Base Salary is NOT listed here — it is managed via the Salaries module (SalariesStructure)
	earnings := []models.EarningType{
		{CompaniesID: sysComp.ID, Name: "Overtime (OT)", IsTaxable: true, IsSso: true, IsPermanent: false},
		{CompaniesID: compA.ID, Name: "Position Allowance", IsTaxable: true, IsSso: false, IsPermanent: true},
		{CompaniesID: compA.ID, Name: "Overtime (OT)", IsTaxable: true, IsSso: true, IsPermanent: false},
		{CompaniesID: compA.ID, Name: "Bonus", IsTaxable: true, IsSso: false, IsPermanent: false},
		{CompaniesID: compB.ID, Name: "Position Allowance", IsTaxable: true, IsSso: false, IsPermanent: true},
		{CompaniesID: compB.ID, Name: "Overtime (OT)", IsTaxable: true, IsSso: true, IsPermanent: false},
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
	sysRole := models.Roles{RolesName: "SUPER_ADMIN", Description: "System Admin Access", IsSuperAdmin: true, CompaniesID: sysComp.ID}
	config.DB.Where(models.Roles{RolesName: "SUPER_ADMIN", CompaniesID: sysComp.ID}).Assign(sysRole).FirstOrCreate(&sysRole)

	sysStaffRole := models.Roles{RolesName: "STAFF", Description: "System Staff", IsSuperAdmin: false, CompaniesID: sysComp.ID}
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
	modules := []models.AppModule{
		// --- Level 1: Main Categories ---
		{ID: 1, ModuleName: "Dashboard", ModuleKey: "DashboardPage", RoutePath: "/dashboard", Icon: "Dashboard", Order: 1, IsCategory: false},
		{ID: 2, ModuleName: "Employees", ModuleKey: "ModuleEmployees", RoutePath: "", Icon: "People", Order: 2, IsCategory: true},
		{ID: 3, ModuleName: "Attendance", ModuleKey: "ModuleAttendance", RoutePath: "", Icon: "Event", Order: 3, IsCategory: true},
		{ID: 4, ModuleName: "Payroll Processing", ModuleKey: "ModulePayroll", RoutePath: "", Icon: "AccountBalanceWallet", Order: 4, IsCategory: true},
		{ID: 5, ModuleName: "Payroll Config", ModuleKey: "ModulePayrollConfig", RoutePath: "", Icon: "SettingsSuggest", Order: 5, IsCategory: true},
		{ID: 6, ModuleName: "Settings", ModuleKey: "ModuleSettings", RoutePath: "", Icon: "Settings", Order: 6, IsCategory: true},
		{ID: 7, ModuleName: "Reports", ModuleKey: "ModuleReports", RoutePath: "", Icon: "FactCheck", Order: 7, IsCategory: true},

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
		{ModuleName: "Attendance Summary", ModuleKey: "AttendanceSummaryPage", RoutePath: "/attendance-summary", Icon: "Summarize", Order: 4, ParentID: ptr(3)},

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
		{ModuleName: "Employment Types", ModuleKey: "EmploymentTypePage", RoutePath: "/employment-types", Icon: "WorkHistory", Order: 4, ParentID: ptr(5)},
		{ModuleName: "Earning & Deduction", ModuleKey: "EarningCategoryPage", RoutePath: "/earning-categories", Icon: "Category", Order: 5, ParentID: ptr(5)},

		// Level 2: Sub-Menus under Settings
		{ModuleName: "Department", ModuleKey: "DepartmentSettings", RoutePath: "/departments", Icon: "CorporateFare", Order: 1, ParentID: ptr(6)},
		{ModuleName: "Position", ModuleKey: "PositionSettings", RoutePath: "/positions", Icon: "WorkOutline", Order: 2, ParentID: ptr(6)},
		{ModuleName: "Roles", ModuleKey: "RolesPage", RoutePath: "/roles", Icon: "ManageAccounts", Order: 3, ParentID: ptr(6)},
		{ModuleName: "Permission", ModuleKey: "PermissionSettings", RoutePath: "/permissions", Icon: "LockPerson", Order: 4, ParentID: ptr(6)},
		{ModuleName: "Companies", ModuleKey: "CompaniesPage", RoutePath: "/companies", Icon: "Business", Order: 5, ParentID: ptr(6)},
		{ModuleName: "Attendance Settings", ModuleKey: "AttendanceSettingsPage", RoutePath: "/attendance-settings", Icon: "Tune", Order: 6, ParentID: ptr(6)},
		{ModuleName: "Bank Master", ModuleKey: "MasterBankPage", RoutePath: "/master-banks", Icon: "AccountBalance", Order: 7, ParentID: ptr(6)},
		{ModuleName: "Holiday Calendar", ModuleKey: "HolidayCalendarPage", RoutePath: "/holidays", Icon: "CalendarMonth", Order: 8, ParentID: ptr(6)},
		{ModuleName: "Work Locations", ModuleKey: "WorkLocationPage", RoutePath: "/work-locations", Icon: "LocationOn", Order: 9, ParentID: ptr(6)},
		{ModuleName: "Leave Types", ModuleKey: "MasterLeaveTypePage", RoutePath: "/leave-types", Icon: "EventAvailable", Order: 10, ParentID: ptr(6)},

		// Level 2: Sub-Menus under Reports
		{ModuleName: "Payslips", ModuleKey: "PayslipsReportPage", RoutePath: "/payslips", Icon: "Description", Order: 1, ParentID: ptr(7)},
		{ModuleName: "Bank Transfer", ModuleKey: "BankTransferReportPage", RoutePath: "/reports/bank-transfer", Icon: "AccountBalance", Order: 2, ParentID: ptr(7)},
		{ModuleName: "Tax Forms", ModuleKey: "TaxFormsReportPage", RoutePath: "/reports/tax-forms", Icon: "Receipt", Order: 3, ParentID: ptr(7)},
		{ModuleName: "Social Security (SSO)", ModuleKey: "SSOReportPage", RoutePath: "/reports/sso", Icon: "HealthAndSafety", Order: 4, ParentID: ptr(7)},
		{ModuleName: "Payroll Analysis", ModuleKey: "PayrollAnalysisPage", RoutePath: "/reports/payroll-analysis", Icon: "Analytics", Order: 5, ParentID: ptr(7)},
	}

	for _, m := range modules {
		var existingModule models.AppModule
		config.DB.Where(models.AppModule{ModuleKey: m.ModuleKey}).Assign(m).FirstOrCreate(&existingModule)

		mID := existingModule.ID

		// SUPER_ADMIN (sysComp) — full access, all scope
		upsert := func(roleID, compID uint, scope models.DataScope, full bool) {
			acc := models.RolesModuleAccess{
				RolesID: roleID, ModuleID: mID, CompaniesID: compID,
				CanView: true, CanCreate: full, CanEdit: full, CanDelete: full,
				DataScope: scope,
			}
			config.DB.Where(models.RolesModuleAccess{RolesID: roleID, ModuleID: mID}).Assign(acc).FirstOrCreate(&acc)
		}

		upsert(sysRole.ID, sysComp.ID, models.ScopeAll, true)            // SUPER_ADMIN: all
		upsert(sysStaffRole.ID, sysComp.ID, models.ScopeCompanies, true) // sysStaff: full within company

		// compA roles
		upsert(managerRoleA.ID, compA.ID, models.ScopeCompanies, true) // MANAGER: full within company
		upsert(staffRoleA.ID, compA.ID, models.ScopeDepartment, false) // STAFF: view only, dept scope

		// compB roles
		upsert(managerRoleB.ID, compB.ID, models.ScopeCompanies, true)
		upsert(staffRoleB.ID, compB.ID, models.ScopeDepartment, false)
	}

	// --- Employees Seeding (Using your existing logic) ---
	defaultPass := "password123"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(defaultPass), 10)
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

	type empSeed struct {
		emp         models.Employees
		salary      float64
		bankID      *uint
		bankName    string
		accountName string
		accountNo   string
	}

	usersToSeed := []empSeed{
		{models.Employees{CompaniesID: sysComp.ID, RolesID: sysStaffRole.ID, DeptID: sysDept.ID, PosID: sysPos.ID, EmploymentTypeID: &fullTimeEmpType.ID, EmployeesCode: "SYS-002", FirstName: "Test", LastName: "Staff", Email: "staff@admin.com", Password: hashStr, IsActive: true}, 0, &bankTTB.ID, bankTTB.BankName, "Test Staff", "6789012345"},
		{models.Employees{CompaniesID: compA.ID, RolesID: managerRoleA.ID, DeptID: deptEngA.ID, PosID: posManagerA.ID, EmploymentTypeID: &fullTimeEmpType.ID, EmployeesCode: "EMP-A-01", FirstName: "Alice", LastName: "Johnson", Email: "alice@techa.com", Password: hashStr, IsActive: true}, 45250.50, &bankKBANK.ID, bankKBANK.BankName, "Alice Johnson", "1234567890"},
		{models.Employees{CompaniesID: compA.ID, RolesID: staffRoleA.ID, DeptID: deptEngA.ID, PosID: posStaffA.ID, EmploymentTypeID: &fullTimeEmpType.ID, EmployeesCode: "EMP-A-02", FirstName: "Bob", LastName: "Smith", Email: "bob@techa.com", Password: hashStr, IsActive: true}, 30750.00, &bankSCB.ID, bankSCB.BankName, "Bob Smith", "2345678901"},
		{models.Employees{CompaniesID: compA.ID, RolesID: staffRoleA.ID, DeptID: deptHrA.ID, PosID: posStaffA.ID, EmploymentTypeID: &fullTimeEmpType.ID, EmployeesCode: "EMP-A-03", FirstName: "Carol", LastName: "Davis", Email: "carol@techa.com", Password: hashStr, IsActive: true}, 28500.75, &bankBBL.ID, bankBBL.BankName, "Carol Davis", "3456789012"},
		{models.Employees{CompaniesID: compB.ID, RolesID: managerRoleB.ID, DeptID: deptEngB.ID, PosID: posManagerB.ID, EmploymentTypeID: &fullTimeEmpType.ID, EmployeesCode: "EMP-B-01", FirstName: "David", LastName: "Wilson", Email: "david@bizgroup.com", Password: hashStr, IsActive: true}, 50125.00, &bankKTB.ID, bankKTB.BankName, "David Wilson", "4567890123"},
		{models.Employees{CompaniesID: compB.ID, RolesID: staffRoleB.ID, DeptID: deptEngB.ID, PosID: posStaffB.ID, EmploymentTypeID: &fullTimeEmpType.ID, EmployeesCode: "EMP-B-02", FirstName: "Eve", LastName: "Brown", Email: "eve@bizgroup.com", Password: hashStr, IsActive: true}, 32300.25, &bankBAY.ID, bankBAY.BankName, "Eve Brown", "5678901234"},
	}

	effDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for _, s := range usersToSeed {
		var existingEmp models.Employees
		config.DB.Unscoped().Where(models.Employees{Email: s.emp.Email}).Assign(s.emp).FirstOrCreate(&existingEmp)

		// Update bank fields on employee
		config.DB.Model(&existingEmp).Updates(map[string]interface{}{
			"bank_id":      s.bankID,
			"bank_name":    s.bankName,
			"account_name": s.accountName,
			"account_no":   s.accountNo,
		})

		if s.salary > 0 {
			salary := models.SalariesStructure{EmployeesID: existingEmp.ID, BaseSalaries: s.salary, EffectiveDate: effDate}
			config.DB.Where(models.SalariesStructure{EmployeesID: existingEmp.ID}).Assign(salary).FirstOrCreate(&salary)
		} else {
			var existing models.SalariesStructure
			config.DB.Unscoped().Where("employees_id = ? AND deleted_at = 0", existingEmp.ID).Delete(&existing)
		}
	}

	// Ensure all existing employees are set to Full-time employment type
	config.DB.Model(&models.Employees{}).Where("employment_type_id IS NULL").Update("employment_type_id", fullTimeEmpType.ID)

	// --- TimeLogs for March & April 2026 (payroll cycle test data with identical data) ---
	// Fetch employees by email so we can reference their IDs
	var alice, bob, carol, david, eve, staff models.Employees
	config.DB.Unscoped().Where("email = ?", "alice@techa.com").First(&alice)
	config.DB.Unscoped().Where("email = ?", "bob@techa.com").First(&bob)
	config.DB.Unscoped().Where("email = ?", "carol@techa.com").First(&carol)
	config.DB.Unscoped().Where("email = ?", "david@bizgroup.com").First(&david)
	config.DB.Unscoped().Where("email = ?", "eve@bizgroup.com").First(&eve)
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
		status   models.AttendanceStatus
		remark   string
	}

	// Alice (Tech Corp MANAGER): mostly OT — check-out at 19:00 or 20:00 (10–11h = 2–3h OT/day)
	// Bob (Tech Corp STAFF): some OT, one late day
	// Carol (Tech Corp STAFF): late check-ins, no OT
	// David (Biz Group MANAGER): normal with OT
	// Eve (Biz Group STAFF): a few late, some OT
	// staff@admin.com: mostly normal
	//
	// Weekdays Mar 1–9 & Apr 1–9, 2026: Mon 2, Tue 3, Wed 4, Thu 5, Fri 6, Mon 9
	// March data
	ciA2, coA2 := d(time.March, 2, 9, 0, 19, 0)  // 10h → 2h OT
	ciA3, coA3 := d(time.March, 3, 9, 0, 20, 0)  // 11h → 3h OT
	ciA4, coA4 := d(time.March, 4, 9, 0, 18, 30) // 9.5h → 1.5h OT
	ciA5, coA5 := d(time.March, 5, 9, 0, 19, 0)  // 10h → 2h OT
	ciA6, coA6 := d(time.March, 6, 9, 0, 18, 0)  // 9h → 1h OT
	ciA9, coA9 := d(time.March, 9, 9, 0, 20, 0)  // 11h → 3h OT

	ciB2, coB2 := d(time.March, 2, 9, 0, 18, 0)   // 9h → 1h OT
	ciB3, coB3 := d(time.March, 3, 9, 0, 18, 0)   // 9h → 1h OT
	ciB4, coB4 := d(time.March, 4, 10, 30, 18, 0) // late, 7.5h → no OT
	ciB5, coB5 := d(time.March, 5, 9, 0, 17, 0)   // 8h → no OT
	ciB6, coB6 := d(time.March, 6, 9, 0, 18, 0)   // 9h → 1h OT
	ciB9, coB9 := d(time.March, 9, 9, 0, 17, 0)   // 8h → no OT

	ciC2, coC2 := d(time.March, 2, 10, 0, 18, 0)  // late, 8h
	ciC3, coC3 := d(time.March, 3, 10, 30, 18, 0) // late, 7.5h
	ciC4, coC4 := d(time.March, 4, 9, 0, 17, 0)   // on_time, 8h
	ciC5, coC5 := d(time.March, 5, 10, 15, 18, 0) // late, 7.75h
	ciC6, coC6 := d(time.March, 6, 10, 0, 18, 0)  // late, 8h
	ciC9, coC9 := d(time.March, 9, 9, 0, 17, 0)   // on_time, 8h

	ciD2, coD2 := d(time.March, 2, 9, 0, 18, 0)
	ciD3, coD3 := d(time.March, 3, 9, 0, 19, 30) // 10.5h → 2.5h OT
	ciD4, coD4 := d(time.March, 4, 9, 0, 18, 0)
	ciD5, coD5 := d(time.March, 5, 9, 0, 20, 0) // 11h → 3h OT
	ciD6, coD6 := d(time.March, 6, 9, 0, 18, 0)
	ciD9, coD9 := d(time.March, 9, 9, 0, 19, 0) // 10h → 2h OT

	ciE2, coE2 := d(time.March, 2, 10, 0, 18, 0)  // late
	ciE3, coE3 := d(time.March, 3, 9, 0, 18, 30)  // 9.5h → 1.5h OT
	ciE4, coE4 := d(time.March, 4, 10, 30, 18, 0) // late
	ciE5, coE5 := d(time.March, 5, 9, 0, 18, 0)   // 9h → 1h OT
	ciE6, coE6 := d(time.March, 6, 9, 0, 17, 0)   // 8h
	ciE9, coE9 := d(time.March, 9, 9, 0, 18, 0)   // 9h → 1h OT

	ciS2, coS2 := d(time.March, 2, 9, 0, 17, 0)
	ciS3, coS3 := d(time.March, 3, 9, 0, 17, 0)
	ciS4, coS4 := d(time.March, 4, 9, 0, 17, 0)
	ciS5, coS5 := d(time.March, 5, 9, 0, 17, 0)
	ciS6, coS6 := d(time.March, 6, 9, 0, 17, 0)
	ciS9, coS9 := d(time.March, 9, 9, 0, 17, 0)

	// April data (identical times, just different dates)
	ciA2Apr, coA2Apr := d(time.April, 2, 9, 0, 19, 0)  // 10h → 2h OT
	ciA3Apr, coA3Apr := d(time.April, 3, 9, 0, 20, 0)  // 11h → 3h OT
	ciA4Apr, coA4Apr := d(time.April, 4, 9, 0, 18, 30) // 9.5h → 1.5h OT
	ciA5Apr, coA5Apr := d(time.April, 5, 9, 0, 19, 0)  // 10h → 2h OT
	ciA6Apr, coA6Apr := d(time.April, 6, 9, 0, 18, 0)  // 9h → 1h OT
	ciA9Apr, coA9Apr := d(time.April, 9, 9, 0, 20, 0)  // 11h → 3h OT

	ciB2Apr, coB2Apr := d(time.April, 2, 9, 0, 18, 0)   // 9h → 1h OT
	ciB3Apr, coB3Apr := d(time.April, 3, 9, 0, 18, 0)   // 9h → 1h OT
	ciB4Apr, coB4Apr := d(time.April, 4, 10, 30, 18, 0) // late, 7.5h → no OT
	ciB5Apr, coB5Apr := d(time.April, 5, 9, 0, 17, 0)   // 8h → no OT
	ciB6Apr, coB6Apr := d(time.April, 6, 9, 0, 18, 0)   // 9h → 1h OT
	ciB9Apr, coB9Apr := d(time.April, 9, 9, 0, 17, 0)   // 8h → no OT

	ciC2Apr, coC2Apr := d(time.April, 2, 10, 0, 18, 0)  // late, 8h
	ciC3Apr, coC3Apr := d(time.April, 3, 10, 30, 18, 0) // late, 7.5h
	ciC4Apr, coC4Apr := d(time.April, 4, 9, 0, 17, 0)   // on_time, 8h
	ciC5Apr, coC5Apr := d(time.April, 5, 10, 15, 18, 0) // late, 7.75h
	ciC6Apr, coC6Apr := d(time.April, 6, 10, 0, 18, 0)  // late, 8h
	ciC9Apr, coC9Apr := d(time.April, 9, 9, 0, 17, 0)   // on_time, 8h

	ciD2Apr, coD2Apr := d(time.April, 2, 9, 0, 18, 0)
	ciD3Apr, coD3Apr := d(time.April, 3, 9, 0, 19, 30) // 10.5h → 2.5h OT
	ciD4Apr, coD4Apr := d(time.April, 4, 9, 0, 18, 0)
	ciD5Apr, coD5Apr := d(time.April, 5, 9, 0, 20, 0) // 11h → 3h OT
	ciD6Apr, coD6Apr := d(time.April, 6, 9, 0, 18, 0)
	ciD9Apr, coD9Apr := d(time.April, 9, 9, 0, 19, 0) // 10h → 2h OT

	ciE2Apr, coE2Apr := d(time.April, 2, 10, 0, 18, 0)  // late
	ciE3Apr, coE3Apr := d(time.April, 3, 9, 0, 18, 30)  // 9.5h → 1.5h OT
	ciE4Apr, coE4Apr := d(time.April, 4, 10, 30, 18, 0) // late
	ciE5Apr, coE5Apr := d(time.April, 5, 9, 0, 18, 0)   // 9h → 1h OT
	ciE6Apr, coE6Apr := d(time.April, 6, 9, 0, 17, 0)   // 8h
	ciE9Apr, coE9Apr := d(time.April, 9, 9, 0, 18, 0)   // 9h → 1h OT

	ciS2Apr, coS2Apr := d(time.April, 2, 9, 0, 17, 0)
	ciS3Apr, coS3Apr := d(time.April, 3, 9, 0, 17, 0)
	ciS4Apr, coS4Apr := d(time.April, 4, 9, 0, 17, 0)
	ciS5Apr, coS5Apr := d(time.April, 5, 9, 0, 17, 0)
	ciS6Apr, coS6Apr := d(time.April, 6, 9, 0, 17, 0)
	ciS9Apr, coS9Apr := d(time.April, 9, 9, 0, 17, 0)

	timelogs := []tlSeed{
		// === MARCH 2026 ===
		// Alice — OT every day
		{alice.ID, compA.ID, ciA2, coA2, models.StatusOnTime, ""},
		{alice.ID, compA.ID, ciA3, coA3, models.StatusOnTime, "Sprint deadline"},
		{alice.ID, compA.ID, ciA4, coA4, models.StatusOnTime, ""},
		{alice.ID, compA.ID, ciA5, coA5, models.StatusOnTime, "Client meeting"},
		{alice.ID, compA.ID, ciA6, coA6, models.StatusOnTime, ""},
		{alice.ID, compA.ID, ciA9, coA9, models.StatusOnTime, "Sprint deadline"},
		// Bob — some OT, one late
		{bob.ID, compA.ID, ciB2, coB2, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB3, coB3, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB4, coB4, models.StatusLate, "Traffic"},
		{bob.ID, compA.ID, ciB5, coB5, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB6, coB6, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB9, coB9, models.StatusOnTime, ""},
		// Carol — late most days, no OT
		{carol.ID, compA.ID, ciC2, coC2, models.StatusLate, "Traffic"},
		{carol.ID, compA.ID, ciC3, coC3, models.StatusLate, "Doctor appointment"},
		{carol.ID, compA.ID, ciC4, coC4, models.StatusOnTime, ""},
		{carol.ID, compA.ID, ciC5, coC5, models.StatusLate, "Traffic"},
		{carol.ID, compA.ID, ciC6, coC6, models.StatusLate, ""},
		{carol.ID, compA.ID, ciC9, coC9, models.StatusOnTime, ""},
		// David — OT several days
		{david.ID, compB.ID, ciD2, coD2, models.StatusOnTime, ""},
		{david.ID, compB.ID, ciD3, coD3, models.StatusOnTime, "Presentation prep"},
		{david.ID, compB.ID, ciD4, coD4, models.StatusOnTime, ""},
		{david.ID, compB.ID, ciD5, coD5, models.StatusOnTime, "Quarter close"},
		{david.ID, compB.ID, ciD6, coD6, models.StatusOnTime, ""},
		{david.ID, compB.ID, ciD9, coD9, models.StatusOnTime, ""},
		// Eve — mixed: late + some OT
		{eve.ID, compB.ID, ciE2, coE2, models.StatusLate, "Traffic"},
		{eve.ID, compB.ID, ciE3, coE3, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE4, coE4, models.StatusLate, ""},
		{eve.ID, compB.ID, ciE5, coE5, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE6, coE6, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE9, coE9, models.StatusOnTime, ""},
		// staff@admin.com — clean, on time, no OT
		{staff.ID, sysComp.ID, ciS2, coS2, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS3, coS3, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS4, coS4, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS5, coS5, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS6, coS6, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS9, coS9, models.StatusOnTime, ""},

		// === APRIL 2026 (identical to March) ===
		// Alice — OT every day
		{alice.ID, compA.ID, ciA2Apr, coA2Apr, models.StatusOnTime, ""},
		{alice.ID, compA.ID, ciA3Apr, coA3Apr, models.StatusOnTime, "Sprint deadline"},
		{alice.ID, compA.ID, ciA4Apr, coA4Apr, models.StatusOnTime, ""},
		{alice.ID, compA.ID, ciA5Apr, coA5Apr, models.StatusOnTime, "Client meeting"},
		{alice.ID, compA.ID, ciA6Apr, coA6Apr, models.StatusOnTime, ""},
		{alice.ID, compA.ID, ciA9Apr, coA9Apr, models.StatusOnTime, "Sprint deadline"},
		// Bob — some OT, one late
		{bob.ID, compA.ID, ciB2Apr, coB2Apr, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB3Apr, coB3Apr, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB4Apr, coB4Apr, models.StatusLate, "Traffic"},
		{bob.ID, compA.ID, ciB5Apr, coB5Apr, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB6Apr, coB6Apr, models.StatusOnTime, ""},
		{bob.ID, compA.ID, ciB9Apr, coB9Apr, models.StatusOnTime, ""},
		// Carol — late most days, no OT
		{carol.ID, compA.ID, ciC2Apr, coC2Apr, models.StatusLate, "Traffic"},
		{carol.ID, compA.ID, ciC3Apr, coC3Apr, models.StatusLate, "Doctor appointment"},
		{carol.ID, compA.ID, ciC4Apr, coC4Apr, models.StatusOnTime, ""},
		{carol.ID, compA.ID, ciC5Apr, coC5Apr, models.StatusLate, "Traffic"},
		{carol.ID, compA.ID, ciC6Apr, coC6Apr, models.StatusLate, ""},
		{carol.ID, compA.ID, ciC9Apr, coC9Apr, models.StatusOnTime, ""},
		// David — OT several days
		{david.ID, compB.ID, ciD2Apr, coD2Apr, models.StatusOnTime, ""},
		{david.ID, compB.ID, ciD3Apr, coD3Apr, models.StatusOnTime, "Presentation prep"},
		{david.ID, compB.ID, ciD4Apr, coD4Apr, models.StatusOnTime, ""},
		{david.ID, compB.ID, ciD5Apr, coD5Apr, models.StatusOnTime, "Quarter close"},
		{david.ID, compB.ID, ciD6Apr, coD6Apr, models.StatusOnTime, ""},
		{david.ID, compB.ID, ciD9Apr, coD9Apr, models.StatusOnTime, ""},
		// Eve — mixed: late + some OT
		{eve.ID, compB.ID, ciE2Apr, coE2Apr, models.StatusLate, "Traffic"},
		{eve.ID, compB.ID, ciE3Apr, coE3Apr, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE4Apr, coE4Apr, models.StatusLate, ""},
		{eve.ID, compB.ID, ciE5Apr, coE5Apr, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE6Apr, coE6Apr, models.StatusOnTime, ""},
		{eve.ID, compB.ID, ciE9Apr, coE9Apr, models.StatusOnTime, ""},
		// staff@admin.com — clean, on time, no OT
		{staff.ID, sysComp.ID, ciS2Apr, coS2Apr, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS3Apr, coS3Apr, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS4Apr, coS4Apr, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS5Apr, coS5Apr, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS6Apr, coS6Apr, models.StatusOnTime, ""},
		{staff.ID, sysComp.ID, ciS9Apr, coS9Apr, models.StatusOnTime, ""},
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
			Status:      tl.status,
			Remark:      tl.remark,
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

	// Mock Jan 2026 payroll data for testing

	// Re-fetch Jan 2026 cycles
	var cycleSysJan models.PayrollCycle
	config.DB.Where("companies_id = ? AND cycle_name = ?", sysComp.ID, "January 2026").First(&cycleSysJan)

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
	}

	for _, s := range mockTxSeeds {
		if s.cycleID == 0 || s.emp.ID == 0 {
			continue
		}
		var tx models.PayrollTransaction
		result := config.DB.Where("cycle_id = ? AND employees_id = ?", s.cycleID, s.emp.ID).First(&tx)
		if result.Error != nil {
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

	// --- Employee Bank Accounts ---
	// Fetch employees for bank account seeding (may already be fetched above, re-fetch to be safe)
	var aliceB, bobB, carolB, davidB, eveB, staffB models.Employees
	config.DB.Unscoped().Where("email = ?", "alice@techa.com").First(&aliceB)
	config.DB.Unscoped().Where("email = ?", "bob@techa.com").First(&bobB)
	config.DB.Unscoped().Where("email = ?", "carol@techa.com").First(&carolB)
	config.DB.Unscoped().Where("email = ?", "david@bizgroup.com").First(&davidB)
	config.DB.Unscoped().Where("email = ?", "eve@bizgroup.com").First(&eveB)
	config.DB.Unscoped().Where("email = ?", "staff@admin.com").First(&staffB)

	log.Println("Migration & Setup completed with Payroll Configs!")
}

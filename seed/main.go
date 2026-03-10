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
	)
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	// --- Companies ---
	sysComp := models.Companies{Name: "System Administration", TaxID: "0000000000000"}
	config.DB.Where(models.Companies{TaxID: "0000000000000"}).Assign(sysComp).FirstOrCreate(&sysComp)

	compA := models.Companies{Name: "Tech Corp", TaxID: "1111111111111"}
	config.DB.Where(models.Companies{TaxID: "1111111111111"}).Assign(compA).FirstOrCreate(&compA)

	compB := models.Companies{Name: "Biz Group", TaxID: "2222222222222"}
	config.DB.Where(models.Companies{TaxID: "2222222222222"}).Assign(compB).FirstOrCreate(&compB)

	// --- Payroll Configs (Global Settings) ---
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

	// --- Attendance Configs ---
	attConfigSys := models.AttendanceConfig{
		CompaniesID:       sysComp.ID,
		LateRatePerMinute: 5.0,
		AbsentRatePerDay:  300.0,
		WorkStartHour:     9,
		WorkStartMin:      0,
	}
	config.DB.Where(models.AttendanceConfig{CompaniesID: sysComp.ID}).Assign(attConfigSys).FirstOrCreate(&attConfigSys)

	attConfigA := models.AttendanceConfig{
		CompaniesID:       compA.ID,
		LateRatePerMinute: 5.0,
		AbsentRatePerDay:  300.0,
		WorkStartHour:     9,
		WorkStartMin:      0,
	}
	config.DB.Where(models.AttendanceConfig{CompaniesID: compA.ID}).Assign(attConfigA).FirstOrCreate(&attConfigA)

	attConfigB := models.AttendanceConfig{
		CompaniesID:       compB.ID,
		LateRatePerMinute: 5.0,
		AbsentRatePerDay:  300.0,
		WorkStartHour:     9,
		WorkStartMin:      0,
	}
	config.DB.Where(models.AttendanceConfig{CompaniesID: compB.ID}).Assign(attConfigB).FirstOrCreate(&attConfigB)

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

		// Level 2: Sub-Menus under Payroll Config
		{ModuleName: "Global Settings", ModuleKey: "PayrollGlobalSettingsPage", RoutePath: "/payroll-global", Icon: "SettingsSuggest", Order: 1, ParentID: ptr(5)},
		{ModuleName: "Earning Types", ModuleKey: "EarningTypesPage", RoutePath: "/earning-types", Icon: "AddCard", Order: 2, ParentID: ptr(5)},
		{ModuleName: "Deduction Types", ModuleKey: "DeductionTypesPage", RoutePath: "/deduction-types", Icon: "PriceCheck", Order: 3, ParentID: ptr(5)},

		// Level 2: Sub-Menus under Settings
		{ModuleName: "Department", ModuleKey: "DepartmentSettings", RoutePath: "/departments", Icon: "CorporateFare", Order: 1, ParentID: ptr(6)},
		{ModuleName: "Position", ModuleKey: "PositionSettings", RoutePath: "/positions", Icon: "WorkOutline", Order: 2, ParentID: ptr(6)},
		{ModuleName: "Roles", ModuleKey: "RolesPage", RoutePath: "/roles", Icon: "ManageAccounts", Order: 3, ParentID: ptr(6)},
		{ModuleName: "Permission", ModuleKey: "PermissionSettings", RoutePath: "/permissions", Icon: "LockPerson", Order: 4, ParentID: ptr(6)},
		{ModuleName: "Companies", ModuleKey: "CompaniesPage", RoutePath: "/companies", Icon: "Business", Order: 5, ParentID: ptr(6)},
		{ModuleName: "Attendance Settings", ModuleKey: "AttendanceSettingsPage", RoutePath: "/attendance-settings", Icon: "Tune", Order: 6, ParentID: ptr(6)},

		// Level 2: Sub-Menus under Reports
		{ModuleName: "Payslips", ModuleKey: "PayslipsReportPage", RoutePath: "/payslips", Icon: "Description", Order: 1, ParentID: ptr(7)},
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
		emp    models.Employees
		salary float64
	}

	usersToSeed := []empSeed{
		{models.Employees{CompaniesID: sysComp.ID, RolesID: sysStaffRole.ID, DeptID: sysDept.ID, PosID: sysPos.ID, EmployeesCode: "SYS-002", FirstName: "Test", LastName: "Staff", Email: "staff@admin.com", Password: hashStr, IsActive: true}, 40000.0},
		{models.Employees{CompaniesID: compA.ID, RolesID: managerRoleA.ID, DeptID: deptEngA.ID, PosID: posManagerA.ID, EmployeesCode: "EMP-A-01", FirstName: "Alice", LastName: "Johnson", Email: "alice@techa.com", Password: hashStr, IsActive: true}, 45000.0},
		{models.Employees{CompaniesID: compA.ID, RolesID: staffRoleA.ID, DeptID: deptEngA.ID, PosID: posStaffA.ID, EmployeesCode: "EMP-A-02", FirstName: "Bob", LastName: "Smith", Email: "bob@techa.com", Password: hashStr, IsActive: true}, 30000.0},
		{models.Employees{CompaniesID: compA.ID, RolesID: staffRoleA.ID, DeptID: deptHrA.ID, PosID: posStaffA.ID, EmployeesCode: "EMP-A-03", FirstName: "Carol", LastName: "Davis", Email: "carol@techa.com", Password: hashStr, IsActive: true}, 28000.0},
		{models.Employees{CompaniesID: compB.ID, RolesID: managerRoleB.ID, DeptID: deptEngB.ID, PosID: posManagerB.ID, EmployeesCode: "EMP-B-01", FirstName: "David", LastName: "Wilson", Email: "david@bizgroup.com", Password: hashStr, IsActive: true}, 50000.0},
		{models.Employees{CompaniesID: compB.ID, RolesID: staffRoleB.ID, DeptID: deptEngB.ID, PosID: posStaffB.ID, EmployeesCode: "EMP-B-02", FirstName: "Eve", LastName: "Brown", Email: "eve@bizgroup.com", Password: hashStr, IsActive: true}, 32000.0},
	}

	effDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for _, s := range usersToSeed {
		var existingEmp models.Employees
		config.DB.Unscoped().Where(models.Employees{Email: s.emp.Email}).Assign(s.emp).FirstOrCreate(&existingEmp)

		salary := models.SalariesStructure{EmployeesID: existingEmp.ID, BaseSalaries: s.salary, IsEnabled: true, EffectiveDate: effDate}
		config.DB.Where(models.SalariesStructure{EmployeesID: existingEmp.ID}).Assign(salary).FirstOrCreate(&salary)
	}

	// --- TimeLogs for March 2026 (payroll cycle test data) ---
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
	// Weekdays Mar 1–9, 2026: Mon 2, Tue 3, Wed 4, Thu 5, Fri 6, Mon 9
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

	timelogs := []tlSeed{
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
		// Tech Corp (compA)
		{models.PayrollCycle{CompaniesID: compA.ID, CycleName: "January 2026", StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC), IsClosed: true}, compA.ID},
		{models.PayrollCycle{CompaniesID: compA.ID, CycleName: "February 2026", StartDate: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 2, 28, 23, 59, 59, 0, time.UTC), IsClosed: true}, compA.ID},
		{models.PayrollCycle{CompaniesID: compA.ID, CycleName: "March 2026", StartDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 3, 31, 23, 59, 59, 0, time.UTC), IsClosed: false}, compA.ID},
		// Biz Group (compB)
		{models.PayrollCycle{CompaniesID: compB.ID, CycleName: "January 2026", StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC), IsClosed: true}, compB.ID},
		{models.PayrollCycle{CompaniesID: compB.ID, CycleName: "February 2026", StartDate: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 2, 28, 23, 59, 59, 0, time.UTC), IsClosed: true}, compB.ID},
		{models.PayrollCycle{CompaniesID: compB.ID, CycleName: "March 2026", StartDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 3, 31, 23, 59, 59, 0, time.UTC), IsClosed: false}, compB.ID},
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

	log.Println("Migration & Setup completed with Payroll Configs!")
}

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
		&models.Companies{},
		&models.TimeLog{},
		&models.LeaveRequest{},
		&models.LeaveBalance{},
		&models.PayrollConfig{},
		&models.EarningType{},
		&models.DeductionType{},
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
	configA := models.PayrollConfig{
		CompaniesID:    compA.ID,
		SsoRate:        5.0,
		SsoMaxAmount:   750,
		SsoMinSalary:   1650,
		TaxCalculation: "Step",
	}
	config.DB.Where(models.PayrollConfig{CompaniesID: compA.ID}).Assign(configA).FirstOrCreate(&configA)

	// --- Earning Types ---
	earnings := []models.EarningType{
		{CompaniesID: compA.ID, Name: "Base Salary", IsTaxable: true, IsSso: true, IsPermanent: true},
		{CompaniesID: compA.ID, Name: "Position Allowance", IsTaxable: true, IsSso: false, IsPermanent: true},
		{CompaniesID: compA.ID, Name: "Overtime (OT)", IsTaxable: true, IsSso: true, IsPermanent: false},
		{CompaniesID: compA.ID, Name: "Bonus", IsTaxable: true, IsSso: false, IsPermanent: false},
	}
	for _, e := range earnings {
		config.DB.Where(models.EarningType{CompaniesID: e.CompaniesID, Name: e.Name}).Assign(e).FirstOrCreate(&e)
	}

	// --- Deduction Types ---
	deductions := []models.DeductionType{
		{CompaniesID: compA.ID, Name: "Loan Repayment", IsTaxDeductible: false, IsSsoDeductible: false},
		{CompaniesID: compA.ID, Name: "Provident Fund", IsTaxDeductible: true, IsSsoDeductible: false},
	}
	for _, d := range deductions {
		config.DB.Where(models.DeductionType{CompaniesID: d.CompaniesID, Name: d.Name}).Assign(d).FirstOrCreate(&d)
	}

	// --- Roles ---
	sysRole := models.Roles{RolesName: "SUPER_ADMIN", Description: "System Admin Access", IsSuperAdmin: true, CompaniesID: sysComp.ID}
	config.DB.Where(models.Roles{RolesName: "SUPER_ADMIN", CompaniesID: sysComp.ID}).Assign(sysRole).FirstOrCreate(&sysRole)

	managerRoleA := models.Roles{RolesName: "MANAGER", Description: "Sees whole companies", CompaniesID: compA.ID}
	config.DB.Where(models.Roles{RolesName: "MANAGER", CompaniesID: compA.ID}).Assign(managerRoleA).FirstOrCreate(&managerRoleA)
	staffRoleA := models.Roles{RolesName: "STAFF", Description: "Sees only department", CompaniesID: compA.ID}
	config.DB.Where(models.Roles{RolesName: "STAFF", CompaniesID: compA.ID}).Assign(staffRoleA).FirstOrCreate(&staffRoleA)

	// --- Departments & Positions (Omitted for brevity, keep your original logic here) ---
	sysDept := models.Department{DeptName: "System IT", CompaniesID: sysComp.ID}
	config.DB.Where(models.Department{DeptName: "System IT", CompaniesID: sysComp.ID}).Assign(sysDept).FirstOrCreate(&sysDept)
	deptEngA := models.Department{DeptName: "Engineering", CompaniesID: compA.ID}
	config.DB.Where(models.Department{DeptName: "Engineering", CompaniesID: compA.ID}).Assign(deptEngA).FirstOrCreate(&deptEngA)
	sysPos := models.Position{PosName: "System Admin", CompaniesID: sysComp.ID}
	config.DB.Where(models.Position{PosName: "System Admin", CompaniesID: sysComp.ID}).Assign(sysPos).FirstOrCreate(&sysPos)
	posManagerA := models.Position{PosName: "Manager", CompaniesID: compA.ID}
	config.DB.Where(models.Position{PosName: "Manager", CompaniesID: compA.ID}).Assign(posManagerA).FirstOrCreate(&posManagerA)
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

		// Level 2: Sub-Menus under Attendance
		{ModuleName: "Timelogs", ModuleKey: "TimelogsListPage", RoutePath: "/timelogs", Icon: "Schedule", Order: 1, ParentID: ptr(3)},
		{ModuleName: "Leave Requests", ModuleKey: "LeaveRequestsPage", RoutePath: "/leave-requests", Icon: "EventNote", Order: 2, ParentID: ptr(3)},
		{ModuleName: "Leave Approved", ModuleKey: "LeaveApprovedPage", RoutePath: "/leave-approved", Icon: "FactCheck", Order: 3, ParentID: ptr(3)},

		// Level 2: Sub-Menus under Payroll Processing
		{ModuleName: "Payroll Cycles", ModuleKey: "PayrollCyclesPage", RoutePath: "/payroll-cycles", Icon: "Sync", Order: 1, ParentID: ptr(4)},
		{ModuleName: "Run Payroll", ModuleKey: "RunPayrollPage", RoutePath: "/run-payroll", Icon: "PlayCircle", Order: 2, ParentID: ptr(4)},

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

		// Level 2: Sub-Menus under Reports
		{ModuleName: "Payslips", ModuleKey: "PayslipsReportPage", RoutePath: "/payslips", Icon: "Description", Order: 1, ParentID: ptr(7)},
	}

	for _, m := range modules {
		var existingModule models.AppModule
		config.DB.Where(models.AppModule{ModuleKey: m.ModuleKey}).Assign(m).FirstOrCreate(&existingModule)

		// Roles Access logic remains the same (Looping through modules)
		accSys := models.RolesModuleAccess{RolesID: sysRole.ID, ModuleID: existingModule.ID, CompaniesID: sysComp.ID, CanView: true, CanCreate: true, CanEdit: true, CanDelete: true, DataScope: models.ScopeAll}
		config.DB.Where(models.RolesModuleAccess{RolesID: sysRole.ID, ModuleID: existingModule.ID}).Assign(accSys).FirstOrCreate(&accSys)
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

	usersToSeed := []models.Employees{
		{CompaniesID: compA.ID, RolesID: managerRoleA.ID, DeptID: deptEngA.ID, PosID: posManagerA.ID, EmployeesCode: "EMP-A-01", FirstName: "Alice", LastName: "ManagerA", Email: "alice@techa.com", Password: hashStr, IsActive: true},
	}

	for _, u := range usersToSeed {
		var existingEmp models.Employees
		config.DB.Unscoped().Where(models.Employees{Email: u.Email}).Assign(u).FirstOrCreate(&existingEmp)

		// Seed Salary for testing (English Comment)
		salary := models.SalariesStructure{EmployeesID: existingEmp.ID, BaseSalaries: 30000.0, IsEnabled: true, EffectiveDate: time.Now()}
		config.DB.Where(models.SalariesStructure{EmployeesID: existingEmp.ID}).Assign(salary).FirstOrCreate(&salary)
	}

	log.Println("Migration & Setup completed with Payroll Configs!")
}

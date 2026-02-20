package main

import (
	"fmt"
	"log"
	"payroll-backend/config"
	"payroll-backend/models"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	config.ConnectDatabase()

	err := config.DB.AutoMigrate(
		&models.Companies{},
		&models.Roles{},
		&models.AppModule{},
		&models.RolesModuleAccess{},
		&models.Department{},
		&models.Position{},
		&models.Employees{},
	)
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	sysComp := models.Companies{Name: "System Administration", TaxID: "0000000000000"}
	config.DB.Where(models.Companies{TaxID: "0000000000000"}).Assign(sysComp).FirstOrCreate(&sysComp)

	compA := models.Companies{Name: "Tech Corp", TaxID: "1111111111111"}
	config.DB.Where(models.Companies{TaxID: "1111111111111"}).Assign(compA).FirstOrCreate(&compA)

	compB := models.Companies{Name: "Biz Group", TaxID: "2222222222222"}
	config.DB.Where(models.Companies{TaxID: "2222222222222"}).Assign(compB).FirstOrCreate(&compB)

	sysRole := models.Roles{RolesName: "SUPER_ADMIN", Description: "System Admin Access", IsSuperAdmin: true, CompaniesID: sysComp.ID}
	config.DB.Where(models.Roles{RolesName: "SUPER_ADMIN", CompaniesID: sysComp.ID}).Assign(sysRole).FirstOrCreate(&sysRole)

	managerRoleA := models.Roles{RolesName: "MANAGER", Description: "Sees whole company", CompaniesID: compA.ID}
	config.DB.Where(models.Roles{RolesName: "MANAGER", CompaniesID: compA.ID}).Assign(managerRoleA).FirstOrCreate(&managerRoleA)
	staffRoleA := models.Roles{RolesName: "STAFF", Description: "Sees only department", CompaniesID: compA.ID}
	config.DB.Where(models.Roles{RolesName: "STAFF", CompaniesID: compA.ID}).Assign(staffRoleA).FirstOrCreate(&staffRoleA)

	managerRoleB := models.Roles{RolesName: "MANAGER", Description: "Sees whole company", CompaniesID: compB.ID}
	config.DB.Where(models.Roles{RolesName: "MANAGER", CompaniesID: compB.ID}).Assign(managerRoleB).FirstOrCreate(&managerRoleB)
	staffRoleB := models.Roles{RolesName: "STAFF", Description: "Sees only department", CompaniesID: compB.ID}
	config.DB.Where(models.Roles{RolesName: "STAFF", CompaniesID: compB.ID}).Assign(staffRoleB).FirstOrCreate(&staffRoleB)

	sysDept := models.Department{DeptName: "System IT", CompaniesID: sysComp.ID}
	config.DB.Where(models.Department{DeptName: "System IT", CompaniesID: sysComp.ID}).Assign(sysDept).FirstOrCreate(&sysDept)

	deptEngA := models.Department{DeptName: "Engineering", CompaniesID: compA.ID}
	config.DB.Where(models.Department{DeptName: "Engineering", CompaniesID: compA.ID}).Assign(deptEngA).FirstOrCreate(&deptEngA)
	deptHrA := models.Department{DeptName: "HR", CompaniesID: compA.ID}
	config.DB.Where(models.Department{DeptName: "HR", CompaniesID: compA.ID}).Assign(deptHrA).FirstOrCreate(&deptHrA)

	deptSalesB := models.Department{DeptName: "Sales", CompaniesID: compB.ID}
	config.DB.Where(models.Department{DeptName: "Sales", CompaniesID: compB.ID}).Assign(deptSalesB).FirstOrCreate(&deptSalesB)
	deptMktB := models.Department{DeptName: "Marketing", CompaniesID: compB.ID}
	config.DB.Where(models.Department{DeptName: "Marketing", CompaniesID: compB.ID}).Assign(deptMktB).FirstOrCreate(&deptMktB)

	sysPos := models.Position{PosName: "System Admin", CompaniesID: sysComp.ID}
	config.DB.Where(models.Position{PosName: "System Admin", CompaniesID: sysComp.ID}).Assign(sysPos).FirstOrCreate(&sysPos)

	posManagerA := models.Position{PosName: "Manager", CompaniesID: compA.ID}
	config.DB.Where(models.Position{PosName: "Manager", CompaniesID: compA.ID}).Assign(posManagerA).FirstOrCreate(&posManagerA)
	posStaffA := models.Position{PosName: "Staff", CompaniesID: compA.ID}
	config.DB.Where(models.Position{PosName: "Staff", CompaniesID: compA.ID}).Assign(posStaffA).FirstOrCreate(&posStaffA)

	posManagerB := models.Position{PosName: "Manager", CompaniesID: compB.ID}
	config.DB.Where(models.Position{PosName: "Manager", CompaniesID: compB.ID}).Assign(posManagerB).FirstOrCreate(&posManagerB)
	posStaffB := models.Position{PosName: "Staff", CompaniesID: compB.ID}
	config.DB.Where(models.Position{PosName: "Staff", CompaniesID: compB.ID}).Assign(posStaffB).FirstOrCreate(&posStaffB)

	modules := []models.AppModule{
		{ModuleName: "Dashboard", ModuleKey: "DASHBOARD", RoutePath: "/dashboard", Icon: "Dashboard", Order: 1},
		{ModuleName: "Employees", ModuleKey: "EMPLOYEES", RoutePath: "/employees", Icon: "Employees", Order: 2},
		{ModuleName: "Attendance", ModuleKey: "ATTENDANCE", RoutePath: "/attendance", Icon: "Attendance", Order: 3},
		{ModuleName: "Settings", ModuleKey: "SETTINGS", RoutePath: "/settings", Icon: "Settings", Order: 4},
		{ModuleName: "Companies", ModuleKey: "COMPANIES", RoutePath: "/companies", Icon: "Companies", Order: 5},
	}

	for _, m := range modules {
		var existingModule models.AppModule
		config.DB.Where(models.AppModule{ModuleKey: m.ModuleKey}).Assign(m).FirstOrCreate(&existingModule)

		accSys := models.RolesModuleAccess{RolesID: sysRole.ID, ModuleID: existingModule.ID, CompaniesID: sysComp.ID, CanView: true, CanCreate: true, CanEdit: true, CanDelete: true, DataScope: models.ScopeAll}
		config.DB.Where(models.RolesModuleAccess{RolesID: sysRole.ID, ModuleID: existingModule.ID}).Assign(accSys).FirstOrCreate(&accSys)

		accMgrA := models.RolesModuleAccess{RolesID: managerRoleA.ID, ModuleID: existingModule.ID, CompaniesID: compA.ID, CanView: true, CanCreate: true, CanEdit: true, CanDelete: false, DataScope: models.ScopeCompany}
		config.DB.Where(models.RolesModuleAccess{RolesID: managerRoleA.ID, ModuleID: existingModule.ID}).Assign(accMgrA).FirstOrCreate(&accMgrA)

		accMgrB := models.RolesModuleAccess{RolesID: managerRoleB.ID, ModuleID: existingModule.ID, CompaniesID: compB.ID, CanView: true, CanCreate: true, CanEdit: true, CanDelete: false, DataScope: models.ScopeCompany}
		config.DB.Where(models.RolesModuleAccess{RolesID: managerRoleB.ID, ModuleID: existingModule.ID}).Assign(accMgrB).FirstOrCreate(&accMgrB)

		accStaffA := models.RolesModuleAccess{RolesID: staffRoleA.ID, ModuleID: existingModule.ID, CompaniesID: compA.ID, CanView: true, CanCreate: false, CanEdit: false, CanDelete: false, DataScope: models.ScopeDepartment}
		config.DB.Where(models.RolesModuleAccess{RolesID: staffRoleA.ID, ModuleID: existingModule.ID}).Assign(accStaffA).FirstOrCreate(&accStaffA)

		accStaffB := models.RolesModuleAccess{RolesID: staffRoleB.ID, ModuleID: existingModule.ID, CompaniesID: compB.ID, CanView: true, CanCreate: false, CanEdit: false, CanDelete: false, DataScope: models.ScopeDepartment}
		config.DB.Where(models.RolesModuleAccess{RolesID: staffRoleB.ID, ModuleID: existingModule.ID}).Assign(accStaffB).FirstOrCreate(&accStaffB)
	}

	defaultPass := "password123"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(defaultPass), 10)
	hashStr := string(hashedPassword)

	admin := models.Employees{Email: "admin@admin.com"}
	config.DB.Unscoped().Where(models.Employees{Email: admin.Email}).
		Assign(models.Employees{
			CompaniesID:   sysComp.ID,
			RolesID:       sysRole.ID,
			EmployeesCode: "SYS-001",
			FirstName:     "System",
			LastName:      "Admin",
			Password:      hashStr,
			DeptID:        sysDept.ID,
			PosID:         sysPos.ID,
		}).FirstOrCreate(&admin)

	usersToSeed := []models.Employees{
		{CompaniesID: compA.ID, RolesID: managerRoleA.ID, DeptID: deptEngA.ID, PosID: posManagerA.ID, EmployeesCode: "EMP-A-01", FirstName: "Alice", LastName: "ManagerA", Email: "alice@techa.com", Password: hashStr, IsActive: true},
		{CompaniesID: compA.ID, RolesID: staffRoleA.ID, DeptID: deptEngA.ID, PosID: posStaffA.ID, EmployeesCode: "EMP-A-02", FirstName: "Bob", LastName: "StaffEngA", Email: "bob@techa.com", Password: hashStr, IsActive: true},
		{CompaniesID: compA.ID, RolesID: staffRoleA.ID, DeptID: deptHrA.ID, PosID: posStaffA.ID, EmployeesCode: "EMP-A-03", FirstName: "Charlie", LastName: "StaffHrA", Email: "charlie@techa.com", Password: hashStr, IsActive: true},
		{CompaniesID: compA.ID, RolesID: staffRoleA.ID, DeptID: deptHrA.ID, PosID: posStaffA.ID, EmployeesCode: "EMP-A-04", FirstName: "Dave", LastName: "StaffHrA", Email: "dave@techa.com", Password: hashStr, IsActive: true},
		{CompaniesID: compA.ID, RolesID: staffRoleA.ID, DeptID: deptHrA.ID, PosID: posStaffA.ID, EmployeesCode: "EMP-A-05", FirstName: "Eve", LastName: "StaffHrA", Email: "eve@techa.com", Password: hashStr, IsActive: true},

		{CompaniesID: compB.ID, RolesID: managerRoleB.ID, DeptID: deptSalesB.ID, PosID: posManagerB.ID, EmployeesCode: "EMP-B-01", FirstName: "Frank", LastName: "ManagerB", Email: "frank@bizb.com", Password: hashStr, IsActive: true},
		{CompaniesID: compB.ID, RolesID: staffRoleB.ID, DeptID: deptSalesB.ID, PosID: posStaffB.ID, EmployeesCode: "EMP-B-02", FirstName: "Grace", LastName: "StaffSalesB", Email: "grace@bizb.com", Password: hashStr, IsActive: true},
		{CompaniesID: compB.ID, RolesID: staffRoleB.ID, DeptID: deptMktB.ID, PosID: posStaffB.ID, EmployeesCode: "EMP-B-03", FirstName: "Heidi", LastName: "StaffMktB", Email: "heidi@bizb.com", Password: hashStr, IsActive: true},
		{CompaniesID: compB.ID, RolesID: staffRoleB.ID, DeptID: deptMktB.ID, PosID: posStaffB.ID, EmployeesCode: "EMP-B-04", FirstName: "Ivan", LastName: "StaffMktB", Email: "ivan@bizb.com", Password: hashStr, IsActive: true},
		{CompaniesID: compB.ID, RolesID: staffRoleB.ID, DeptID: deptMktB.ID, PosID: posStaffB.ID, EmployeesCode: "EMP-B-05", FirstName: "Judy", LastName: "StaffMktB", Email: "judy@bizb.com", Password: hashStr, IsActive: true},
	}

	for _, u := range usersToSeed {
		var existingEmp models.Employees
		err := config.DB.Unscoped().Where(models.Employees{Email: u.Email}).Assign(u).FirstOrCreate(&existingEmp).Error
		if err != nil {
			fmt.Printf("Error seeding user %s: %v\n", u.Email, err)
		}
	}

	log.Println("Migration & Setup completed! System Admin and 10 Employees seeded successfully.")
}

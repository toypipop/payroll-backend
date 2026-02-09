package main

import (
	"log"
	"payroll-backend/config"
	"payroll-backend/models"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	config.ConnectDatabase()

	// 0. Auto Migrate Tables to ensure schema exists
	err := config.DB.AutoMigrate(
		&models.Companies{},
		&models.Role{},
		&models.AppModule{},
		&models.RoleModuleAccess{},
		&models.Department{},
		&models.Position{},
		&models.Employee{},
	)
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	// 1. Create Initial Companies
	companies := models.Companies{
		Name:  "Default Corporation",
		TaxID: "1234567890123",
	}
	config.DB.Where(models.Companies{TaxID: "1234567890123"}).FirstOrCreate(&companies)

	// 2. Create Roles tied to Companies
	adminRole := models.Role{
		RoleName:     "SUPER_ADMIN",
		Description:  "System administrator with full access",
		IsSuperAdmin: true,
		CompaniesID:    companies.ID,
	}
	config.DB.Where(models.Role{RoleName: "SUPER_ADMIN", CompaniesID: companies.ID}).FirstOrCreate(&adminRole)

	userRole := models.Role{
		RoleName:     "USER",
		Description:  "Regular user with limited access",
		IsSuperAdmin: false,
		CompaniesID:    companies.ID,
	}
	config.DB.Where(models.Role{RoleName: "USER", CompaniesID: companies.ID}).FirstOrCreate(&userRole)

	// 3. Create AppModule (Global System Level)
	modules := []models.AppModule{
		{ModuleName: "Dashboard", ModuleKey: "DASHBOARD", RoutePath: "/dashboard", Icon: "LayoutDashboard", Order: 1},
		{ModuleName: "Employees", ModuleKey: "EMPLOYEE", RoutePath: "/employees", Icon: "Users", Order: 2},
		{ModuleName: "Settings", ModuleKey: "SETTINGS", RoutePath: "/settings", Icon: "Settings", Order: 3},
		{ModuleName: "Departments", ModuleKey: "DEPARTMENT", RoutePath: "/departments", Icon: "Building", Order: 4},
		{ModuleName: "Positions", ModuleKey: "POSITION", RoutePath: "/positions", Icon: "UserCheck", Order: 5},
	}

	for _, m := range modules {
		var existingModule models.AppModule
		config.DB.Where(models.AppModule{ModuleKey: m.ModuleKey}).FirstOrCreate(&existingModule, m)

		// 4. Assign Full Access to SUPER_ADMIN
		access := models.RoleModuleAccess{
			RoleID:    adminRole.ID,
			ModuleID:  existingModule.ID,
			CompaniesID: companies.ID,
			CanView:   true,
			CanCreate: true,
			CanEdit:   true,
			CanDelete: true,
			DataScope: "ALL",
		}
		config.DB.Where(models.RoleModuleAccess{RoleID: adminRole.ID, ModuleID: existingModule.ID}).FirstOrCreate(&access)
	}

	// 5. Master Data
	dept := models.Department{DeptName: "IT Administration", CompaniesID: companies.ID}
	config.DB.Where(models.Department{DeptName: "IT Administration", CompaniesID: companies.ID}).FirstOrCreate(&dept)

	pos := models.Position{PosName: "System Administrator", CompaniesID: companies.ID}
	config.DB.Where(models.Position{PosName: "System Administrator", CompaniesID: companies.ID}).FirstOrCreate(&pos)

	// 6. Professional Seeding Style for Employee
	adminPass := "admin@admin.com"
	// Use cost 10 to match CreateEmployee logic
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(adminPass), 10)

	admin := models.Employee{
		Email: "admin@admin.com", // Unique identifier for seeding
	}

	// Use Unscoped to find soft-deleted records and Assign to force update fields
	err = config.DB.Unscoped().Where(models.Employee{Email: admin.Email}).
		Assign(models.Employee{
			CompaniesID:    companies.ID,
			RoleID:       adminRole.ID,
			EmployeeCode: "ADM001",
			FirstName:    "System",
			LastName:     "Admin",
			Password:     string(hashedPassword),
			DeptID:       dept.ID,
			PosID:        pos.ID,
			IsActive:     true,
		}).
		FirstOrCreate(&admin).Error

	if err != nil {
		log.Fatalf("Failed to seed admin: %v", err)
	}

	log.Println("Admin seeded/updated successfully using GORM logic!")
	log.Println("Migration & Seed completed successfully!")
}

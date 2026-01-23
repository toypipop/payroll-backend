package main

import (
	"log"
	"os"
	"payroll-backend/config"
	"payroll-backend/models"
)

func main() {
	config.ConnectDatabase()

	adminPass := os.Getenv("ADMIN_PASSWORD")
	if adminPass == "" {
		adminPass = "Admin@123" // Default password if env is missing
	}

	// 1. Create System Admin Role (English Comment)
	adminRole := models.Role{
		RoleName:     "SUPER_ADMIN",
		Description:  "System administrator with full access",
		IsSuperAdmin: true,
	}
	userRole := models.Role{
		RoleName:     "USER",
		Description:  "Regular user with limited access",
		IsSuperAdmin: false,
	}
	config.DB.Where(models.Role{RoleName: "SUPER_ADMIN"}).FirstOrCreate(&adminRole)
	config.DB.Where(models.Role{RoleName: "USER"}).FirstOrCreate(&userRole)

	// 2. Create Initial Company (English Comment)
	company := models.Company{
		Name:  "Default Corporation",
		TaxID: "1234567890123",
	}
	config.DB.Where(models.Company{TaxID: "1234567890123"}).FirstOrCreate(&company)

	// 3. Create AppModule (Dashboard, Employee, Settings) (English Comment)
	modules := []models.AppModule{
		{ModuleName: "Dashboard", ModuleKey: "DASHBOARD", RoutePath: "/dashboard", Icon: "LayoutDashboard", Order: 1},
		{ModuleName: "Employees", ModuleKey: "EMPLOYEE", RoutePath: "/employees", Icon: "Users", Order: 2},
		{ModuleName: "Settings", ModuleKey: "SETTINGS", RoutePath: "/settings", Icon: "Settings", Order: 3},
	}

	for _, m := range modules {
		config.DB.Where(models.AppModule{ModuleKey: m.ModuleKey}).FirstOrCreate(&m)

		// 4. Assign Full Access to SUPER_ADMIN for each module (English Comment)
		access := models.RoleModuleAccess{
			RoleID:    adminRole.ID,
			ModuleID:  m.ID,
			CanView:   true,
			CanEdit:   true,
			DataScope: "ALL",
		}
		config.DB.Where(models.RoleModuleAccess{RoleID: adminRole.ID, ModuleID: m.ID}).FirstOrCreate(&access)
	}

	// 5. Master Data (English Comment)
	dept := models.Department{DeptName: "IT Administration", CompanyID: company.ID}
	config.DB.Where(models.Department{DeptName: "IT Administration", CompanyID: company.ID}).FirstOrCreate(&dept)

	pos := models.Position{PosName: "System Administrator", CompanyID: company.ID}
	config.DB.Where(models.Position{PosName: "System Administrator", CompanyID: company.ID}).FirstOrCreate(&pos)

	admin := models.Employee{
		CompanyID:    company.ID,
		RoleID:       adminRole.ID,
		EmployeeCode: "ADM001",
		FirstName:    "System",
		LastName:     "Admin",
		Email:        "admin@payroll.com",
		Password:     adminPass,
		DeptID:       dept.ID,
		PosID:        pos.ID,
		IsActive:     true,
	}

	if err := config.DB.Where("email = ?", "admin@payroll.com").FirstOrCreate(&admin).Error; err != nil {
		log.Fatalf("Failed to seed admin: %v", err)
	}

	log.Println("Seed completed: Modules and Admin created!")
}

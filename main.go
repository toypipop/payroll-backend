package main

import (
	"fmt"
	"log"
	"os"
	"payroll-backend/config"
	"payroll-backend/migrations"
	"payroll-backend/routes"
)

func main() {
	config.LoadDotEnv()

	if secret := os.Getenv("JWT_SECRET"); secret == "" {
		log.Fatal("FATAL: JWT_SECRET environment variable is not set")
	}

	// 1. Initialize Database Connection
	config.ConnectDatabase()

	// 2. Run one-time database repair migrations. Schema AutoMigrate is handled by migrate/main.go and seed/main.go.
	fmt.Println("Running Database Repair Migrations...")
	if err := migrations.RunOnce(config.DB, "20260525_drop_legacy_indexes", migrations.DropLegacyIndexes); err != nil {
		log.Fatalf("Legacy index cleanup failed: %v", err)
	}
	if err := migrations.RunOnce(config.DB, "20260525_rename_employee_company_access_table", migrations.RenameEmployeeCompanyAccessTable); err != nil {
		log.Fatalf("Employee-company-access table rename failed: %v", err)
	}
	if err := migrations.RunOnce(config.DB, "20260525_backfill_salaries_structure_companies", migrations.BackfillSalariesStructureCompanies); err != nil {
		log.Fatalf("Salaries company backfill failed: %v", err)
	}
	if err := migrations.RunOnce(config.DB, "20260525_remove_legacy_elevated_role", migrations.RemoveLegacyElevatedRole); err != nil {
		log.Fatalf("Legacy elevated role cleanup failed: %v", err)
	}
	if err := migrations.RunOnce(config.DB, "20260811_remove_dashboard_module", migrations.RemoveDashboardModule); err != nil {
		log.Fatalf("Dashboard module removal failed: %v", err)
	}
	fmt.Println("Database Repair Migrations Successful!")

	// Not RunOnce: the capability columns are a projection of the registry in
	// models/module_capability.go and are refreshed every boot, so code and
	// database can never drift apart.
	if err := migrations.SyncModuleCapabilities(config.DB); err != nil {
		log.Fatalf("Module capability sync failed (run `go run ./migrate` if the capability columns are missing): %v", err)
	}
	if err := migrations.PruneStaleRoleScopes(config.DB); err != nil {
		log.Fatalf("Permission scope normalisation failed: %v", err)
	}

	// 3. Initialize Router (Gin)
	// SetupRouter contains all your API endpoints
	r := routes.SetupRouter()

	// 4. Start Web Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	fmt.Printf("Server is running on port %s\n", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// All Modules seeded by seed/main.go.
// Not listed because they are not permission-controlled: Dashboard and
// My Payslips are universal pages with hardcoded routes and sidebar links.
// - Employees
//   - Employee List
//   - Salaries
//   - Employee Earnings
//   - Employee Deductions
// - Attendance
//   - Timelogs
//   - Leave Requests
//   - Leave Approved
//   - Earning Summary
//   - Deduction Summary
// - Payroll Processing
//   - Payroll Cycles
//   - Run Payroll
//   - Cycle Earnings
//   - Cycle Deductions
//   - Transaction
// - Payroll Config
//   - Global Settings
//   - Earning Types
//   - Deduction Types
// - Settings
//   - Companies
//   - Department
//   - Position
//   - Employment Types
//   - Roles
//   - Permission
//   - Employee Company Access
//   - Attendance Settings
//   - Bank Master
//   - Holiday Calendar
//   - Day Off
//   - Work Locations
//   - Leave Types
// - Reports
//   - Payslips
//   - Bank Transfer
//   - Tax Forms
//   - Social Security (SSO)
//   - Payroll Analysis

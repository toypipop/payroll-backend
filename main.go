package main

import (
	"fmt"
	"log"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/routes"
)

func main() {
	// 1. Initialize Database Connection
	config.ConnectDatabase()

	// 2. Run Database Migration
	// This will create or update tables based on your structs
	fmt.Println("Running Database Migration...")
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
		log.Fatalf("Migration Failed: %v", err)
	}
	fmt.Println("Migration Successful!")

	// 3. Initialize Router (Gin)
	// SetupRouter contains all your API endpoints
	r := routes.SetupRouter()

	// 4. Start Web Server
	fmt.Println("Server is running on port 8000")
	if err := r.Run(":8000"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// All Modules:
// ├── Dashboard
// ├── Employee Management
// │   ├── Employees
// │   └── Salaries Structure
// ├── Time Attendance
// │   ├── Time Logs
// │   ├── Leave Requests
// │   └── Leave Approved
// ├── Payroll Processing
// │   ├── Payroll Cycles
// │   ├── Run Payroll (Calculation)
// │   └── Transactions
// ├── Settings
// │   ├── Companies
// │   ├── Department
// │   ├── Position
// │   ├── Roles
// │   ├── Permission (Roles & Modules)
// │   └── Attendance Settings (Late/Absent On-Off)
// ├── Payroll Config
// │   ├── Global Settings (SSO, Tax)
// │   ├── Earning Types
// │   └── Deduction Types
// └── Reports
//     ├── Payslips
//     └── Government Forms

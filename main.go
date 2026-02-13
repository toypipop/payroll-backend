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
		&models.Companies{},
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

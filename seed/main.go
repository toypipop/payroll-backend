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
		log.Fatal("Error: Please set ADMIN_PASSWORD environment variable")
	}

	// 1. Create Initial Company (English Comment)
	company := models.Company{
		Name:  "Default Corporation",
		TaxID: "1234567890123",
	}
	// Use FirstOrCreate to prevent duplicate company during re-seed
	config.DB.FirstOrCreate(&company, models.Company{TaxID: "1234567890123"})

	// 2. Create Master Data linked to Company (English Comment)
	dept := models.Department{
		DeptName:  "IT Administration",
		CompanyID: company.ID,
	}
	config.DB.FirstOrCreate(&dept, models.Department{
		DeptName:  "IT Administration",
		CompanyID: company.ID,
	})

	pos := models.Position{
		PosName:   "System Administrator",
		CompanyID: company.ID,
	}
	config.DB.FirstOrCreate(&pos, models.Position{
		PosName:   "System Administrator",
		CompanyID: company.ID,
	})

	// 3. Create Admin Employee linked to Company (English Comment)
	admin := models.Employee{
		CompanyID:    company.ID,
		EmployeeCode: "ADM001",
		FirstName:    "System",
		LastName:     "Admin",
		Email:        "admin@payroll.com",
		Password:     adminPass,
		DeptID:       dept.ID,
		PosID:        pos.ID,
		Status:       "ACTIVE",
	}

	if err := config.DB.Create(&admin).Error; err != nil {
		log.Fatalf("Failed to seed admin: %v", err)
	}

	log.Println("✅ Seed completed: Company and Admin user created!")
}

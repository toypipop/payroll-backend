package main

import (
	"log"
	"os"
	"payroll-backend/config"
	"payroll-backend/models"
)

func main() {
	// 1. Initialize DB Connection
	config.ConnectDatabase()

	// 2. Get Password from Environment Variable
	adminPass := os.Getenv("ADMIN_PASSWORD")
	if adminPass == "" {
		log.Fatal("Error: Please set ADMIN_PASSWORD environment variable before running")
	}

	dept := models.Department{DeptName: "IT Administration"}
	config.DB.FirstOrCreate(&dept, models.Department{DeptName: "IT Administration"})

	pos := models.Position{PosName: "System Administrator"}
	config.DB.FirstOrCreate(&pos, models.Position{PosName: "System Administrator"})

	// 3. Define Admin User
	admin := models.Employee{
		EmployeeCode: "ADM001",
		FirstName:    "System",
		LastName:     "Admin",
		Email:        "admin@payroll.com",
		Password:     adminPass, // The 'BeforeCreate' hook will hash this automatically
		DeptID:       dept.ID,
		PosID:        pos.ID,
		Status:       "ACTIVE",
	}

	// 4. Create record using GORM
	if err := config.DB.Create(&admin).Error; err != nil {
		log.Fatalf("Failed to seed admin: %v", err)
	}

	log.Println("✅ Admin user (admin@payroll.com) created successfully!")
}

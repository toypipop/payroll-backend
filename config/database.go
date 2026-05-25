package config

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var DB *gorm.DB

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func ConnectDatabase() {
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "")
	host := getEnv("DB_HOST", "127.0.0.1")
	port := getEnv("DB_PORT", "5432")
	dbName := getEnv("DB_NAME", "payrolldb")
	sslMode := getEnv("DB_SSLMODE", "disable")
	timeZone := getEnv("DB_TIMEZONE", "Asia/Bangkok")

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		host, user, password, dbName, port, sslMode, timeZone,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true, // Singular table names
		},
	})

	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}

	fmt.Printf("Connected to Database: %s\n", dbName)
	DB = db
}

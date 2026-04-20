package config

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/mysql"
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
	user := getEnv("DB_USER", "root")
	password := getEnv("DB_PASSWORD", "1234")
	host := getEnv("DB_HOST", "127.0.0.1")
	port := getEnv("DB_PORT", "3306")
	dbName := getEnv("DB_NAME", "PayrollDB")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, password, host, port, dbName)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
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

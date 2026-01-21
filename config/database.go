package config

import (
	"fmt"
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var DB *gorm.DB

func ConnectDatabase() {
	user := "root"
	password := "@Toy*25094951"
	host := "127.0.0.1"
	port := "3306"
	dbName := "PayrollDB"

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, password, host, port, dbName)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true, // Table will be "company" instead of "company"
		},
	})

	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}

	fmt.Printf("Connected to Database: %s\n", dbName)
	DB = db
}

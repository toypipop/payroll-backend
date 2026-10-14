package config

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"payroll-backend/observability"
	"strings"
	"time"

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

func firstEnv(fallback string, keys ...string) string {
	for _, key := range keys {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return fallback
}

func postgresDSNValue(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, " ", `\ `)
	return escaped
}

func buildPostgresDSN() string {
	if dsn := firstEnv("", "DATABASE_URL", "POSTGRES_URL"); dsn != "" {
		return dsn
	}

	user := firstEnv("postgres", "DB_USER", "PGUSER")
	password := firstEnv("", "DB_PASSWORD", "PGPASSWORD")
	host := firstEnv("127.0.0.1", "DB_HOST", "PGHOST")
	port := firstEnv("5432", "DB_PORT", "PGPORT")
	dbName := firstEnv("payrolldb", "DB_NAME", "PGDATABASE")
	sslMode := firstEnv("disable", "DB_SSLMODE", "PGSSLMODE")
	timeZone := firstEnv("Asia/Bangkok", "DB_TIMEZONE", "PGTZ", "TZ")
	channelBinding := firstEnv("", "DB_CHANNEL_BINDING", "PGCHANNELBINDING")

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		postgresDSNValue(host),
		postgresDSNValue(user),
		postgresDSNValue(password),
		postgresDSNValue(dbName),
		postgresDSNValue(port),
		postgresDSNValue(sslMode),
		postgresDSNValue(timeZone),
	)
	if channelBinding != "" {
		dsn += " channel_binding=" + postgresDSNValue(channelBinding)
	}

	return dsn
}

func ConnectDatabase() {
	pool, err := readDatabasePool()
	if err != nil {
		log.Fatalf("Invalid database pool configuration: %T", err)
	}
	dbName := firstEnv("payrolldb", "DB_NAME", "PGDATABASE")

	db, err := gorm.Open(postgres.Open(buildPostgresDSN()), &gorm.Config{
		Logger: observability.NewDatabaseLogger(slog.Default(), 200*time.Millisecond),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true, // Singular table names
		},
	})

	if err != nil {
		observability.LogError("Failed to connect to database", err)
		os.Exit(1)
	}

	sqlDB, err := db.DB()
	if err != nil {
		observability.LogError("Failed to obtain database pool", err)
		os.Exit(1)
	}
	pool.apply(sqlDB)
	log.Printf("Database pool configured: max_open=%d max_idle=%d max_lifetime=%s max_idle_time=%s", pool.maxOpen, pool.maxIdle, pool.lifetime, pool.idleTime)

	fmt.Printf("Connected to Database: %s\n", dbName)
	DB = db
}

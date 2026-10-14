package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"payroll-backend/config"
	"payroll-backend/middleware"
	"payroll-backend/migrations"
	"payroll-backend/routes"
	"strings"

	"github.com/gin-gonic/gin"
)

func main() {
	config.LoadDotEnv()
	configureGinMode()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))

	if secret := os.Getenv("JWT_SECRET"); secret == "" {
		log.Fatal("FATAL: JWT_SECRET environment variable is not set")
	}

	// 1. Initialize Database Connection
	config.ConnectDatabase()

	// 2. Run one-time database repair migrations. Schema AutoMigrate is handled by migrate/main.go and seed/main.go.
	fmt.Println("Running Database Repair Migrations...")
	if err := migrations.RunOnce(config.DB, "20260525_drop_legacy_indexes", migrations.DropLegacyIndexes); err != nil {
		log.Fatalf("Legacy index cleanup failed: %T", err)
	}
	if err := migrations.RunOnce(config.DB, "20260525_rename_employee_company_access_table", migrations.RenameEmployeeCompanyAccessTable); err != nil {
		log.Fatalf("Employee-company-access table rename failed: %T", err)
	}
	if err := migrations.RunOnce(config.DB, "20260525_backfill_salaries_structure_companies", migrations.BackfillSalariesStructureCompanies); err != nil {
		log.Fatalf("Salaries company backfill failed: %T", err)
	}
	if err := migrations.RunOnce(config.DB, "20260525_remove_legacy_elevated_role", migrations.RemoveLegacyElevatedRole); err != nil {
		log.Fatalf("Legacy elevated role cleanup failed: %T", err)
	}
	if err := migrations.RunOnce(config.DB, "20260824_normalize_salary_active_flags", migrations.NormalizeSalaryActiveFlags); err != nil {
		log.Fatalf("Salary active-flag normalisation failed: %T", err)
	}
	if err := migrations.RunOnce(config.DB, "20261012_widen_copied_text_columns", migrations.WidenCopiedTextColumns); err != nil {
		log.Fatalf("Copied text column migration failed: %T", err)
	}

	if err := migrations.RunOnce(config.DB, "20260827_normalize_business_dates_to_bangkok", migrations.NormalizeBusinessDatesToBangkok); err != nil {
		log.Fatalf("Business date normalisation failed: %T", err)
	}
	fmt.Println("Database Repair Migrations Successful!")

	// Not RunOnce: the capability columns are a projection of the registry, re-synced on every boot.
	if err := migrations.SyncModuleCapabilities(config.DB); err != nil {
		log.Fatalf("Module capability sync failed (run `go run ./migrate` if the capability columns are missing): %T", err)
	}
	if err := migrations.PruneStaleRoleScopes(config.DB); err != nil {
		log.Fatalf("Permission scope normalisation failed: %T", err)
	}

	// 3. Compose every repository, service and handler once. This is the only place that knows config.DB.
	r := routes.SetupRouter(buildHandlers(config.DB), buildGuard(config.DB), buildDatabaseAdmission(config.DB))
	trustedProxyCIDRs := os.Getenv("TRUSTED_PROXY_CIDRS")
	if requiresTrustedProxyConfiguration() && strings.TrimSpace(trustedProxyCIDRs) == "" {
		log.Fatal("FATAL: TRUSTED_PROXY_CIDRS must list the reverse-proxy Docker subnet in UAT or production")
	}
	if err := middleware.ConfigureTrustedProxies(r, trustedProxyCIDRs); err != nil {
		log.Fatalf("FATAL: configure trusted proxies: %T", err)
	}

	// 4. Start Web Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	fmt.Printf("Server is running on port %s\n", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %T", err)
	}
}

func configureGinMode() {
	mode := os.Getenv("GIN_MODE")
	if mode == "" {
		mode = gin.ReleaseMode
	}
	gin.SetMode(mode)
}

func requiresTrustedProxyConfiguration() bool {
	environment := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	return environment == "uat" || environment == "production"
}

// All Modules seeded by seed/main.go.

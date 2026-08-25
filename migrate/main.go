package main

import (
	"log"
	"payroll-backend/config"
	"payroll-backend/migrations"
)

func main() {
	config.LoadDotEnv()

	config.ConnectDatabase()

	if err := migrations.RunOnce(config.DB, "20260525_drop_legacy_indexes", migrations.DropLegacyIndexes); err != nil {
		log.Fatalf("Legacy index cleanup failed: %v", err)
	}
	if err := migrations.RunOnce(config.DB, "20260525_rename_employee_company_access_table", migrations.RenameEmployeeCompanyAccessTable); err != nil {
		log.Fatalf("Employee-company-access table rename failed: %v", err)
	}
	if err := migrations.RunOnce(config.DB, "20260525_backfill_salaries_structure_companies", migrations.BackfillSalariesStructureCompanies); err != nil {
		log.Fatalf("Salaries company backfill failed: %v", err)
	}
	if err := migrations.RunOnce(config.DB, "20260703_recreate_taxid_partial_index", migrations.RecreateTaxIDPartialIndex); err != nil {
		log.Fatalf("Tax ID partial index recreate failed: %v", err)
	}

	if err := migrations.AutoMigrateSchema(config.DB); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	if err := migrations.RunOnce(config.DB, "20260525_remove_legacy_elevated_role", migrations.RemoveLegacyElevatedRole); err != nil {
		log.Fatalf("Legacy elevated role cleanup failed: %v", err)
	}
	if err := migrations.RunOnce(config.DB, "20260824_normalize_salary_active_flags", migrations.NormalizeSalaryActiveFlags); err != nil {
		log.Fatalf("Salary active-flag normalisation failed: %v", err)
	}

	log.Println("Database migrations completed successfully")
}
